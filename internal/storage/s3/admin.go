package s3store

import (
	"context"
	"errors"
	"path"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/sa6mwa/mkpod/internal/app/model"
	logger "github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/prompt"
)

var (
	ErrEmptyStore          = errors.New("empty store/bucket name")
	ErrEmptyKey            = errors.New("empty key/file name")
	ErrEmptyFilename       = errors.New("empty or missing filename given")
	ErrFileNotFound        = errors.New("file not found in remote storage")
	ErrInvalidStorageClass = errors.New("invalid storage class")
)

const (
	StorageClassStandard           = "STANDARD"
	StorageClassReducedRedundancy  = "REDUCED_REDUNDANCY"
	StorageClassStandardIA         = "STANDARD_IA"
	StorageClassOnezoneIA          = "ONEZONE_IA"
	StorageClassIntelligentTiering = "INTELLIGENT_TIERING"
	StorageClassGlacier            = "GLACIER"
	StorageClassDeepArchive        = "DEEP_ARCHIVE"
	StorageClassGlacierIR          = "GLACIER_IR"
	DefaultStorageClass            = StorageClassIntelligentTiering
)

type FileInfo struct {
	Size         int64
	ContentType  string
	StorageClass string
	LastModified string
	ETag         string
	Region       string
	Exists       bool
}

type Client struct {
	prompter prompt.Prompter
	atom     *model.Podcast
	cfg      awsv2.Config
	cfgReady bool
	s3       *awss3.Client
}

func New(atom *model.Podcast, prompter prompt.Prompter) *Client {
	return &Client{
		prompter: prompter,
		atom:     atom,
	}
}

func (a *Client) ensureClient(ctx context.Context) error {
	if a.s3 != nil {
		return nil
	}
	if !a.cfgReady {
		options := []func(*awsconfig.LoadOptions) error{}
		if a.atom != nil {
			if region := a.atom.Config.Aws.Region; region != "" {
				options = append(options, awsconfig.WithRegion(region))
			}
			if profile := a.atom.Config.Aws.Profile; profile != "" {
				options = append(options, awsconfig.WithSharedConfigProfile(profile))
			}
		}
		cfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
		if err != nil {
			return err
		}
		a.cfg = cfg
		a.cfgReady = true
	}
	a.s3 = awss3.NewFromConfig(a.cfg)
	return nil
}

func (a *Client) newUploader() *manager.Uploader {
	return manager.NewUploader(a.s3)
}

func (a *Client) newDownloader() *manager.Downloader {
	return manager.NewDownloader(a.s3)
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var noSuchKey *s3types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return true
		}
	}
	return false
}

func ValidStorageClasses() []string {
	return []string{
		StorageClassStandard,
		StorageClassReducedRedundancy,
		StorageClassStandardIA,
		StorageClassOnezoneIA,
		StorageClassIntelligentTiering,
		StorageClassGlacier,
		StorageClassDeepArchive,
		StorageClassGlacierIR,
	}
}

func IsValidStorageClass(storageClass string) bool {
	for _, valid := range ValidStorageClasses() {
		if storageClass == valid {
			return true
		}
	}
	return false
}

func validateObjectArgs(bucket, key string) error {
	if bucket == "" {
		return ErrEmptyStore
	}
	if key == "" {
		return ErrEmptyKey
	}
	return nil
}

func (a *Client) DeleteRemoteFile(ctx context.Context, bucket, key string) error {
	l := logger.FromContext(ctx)
	if err := validateObjectArgs(bucket, key); err != nil {
		return err
	}
	if err := a.ensureClient(ctx); err != nil {
		return err
	}

	s3path := "s3://" + path.Join(bucket, key)
	l.Info("About to delete remote file", "path", s3path)

	exists, err := a.FileExists(ctx, bucket, key)
	if err != nil {
		return err
	}
	if !exists {
		l.Warn("File does not exist, skipping deletion", "path", s3path)
		return nil
	}

	if !a.prompter.Ask(ctx, "Delete remote file %s?", s3path) {
		l.Info("Deletion cancelled by user", "path", s3path)
		return nil
	}

	_, err = a.s3.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		l.Error("Failed to delete remote file", "error", err, "path", s3path)
		return err
	}

	l.Info("Successfully deleted remote file", "path", s3path)
	return nil
}

func (a *Client) ChangeStorageClass(ctx context.Context, bucket, key, newStorageClass string) error {
	l := logger.FromContext(ctx)
	if err := validateObjectArgs(bucket, key); err != nil {
		return err
	}
	if err := a.ensureClient(ctx); err != nil {
		return err
	}
	if newStorageClass == "" {
		newStorageClass = DefaultStorageClass
	}
	if !IsValidStorageClass(newStorageClass) {
		return ErrInvalidStorageClass
	}

	s3path := "s3://" + path.Join(bucket, key)
	exists, err := a.FileExists(ctx, bucket, key)
	if err != nil {
		return err
	}
	if !exists {
		return ErrFileNotFound
	}

	currentClass, err := a.GetStorageClass(ctx, bucket, key)
	if err != nil {
		return err
	}
	if currentClass == newStorageClass {
		l.Info("Storage class already matches, no change needed", "path", s3path, "storageClass", currentClass)
		return nil
	}

	l.Info("Changing storage class", "path", s3path, "from", currentClass, "to", newStorageClass)
	if !a.prompter.Ask(ctx, "Change storage class of %s from %s to %s?", s3path, currentClass, newStorageClass) {
		l.Info("Storage class change cancelled by user", "path", s3path)
		return nil
	}

	storageClass := s3types.StorageClass(newStorageClass)
	copySource := path.Join(bucket, key)
	_, err = a.s3.CopyObject(ctx, &awss3.CopyObjectInput{
		Bucket:       &bucket,
		Key:          &key,
		CopySource:   &copySource,
		StorageClass: storageClass,
	})
	if err != nil {
		l.Error("Failed to change storage class", "error", err, "path", s3path)
		return err
	}

	l.Info("Successfully changed storage class", "path", s3path, "from", currentClass, "to", newStorageClass)
	return nil
}

func (a *Client) FileExists(ctx context.Context, bucket, key string) (bool, error) {
	l := logger.FromContext(ctx)
	if err := validateObjectArgs(bucket, key); err != nil {
		return false, err
	}
	if err := a.ensureClient(ctx); err != nil {
		return false, err
	}

	s3path := "s3://" + path.Join(bucket, key)
	_, err := a.s3.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		if isNotFoundError(err) {
			l.Debug("File does not exist", "path", s3path)
			return false, nil
		}
		l.Error("Error checking file existence", "error", err, "path", s3path)
		return false, err
	}

	l.Debug("File exists", "path", s3path)
	return true, nil
}

func (a *Client) GetStorageClass(ctx context.Context, bucket, key string) (string, error) {
	l := logger.FromContext(ctx)
	if err := validateObjectArgs(bucket, key); err != nil {
		return "", err
	}
	if err := a.ensureClient(ctx); err != nil {
		return "", err
	}

	s3path := "s3://" + path.Join(bucket, key)
	result, err := a.s3.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		if isNotFoundError(err) {
			l.Debug("File does not exist", "path", s3path)
			return "", ErrFileNotFound
		}
		l.Error("Error getting storage class", "error", err, "path", s3path)
		return "", err
	}

	storageClass := StorageClassStandard
	if result.StorageClass != "" {
		storageClass = string(result.StorageClass)
	}
	l.Debug("Got storage class", "path", s3path, "storageClass", storageClass)
	return storageClass, nil
}

func (a *Client) GetFileInfo(ctx context.Context, bucket, key string) (*FileInfo, error) {
	l := logger.FromContext(ctx)
	if err := validateObjectArgs(bucket, key); err != nil {
		return nil, err
	}
	if err := a.ensureClient(ctx); err != nil {
		return nil, err
	}

	s3path := "s3://" + path.Join(bucket, key)
	result, err := a.s3.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		if isNotFoundError(err) {
			l.Debug("File does not exist", "path", s3path)
			return &FileInfo{Exists: false}, nil
		}
		l.Error("Error getting file info", "error", err, "path", s3path)
		return nil, err
	}

	info := &FileInfo{Exists: true, Size: 0, StorageClass: StorageClassStandard, Region: a.atom.Config.Aws.Region}
	if result.ContentLength != nil {
		info.Size = *result.ContentLength
	}
	if result.ContentType != nil {
		info.ContentType = *result.ContentType
	}
	if result.StorageClass != "" {
		info.StorageClass = string(result.StorageClass)
	}
	if result.LastModified != nil {
		info.LastModified = result.LastModified.Format("2006-01-02T15:04:05Z07:00")
	}
	if result.ETag != nil {
		info.ETag = *result.ETag
	}

	l.Debug("Got file info", "path", s3path, "size", info.Size, "storageClass", info.StorageClass)
	return info, nil
}

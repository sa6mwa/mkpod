package s3store

import (
	"context"
	"errors"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/sa6mwa/mkpod/internal/app/model"
	logger "github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/prompt"
)

var (
	ErrNilPointerRequest   = errors.New("received nil pointer as request")
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
	session  *session.Session
	s3       *awss3.S3
}

func New(atom *model.Podcast, prompter prompt.Prompter) *Client {
	s := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: atom.Config.Aws.Profile,
		Config: aws.Config{
			Region: aws.String(atom.Config.Aws.Region),
		},
	}))
	return &Client{
		prompter: prompter,
		atom:     atom,
		session:  s,
		s3:       awss3.New(s),
	}
}

func NewAdminClient(atom *model.Podcast, prompter prompt.Prompter) *Client {
	return New(atom, prompter)
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

	_, err = a.s3.DeleteObject(&awss3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
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

	_, err = a.s3.CopyObject(&awss3.CopyObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key),
		CopySource:   aws.String(path.Join(bucket, key)),
		StorageClass: aws.String(newStorageClass),
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

	s3path := "s3://" + path.Join(bucket, key)
	_, err := a.s3.HeadObject(&awss3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "NotFound", "NoSuchKey":
				l.Debug("File does not exist", "path", s3path)
				return false, nil
			default:
				l.Error("Error checking file existence", "error", err, "path", s3path)
				return false, err
			}
		}
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

	s3path := "s3://" + path.Join(bucket, key)
	result, err := a.s3.HeadObject(&awss3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "NotFound", "NoSuchKey":
				l.Debug("File does not exist", "path", s3path)
				return "", ErrFileNotFound
			default:
				l.Error("Error getting storage class", "error", err, "path", s3path)
				return "", err
			}
		}
		return "", err
	}

	storageClass := StorageClassStandard
	if result.StorageClass != nil {
		storageClass = *result.StorageClass
	}
	l.Debug("Got storage class", "path", s3path, "storageClass", storageClass)
	return storageClass, nil
}

func (a *Client) GetFileInfo(ctx context.Context, bucket, key string) (*FileInfo, error) {
	l := logger.FromContext(ctx)
	if err := validateObjectArgs(bucket, key); err != nil {
		return nil, err
	}

	s3path := "s3://" + path.Join(bucket, key)
	result, err := a.s3.HeadObject(&awss3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "NotFound", "NoSuchKey":
				l.Debug("File does not exist", "path", s3path)
				return &FileInfo{Exists: false}, nil
			default:
				l.Error("Error getting file info", "error", err, "path", s3path)
				return nil, err
			}
		}
		return nil, err
	}

	info := &FileInfo{Exists: true, Size: 0, StorageClass: StorageClassStandard, Region: a.atom.Config.Aws.Region}
	if result.ContentLength != nil {
		info.Size = *result.ContentLength
	}
	if result.ContentType != nil {
		info.ContentType = *result.ContentType
	}
	if result.StorageClass != nil {
		info.StorageClass = *result.StorageClass
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

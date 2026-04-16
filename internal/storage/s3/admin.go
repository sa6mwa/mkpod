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
	"github.com/sa6mwa/mkpod/internal/infra/adapters/asker"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
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

type ObjectRequest struct {
	Store           string
	Key             string
	NewStorageClass string
	Region          string
}

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
	prompter asker.Prompter
	atom     *model.Podcast
	session  *session.Session
	s3       *awss3.S3
}

func New(atom *model.Podcast, prompter asker.Prompter) *Client {
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

func NewAdminClient(atom *model.Podcast, prompter asker.Prompter) *Client {
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

func (a *Client) DeleteRemoteFile(ctx context.Context, request *ObjectRequest) error {
	l := logger.FromContext(ctx)

	if request == nil {
		return ErrNilPointerRequest
	}
	if request.Store == "" {
		return ErrEmptyStore
	}
	if request.Key == "" {
		return ErrEmptyKey
	}

	s3path := "s3://" + path.Join(request.Store, request.Key)
	l.Info("About to delete remote file", "path", s3path)

	exists, err := a.FileExists(ctx, request)
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
		Bucket: aws.String(request.Store),
		Key:    aws.String(request.Key),
	})
	if err != nil {
		l.Error("Failed to delete remote file", "error", err, "path", s3path)
		return err
	}

	l.Info("Successfully deleted remote file", "path", s3path)
	return nil
}

func (a *Client) ChangeStorageClass(ctx context.Context, request *ObjectRequest) error {
	l := logger.FromContext(ctx)

	if request == nil {
		return ErrNilPointerRequest
	}
	if request.Store == "" {
		return ErrEmptyStore
	}
	if request.Key == "" {
		return ErrEmptyKey
	}
	if request.NewStorageClass == "" {
		request.NewStorageClass = DefaultStorageClass
	}
	if !IsValidStorageClass(request.NewStorageClass) {
		return ErrInvalidStorageClass
	}

	s3path := "s3://" + path.Join(request.Store, request.Key)
	exists, err := a.FileExists(ctx, request)
	if err != nil {
		return err
	}
	if !exists {
		return ErrFileNotFound
	}

	currentClass, err := a.GetStorageClass(ctx, request)
	if err != nil {
		return err
	}
	if currentClass == request.NewStorageClass {
		l.Info("Storage class already matches, no change needed", "path", s3path, "storageClass", currentClass)
		return nil
	}

	l.Info("Changing storage class", "path", s3path, "from", currentClass, "to", request.NewStorageClass)
	if !a.prompter.Ask(ctx, "Change storage class of %s from %s to %s?", s3path, currentClass, request.NewStorageClass) {
		l.Info("Storage class change cancelled by user", "path", s3path)
		return nil
	}

	_, err = a.s3.CopyObject(&awss3.CopyObjectInput{
		Bucket:       aws.String(request.Store),
		Key:          aws.String(request.Key),
		CopySource:   aws.String(path.Join(request.Store, request.Key)),
		StorageClass: aws.String(request.NewStorageClass),
	})
	if err != nil {
		l.Error("Failed to change storage class", "error", err, "path", s3path)
		return err
	}

	l.Info("Successfully changed storage class", "path", s3path, "from", currentClass, "to", request.NewStorageClass)
	return nil
}

func (a *Client) FileExists(ctx context.Context, request *ObjectRequest) (bool, error) {
	l := logger.FromContext(ctx)

	if request == nil {
		return false, ErrNilPointerRequest
	}
	if request.Store == "" {
		return false, ErrEmptyStore
	}
	if request.Key == "" {
		return false, ErrEmptyKey
	}

	s3path := "s3://" + path.Join(request.Store, request.Key)
	_, err := a.s3.HeadObject(&awss3.HeadObjectInput{
		Bucket: aws.String(request.Store),
		Key:    aws.String(request.Key),
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

func (a *Client) GetStorageClass(ctx context.Context, request *ObjectRequest) (string, error) {
	l := logger.FromContext(ctx)

	if request == nil {
		return "", ErrNilPointerRequest
	}
	if request.Store == "" {
		return "", ErrEmptyStore
	}
	if request.Key == "" {
		return "", ErrEmptyKey
	}

	s3path := "s3://" + path.Join(request.Store, request.Key)
	result, err := a.s3.HeadObject(&awss3.HeadObjectInput{
		Bucket: aws.String(request.Store),
		Key:    aws.String(request.Key),
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

func (a *Client) GetFileInfo(ctx context.Context, request *ObjectRequest) (*FileInfo, error) {
	l := logger.FromContext(ctx)

	if request == nil {
		return nil, ErrNilPointerRequest
	}
	if request.Store == "" {
		return nil, ErrEmptyStore
	}
	if request.Key == "" {
		return nil, ErrEmptyKey
	}

	s3path := "s3://" + path.Join(request.Store, request.Key)
	result, err := a.s3.HeadObject(&awss3.HeadObjectInput{
		Bucket: aws.String(request.Store),
		Key:    aws.String(request.Key),
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

	info := &FileInfo{
		Exists:       true,
		Size:         0,
		StorageClass: StorageClassStandard,
		Region:       a.atom.Config.Aws.Region,
	}
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

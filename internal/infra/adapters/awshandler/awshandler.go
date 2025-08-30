// awshandler is the AWS S3 remote file administration adapter that
// implements the ForAdministeringRemoteFiles port interface.
package awshandler

import (
	"context"
	"errors"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
)

var (
	ErrNilPointerRequest = errors.New("received nil pointer as request")
	ErrEmptyStore        = errors.New("empty store/bucket name")
	ErrEmptyKey          = errors.New("empty key/file name")
	ErrFileNotFound      = errors.New("file not found in remote storage")
	ErrInvalidStorageClass = errors.New("invalid storage class")
)

type forAdministeringRemoteFiles struct {
	ports.ForAsking
	atom    *model.Atom
	session *session.Session
	s3      *s3.S3
}

// New creates a new AWS S3 remote file administration adapter
func New(atom *model.Atom, asker ports.ForAsking) ports.ForAdministeringRemoteFiles {
	s := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: atom.Config.Aws.Profile,
		Config: aws.Config{
			Region: aws.String(atom.Config.Aws.Region),
		},
	}))
	return &forAdministeringRemoteFiles{
		ForAsking: asker,
		atom:      atom,
		session:   s,
		s3:        s3.New(s),
	}
}

// DeleteRemoteFile removes a file from S3 storage
func (a *forAdministeringRemoteFiles) DeleteRemoteFile(ctx context.Context, request *ports.ForAdministeringRemoteFilesRequest) error {
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
	
	// Check if file exists first
	exists, err := a.FileExists(ctx, request)
	if err != nil {
		return err
	}
	if !exists {
		l.Warn("File does not exist, skipping deletion", "path", s3path)
		return nil
	}

	// Ask for confirmation unless forced
	if !a.Ask(ctx, "Delete remote file %s?", s3path) {
		l.Info("Deletion cancelled by user", "path", s3path)
		return nil
	}

	// Perform deletion
	_, err = a.s3.DeleteObject(&s3.DeleteObjectInput{
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

// ChangeStorageClass changes the storage class of a file in S3
func (a *forAdministeringRemoteFiles) ChangeStorageClass(ctx context.Context, request *ports.ForAdministeringRemoteFilesRequest) error {
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
		request.NewStorageClass = ports.DefaultStorageClass
	}
	if !ports.IsValidStorageClass(request.NewStorageClass) {
		return ErrInvalidStorageClass
	}

	s3path := "s3://" + path.Join(request.Store, request.Key)
	
	// Check if file exists
	exists, err := a.FileExists(ctx, request)
	if err != nil {
		return err
	}
	if !exists {
		return ErrFileNotFound
	}

	// Get current storage class
	currentClass, err := a.GetStorageClass(ctx, request)
	if err != nil {
		return err
	}

	if currentClass == request.NewStorageClass {
		l.Info("Storage class already matches, no change needed", 
			"path", s3path, 
			"storageClass", currentClass)
		return nil
	}

	l.Info("Changing storage class", 
		"path", s3path, 
		"from", currentClass, 
		"to", request.NewStorageClass)

	// Ask for confirmation unless forced
	if !a.Ask(ctx, "Change storage class of %s from %s to %s?", s3path, currentClass, request.NewStorageClass) {
		l.Info("Storage class change cancelled by user", "path", s3path)
		return nil
	}

	// Copy object to itself with new storage class
	_, err = a.s3.CopyObject(&s3.CopyObjectInput{
		Bucket:       aws.String(request.Store),
		Key:          aws.String(request.Key),
		CopySource:   aws.String(path.Join(request.Store, request.Key)),
		StorageClass: aws.String(request.NewStorageClass),
	})
	if err != nil {
		l.Error("Failed to change storage class", "error", err, "path", s3path)
		return err
	}

	l.Info("Successfully changed storage class", 
		"path", s3path, 
		"from", currentClass, 
		"to", request.NewStorageClass)
	return nil
}

// FileExists checks if a file exists in S3 storage
func (a *forAdministeringRemoteFiles) FileExists(ctx context.Context, request *ports.ForAdministeringRemoteFilesRequest) (bool, error) {
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
	
	_, err := a.s3.HeadObject(&s3.HeadObjectInput{
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

// GetStorageClass returns the current storage class of a file in S3
func (a *forAdministeringRemoteFiles) GetStorageClass(ctx context.Context, request *ports.ForAdministeringRemoteFilesRequest) (string, error) {
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
	
	result, err := a.s3.HeadObject(&s3.HeadObjectInput{
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

	storageClass := ports.StorageClassStandard // Default if not specified
	if result.StorageClass != nil {
		storageClass = *result.StorageClass
	}

	l.Debug("Got storage class", "path", s3path, "storageClass", storageClass)
	return storageClass, nil
}

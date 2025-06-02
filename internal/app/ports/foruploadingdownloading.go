package ports

import (
	"context"
	"errors"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

var (
	// ports.ErrNotFound should be used by code calling an adapter as
	// the adapter implementing ForUploadingDownloading (or either
	// ForUploading or ForDownloading) will return this error if a key,
	// name or file does not exist in the storage backend.
	ErrNotFound error = errors.New("no such file or key")
)

type ForUploading interface {
	UploadFile(ctx context.Context, bucketOrStore, contentType, fileToUpload string) error
	GetSize(ctx context.Context, bucketOrStore, keyOrName string) (int64, error)
	Diff(ctx context.Context, bucketOrStore, keyOrName, fileToDiff string) error
}

type ForDownloading interface {
	DownloadFile(ctx context.Context, bucketOrStore, keyOrName string) error
	GetSize(ctx context.Context, bucketOrStore, keyOrName string) (int64, error)
}

type ForUploadingDownloading interface {
	ForUploading
	ForDownloading
}

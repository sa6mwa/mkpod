package ports

import (
	"context"
	"errors"
)

var (
	// ports.ErrNotFound should be used by code calling an adapter as
	// the adapter implementing ForUploadingDownloading (or either
	// ForUploading or ForDownloading) will return this error if a key,
	// name or file does not exist in the storage backend.
	ErrNotFound error = errors.New("no such file or key")
)

type ForUploadingRequest struct {
	// Bucket or store to upload to.
	Store string
	// Key or name of target. If empty, default to the From field.
	To string
	// From is the path to upload from (local disk or URI depending on
	// adapter implementation).
	From        string
	ContentType string
	// StorageClass only used for AWS. Can be STANDARD,
	// REDUCED_REDUNDANCY, STANDARD_IA, ONEZONE_IA, INTELLIGENT_TIERING,
	// GLACIER, DEEP_ARCHIVE, and GLACIER_IR. If empty, STANDARD is the
	// default.
	StorageClass string
}

type ForUploading interface {
	Upload(ctx context.Context, request *ForUploadingRequest) error
	Diff(ctx context.Context, bucketOrStore, keyOrName, fileToDiff string) error
}

type ForDownloading interface {
	ForAsking
	Download(ctx context.Context, bucketOrStore, keyOrName string) error
	GetSize(ctx context.Context, bucketOrStore, keyOrName string) (int64, error)
}

// ForUploadingDownloading is a composite interface of both
// ForUploading and ForDownloading. Use NewUploaderDownloader() to
// return a composite implementation from an uploader and a downloader
// adapter.
type ForUploadingDownloading interface {
	ForUploading
	ForDownloading
}

type forUploadingDownloading struct {
	ForUploading
	ForDownloading
}

// NewUploaderDownloader returns the composite interface
// ForUploadingDownloading. The uploader adapter is used as
// ForUploading implementation and the downloader is used as
// ForDownloading implementation.
func NewUploaderDownloader(uploader ForUploading, downloader ForDownloading) ForUploadingDownloading {
	return &forUploadingDownloading{
		ForUploading:   uploader,
		ForDownloading: downloader,
	}
}

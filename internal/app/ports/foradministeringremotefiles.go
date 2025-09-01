package ports

import (
	"context"
)

// AWS S3 Storage Classes
const (
	StorageClassStandard            = "STANDARD"
	StorageClassReducedRedundancy   = "REDUCED_REDUNDANCY"
	StorageClassStandardIA          = "STANDARD_IA"
	StorageClassOnezoneIA           = "ONEZONE_IA"
	StorageClassIntelligentTiering  = "INTELLIGENT_TIERING"
	StorageClassGlacier             = "GLACIER"
	StorageClassDeepArchive         = "DEEP_ARCHIVE"
	StorageClassGlacierIR           = "GLACIER_IR"
	DefaultStorageClass             = StorageClassIntelligentTiering
)

// ForAdministeringRemoteFilesRequest contains parameters for remote file operations
type ForAdministeringRemoteFilesRequest struct {
	// Bucket or store to operate on
	Store string
	// Key or name of the remote file
	Key string
	// NewStorageClass for ChangeStorageClass operations (optional)
	NewStorageClass string
	// Region where the bucket is located (optional, uses adapter default if empty)
	Region string
}

// RemoteFileInfo contains metadata about a remote file
type RemoteFileInfo struct {
	// Size in bytes
	Size int64
	// ContentType (MIME type)
	ContentType string
	// StorageClass (e.g., STANDARD, INTELLIGENT_TIERING)
	StorageClass string
	// LastModified timestamp
	LastModified string
	// ETag (version identifier)
	ETag string
	// Region where the file is stored (e.g., us-east-1, eu-west-1)
	Region string
	// Exists indicates if the file exists
	Exists bool
}

// ForAdministeringRemoteFiles provides administrative operations for remote files
// such as deletion and storage class changes
type ForAdministeringRemoteFiles interface {
	ForAsking
	// DeleteRemoteFile removes a file from the remote storage
	DeleteRemoteFile(ctx context.Context, request *ForAdministeringRemoteFilesRequest) error
	// ChangeStorageClass changes the storage class of a remote file
	ChangeStorageClass(ctx context.Context, request *ForAdministeringRemoteFilesRequest) error
	// FileExists checks if a file exists in the remote storage
	FileExists(ctx context.Context, request *ForAdministeringRemoteFilesRequest) (bool, error)
	// GetStorageClass returns the current storage class of a remote file
	GetStorageClass(ctx context.Context, request *ForAdministeringRemoteFilesRequest) (string, error)
	// GetFileInfo returns comprehensive metadata about a remote file
	GetFileInfo(ctx context.Context, request *ForAdministeringRemoteFilesRequest) (*RemoteFileInfo, error)
}

// ValidStorageClasses returns a list of valid AWS S3 storage classes
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

// IsValidStorageClass checks if the given storage class is valid
func IsValidStorageClass(storageClass string) bool {
	for _, valid := range ValidStorageClasses() {
		if storageClass == valid {
			return true
		}
	}
	return false
}

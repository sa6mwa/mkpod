package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sa6mwa/mkpod/internal/app/model"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
)

type workflowOperation struct {
	Kind           string `json:"kind"`
	Label          string `json:"label,omitempty"`
	Bucket         string `json:"bucket,omitempty"`
	Key            string `json:"key,omitempty"`
	LocalPath      string `json:"localPath,omitempty"`
	Reason         string `json:"reason"`
	LocalExists    bool   `json:"localExists,omitempty"`
	LocalSize      int64  `json:"localSize,omitempty"`
	LocalChecksum  string `json:"localChecksum,omitempty"`
	RemoteExists   string `json:"remoteExists,omitempty"`
	RemoteSize     int64  `json:"remoteSize,omitempty"`
	RemoteETag     string `json:"remoteETag,omitempty"`
	RequiresPrompt bool   `json:"requiresPrompt,omitempty"`
	DefaultAnswer  bool   `json:"defaultAnswer,omitempty"`
	SafetyStatus   string `json:"safetyStatus,omitempty"`
}

type outputExistenceClient interface {
	FileExists(context.Context, string, string) (bool, error)
}

type remoteMasterInfoClient interface {
	GetFileInfo(context.Context, string, string) (*s3store.FileInfo, error)
}

func decideLocalAssetSync(ctx context.Context, atom *model.Podcast, client assetInfoClient, buckets []string, key, label string) (workflowOperation, error) {
	key = normalizeStorageKey(atom, key)
	if key == "" {
		return workflowOperation{
			Kind:   "skip-asset",
			Label:  label,
			Reason: "asset key is empty or not managed by mkpod storage",
		}, nil
	}
	localPath := localAssetPath(atom, key)
	if info, err := os.Stat(localPath); err == nil {
		return workflowOperation{
			Kind:        "skip-asset",
			Label:       label,
			Key:         key,
			LocalPath:   localPath,
			Reason:      "local asset exists",
			LocalExists: true,
			LocalSize:   info.Size(),
		}, nil
	} else if !os.IsNotExist(err) {
		return workflowOperation{}, fmt.Errorf("failed to access local %s %s: %w", label, localPath, err)
	}
	if client == nil {
		return workflowOperation{}, fmt.Errorf("missing local %s %s and no remote storage client is configured", label, localPath)
	}

	for _, bucket := range buckets {
		bucket = strings.TrimSpace(bucket)
		if bucket == "" {
			continue
		}
		info, err := client.GetFileInfo(ctx, bucket, key)
		if err != nil {
			return workflowOperation{}, fmt.Errorf("failed to check remote %s %s in bucket %s: %w", label, key, bucket, err)
		}
		if info != nil && info.Exists {
			return workflowOperation{
				Kind:         "download-asset",
				Label:        label,
				Bucket:       bucket,
				Key:          key,
				LocalPath:    localPath,
				Reason:       "local asset is missing and remote asset exists",
				RemoteExists: "true",
				RemoteSize:   info.Size,
			}, nil
		}
	}

	return workflowOperation{}, fmt.Errorf("missing %s %s locally and in configured S3 buckets", label, key)
}

func decideOutputUpload(ctx context.Context, atom *model.Podcast, episode *model.Episode, client outputExistenceClient, wasEncoded bool) (workflowOperation, error) {
	bucket := atom.Config.Aws.Buckets.Output
	key := episode.Output
	localPath := filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(key))

	info, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		return workflowOperation{
			Kind:      "skip-output-upload",
			Bucket:    bucket,
			Key:       key,
			LocalPath: localPath,
			Reason:    "local output is missing",
		}, nil
	}
	if err != nil {
		return workflowOperation{}, fmt.Errorf("failed to access local encoded file %s: %w", localPath, err)
	}
	if client == nil {
		return workflowOperation{
			Kind:        "skip-output-upload",
			Bucket:      bucket,
			Key:         key,
			LocalPath:   localPath,
			LocalExists: true,
			LocalSize:   info.Size(),
			Reason:      "remote output check skipped",
		}, nil
	}

	exists, err := client.FileExists(ctx, bucket, key)
	if err != nil {
		return workflowOperation{}, fmt.Errorf("failed to check remote output %s in bucket %s: %w", key, bucket, err)
	}
	remoteExists := "false"
	if exists {
		remoteExists = "true"
	}
	operation := workflowOperation{
		Kind:         "skip-output-upload",
		Bucket:       bucket,
		Key:          key,
		LocalPath:    localPath,
		Reason:       "remote output exists and output was not re-encoded",
		LocalExists:  true,
		LocalSize:    info.Size(),
		RemoteExists: remoteExists,
	}
	if !exists {
		operation.Kind = "upload-output"
		operation.Reason = "local output exists but remote output is missing"
		operation.RequiresPrompt = true
		return operation, nil
	}
	if wasEncoded {
		operation.Kind = "upload-output"
		operation.Reason = "output was re-encoded and remote output exists"
		operation.RequiresPrompt = true
		return operation, nil
	}
	return operation, nil
}

func decidePlannedOutputUpload(ctx context.Context, atom *model.Podcast, episode *model.Episode, client outputExistenceClient, localOutputExists, willEncode bool) (workflowOperation, error) {
	if !willEncode {
		return decideOutputUpload(ctx, atom, episode, client, false)
	}
	bucket := atom.Config.Aws.Buckets.Output
	key := episode.Output
	localPath := filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(key))
	if client == nil {
		return workflowOperation{
			Kind:      "check-output-upload",
			Bucket:    bucket,
			Key:       key,
			LocalPath: localPath,
			Reason:    "output will be checked for upload after encoding",
		}, nil
	}
	exists, err := client.FileExists(ctx, bucket, key)
	if err != nil {
		return workflowOperation{}, fmt.Errorf("failed to check remote output %s in bucket %s: %w", key, bucket, err)
	}
	return workflowOperation{
		Kind:           "upload-output",
		Bucket:         bucket,
		Key:            key,
		LocalPath:      localPath,
		Reason:         plannedOutputUploadReason(exists, localOutputExists),
		LocalExists:    localOutputExists,
		RemoteExists:   boolText(exists),
		RequiresPrompt: true,
	}, nil
}

func plannedOutputUploadReason(remoteExists, localOutputExists bool) string {
	if remoteExists {
		return "output will be re-encoded and remote output exists"
	}
	if localOutputExists {
		return "output will be re-encoded and remote output is missing"
	}
	return "output will be encoded and remote output is missing"
}

func decideRemoteMasterRemoval(ctx context.Context, atom *model.Podcast, episode *model.Episode, client remoteMasterInfoClient) (workflowOperation, error) {
	bucket := atom.Config.Aws.Buckets.Input
	key := normalizeStorageKey(atom, episode.Input)
	localPath := localAssetPath(atom, key)
	localInfo, localErr := os.Stat(localPath)
	if localErr != nil && !os.IsNotExist(localErr) {
		return workflowOperation{}, fmt.Errorf("failed to check local master file %s: %w", localPath, localErr)
	}
	if client == nil {
		return workflowOperation{
			Kind:        "skip-remote-master-delete",
			Bucket:      bucket,
			Key:         key,
			LocalPath:   localPath,
			Reason:      "remote master check skipped",
			LocalExists: localErr == nil,
			LocalSize:   fileSize(localInfo),
		}, nil
	}

	remoteInfo, err := client.GetFileInfo(ctx, bucket, key)
	if err != nil {
		return workflowOperation{}, fmt.Errorf("failed to get remote master file info: %w", err)
	}
	remoteExists := remoteInfo != nil && remoteInfo.Exists
	remoteSize := int64(0)
	if remoteInfo != nil {
		remoteSize = remoteInfo.Size
	}
	decision := s3store.EvaluateRemoteMasterRemoval(localErr == nil, fileSize(localInfo), remoteExists, remoteSize)
	operation := workflowOperation{
		Kind:           "skip-remote-master-delete",
		Bucket:         bucket,
		Key:            key,
		LocalPath:      localPath,
		Reason:         remoteMasterRemovalReason(decision.Reason),
		LocalExists:    localErr == nil,
		LocalSize:      decision.LocalSize,
		RemoteExists:   boolText(remoteExists),
		RemoteSize:     decision.RemoteSize,
		RequiresPrompt: decision.Allowed,
		SafetyStatus:   decision.Reason,
	}
	if decision.Allowed {
		operation.Kind = "delete-remote-master"
	}
	return operation, nil
}

func remoteMasterRemovalReason(reason string) string {
	switch reason {
	case s3store.RemovalAllowed:
		return "local master satisfies remote deletion safety checks"
	case s3store.RemovalLocalMissing:
		return "local master file does not exist"
	case s3store.RemovalRemoteMissing:
		return "remote master file does not exist"
	case s3store.RemovalLocalTooSmall:
		return "local master file is too small compared to remote master"
	default:
		return reason
	}
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

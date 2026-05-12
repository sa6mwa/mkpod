package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func decidePublishImage(ctx context.Context, atom *model.Podcast, image referencedImage, client publishAssetClient) (workflowOperation, error) {
	if atom == nil {
		return workflowOperation{}, fmt.Errorf("podcast is nil")
	}
	localPath := localAssetPath(atom, image.Key)
	fi, err := os.Stat(localPath)
	localExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return workflowOperation{}, fmt.Errorf("failed to access local %s image %s: %w", image.Label, localPath, err)
	}

	if client == nil {
		if !localExists {
			return workflowOperation{}, fmt.Errorf("missing local %s image %s", image.Label, localPath)
		}
		return workflowOperation{
			Kind:        "check-image-upload",
			Label:       image.Label,
			Key:         image.Key,
			LocalPath:   localPath,
			Reason:      "remote image check skipped",
			LocalExists: true,
			LocalSize:   fi.Size(),
		}, nil
	}

	info, err := client.GetFileInfo(ctx, atom.Config.Aws.Buckets.Output, image.Key)
	if err != nil {
		return workflowOperation{}, fmt.Errorf("failed to check remote %s image %s in bucket %s: %w", image.Label, image.Key, atom.Config.Aws.Buckets.Output, err)
	}
	remoteExists := info != nil && info.Exists
	remoteSize := int64(0)
	if info != nil {
		remoteSize = info.Size
	}

	if !localExists {
		if remoteExists {
			return workflowOperation{
				Kind:         "download-image",
				Label:        image.Label,
				Bucket:       atom.Config.Aws.Buckets.Output,
				Key:          image.Key,
				LocalPath:    localPath,
				Reason:       "local image is missing and remote image exists",
				RemoteExists: "true",
				RemoteSize:   remoteSize,
			}, nil
		}
		return workflowOperation{}, fmt.Errorf("missing %s image %s locally and in output bucket %s", image.Label, image.Key, atom.Config.Aws.Buckets.Output)
	}

	operation := workflowOperation{
		Kind:         "skip-image-upload",
		Label:        image.Label,
		Bucket:       atom.Config.Aws.Buckets.Output,
		Key:          image.Key,
		LocalPath:    localPath,
		Reason:       "remote image exists with matching size",
		LocalExists:  true,
		LocalSize:    fi.Size(),
		RemoteExists: boolText(remoteExists),
		RemoteSize:   remoteSize,
	}
	if remoteExists && remoteSize == fi.Size() {
		return operation, nil
	}
	operation.Kind = "upload-image"
	operation.Reason = "local image exists but remote image is missing or differs"
	operation.RequiresPrompt = true
	return operation, nil
}

func decideFeedUpload(ctx context.Context, atom *model.Podcast, feedPath string, client assetInfoClient) (workflowOperation, error) {
	if atom == nil {
		return workflowOperation{}, fmt.Errorf("podcast is nil")
	}
	operation := workflowOperation{
		Kind:           "upload-feed",
		Bucket:         atom.Config.Aws.Buckets.Output,
		Key:            atom.FeedFile,
		LocalPath:      feedPath,
		Reason:         "feed will be generated, diffed, and prompt before upload",
		RequiresPrompt: true,
		RemoteExists:   "unknown",
	}
	if client != nil {
		info, err := client.GetFileInfo(ctx, atom.Config.Aws.Buckets.Output, atom.FeedFile)
		if err != nil {
			return workflowOperation{}, fmt.Errorf("failed to check remote feed %s in bucket %s: %w", atom.FeedFile, atom.Config.Aws.Buckets.Output, err)
		}
		operation.RemoteExists = boolText(info != nil && info.Exists)
		if info != nil {
			operation.RemoteSize = info.Size
		}
	}
	return operation, nil
}

func planFeedOperations(ctx context.Context, atom *model.Podcast, feedPath string, storageClient publishAssetClient) ([]workflowOperation, error) {
	operations := make([]workflowOperation, 0)
	for _, image := range collectReferencedImages(atom) {
		operation, err := decidePublishImage(ctx, atom, image, storageClient)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	feedOperation, err := decideFeedUpload(ctx, atom, feedPath, storageClient)
	if err != nil {
		return nil, err
	}
	operations = append(operations, feedOperation)
	return operations, nil
}

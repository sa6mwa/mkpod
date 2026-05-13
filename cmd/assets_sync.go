package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	logger "github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/spec"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
)

type assetInfoClient interface {
	GetFileInfo(context.Context, string, string) (*s3store.FileInfo, error)
	DownloadFile(context.Context, string, string) error
}

type publishAssetClient interface {
	assetInfoClient
	UploadFile(context.Context, string, string, string, *s3store.UploadOptions) error
}

type referencedImage struct {
	Key         string
	Label       string
	ContentType string
}

func localAssetPath(atom *model.Podcast, key string) string {
	return filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(key))
}

func normalizeStorageKey(atom *model.Podcast, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		return strings.TrimLeft(filepath.ToSlash(raw), "/")
	}

	prefixes := []string{
		strings.TrimRight(atom.Config.BaseURL, "/") + "/",
		fmt.Sprintf("https://%s.s3.%s.amazonaws.com/", atom.Config.Aws.Buckets.Output, atom.Config.Aws.Region),
	}
	for _, prefix := range prefixes {
		if prefix != "/" && strings.HasPrefix(raw, prefix) {
			return strings.TrimLeft(strings.TrimPrefix(raw, prefix), "/")
		}
	}
	return ""
}

func syncLocalAssetFromBuckets(ctx context.Context, atom *model.Podcast, client assetInfoClient, buckets []string, key, label string) error {
	operation, err := decideLocalAssetSync(ctx, atom, client, buckets, key, label)
	if err != nil {
		return err
	}
	if operation.Kind != "download-asset" {
		return nil
	}
	if err := client.DownloadFile(ctx, operation.Bucket, operation.Key); err != nil {
		return fmt.Errorf("failed to download remote %s %s from bucket %s: %w", label, operation.Key, operation.Bucket, err)
	}
	return nil
}

func collectReferencedImages(atom *model.Podcast) []referencedImage {
	return collectReferencedImagesAt(atom, time.Now())
}

func collectReferencedImagesAt(atom *model.Podcast, now time.Time) []referencedImage {
	seen := make(map[string]struct{})
	images := make([]referencedImage, 0)
	appendImage := func(key, label string) {
		key = normalizeStorageKey(atom, key)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		contentType := "image/jpeg"
		if strings.HasSuffix(strings.ToLower(key), ".png") {
			contentType = "image/png"
		}
		images = append(images, referencedImage{Key: key, Label: label, ContentType: contentType})
	}

	appendImage(atom.Config.Image, "podcast")
	renderable, _ := spec.RenderableEpisodes(atom, atom.Episodes)
	for _, episode := range renderable {
		if episode.PubDate.Time.IsZero() || episode.PubDate.Time.After(now) {
			continue
		}
		appendImage(spec.EffectiveEpisodeImage(atom, &episode), fmt.Sprintf("episode %d", episode.UID))
	}
	return images
}

func syncReferencedImagesForPublish(ctx context.Context, atom *model.Podcast, prompter interface {
	Ask(context.Context, string, ...any) bool
}, client publishAssetClient) error {
	l := logger.FromContext(ctx)
	if atom == nil {
		return fmt.Errorf("podcast is nil")
	}
	for _, image := range collectReferencedImages(atom) {
		operation, err := decidePublishImage(ctx, atom, image, client)
		if err != nil {
			return err
		}
		switch operation.Kind {
		case "check-image-upload":
			l.Info("Would publish local image", "label", image.Label, "key", image.Key, "path", operation.LocalPath)
		case "download-image":
			if err := client.DownloadFile(ctx, operation.Bucket, operation.Key); err != nil {
				return fmt.Errorf("failed to download remote %s image %s from bucket %s: %w", image.Label, image.Key, operation.Bucket, err)
			}
		case "upload-image":
			if !prompter.Ask(ctx, "Upload %s image %s to S3?", image.Label, image.Key) {
				return fmt.Errorf("required %s image %s was not uploaded to bucket %s", image.Label, image.Key, atom.Config.Aws.Buckets.Output)
			}
			if err := client.UploadFile(ctx, operation.Bucket, operation.Key, operation.LocalPath, &s3store.UploadOptions{ContentType: image.ContentType}); err != nil {
				return fmt.Errorf("failed to upload %s image %s to bucket %s: %w", image.Label, image.Key, operation.Bucket, err)
			}
		}
	}
	return nil
}

func previewReferencedImagesForPublish(ctx context.Context, atom *model.Podcast, client publishAssetClient) error {
	l := logger.FromContext(ctx)
	if atom == nil {
		return fmt.Errorf("podcast is nil")
	}
	for _, image := range collectReferencedImages(atom) {
		operation, err := decidePublishImage(ctx, atom, image, client)
		if err != nil {
			return err
		}
		switch operation.Kind {
		case "check-image-upload":
			l.Info("Would publish local image", "label", image.Label, "key", image.Key, "path", operation.LocalPath, "reason", operation.Reason)
		case "skip-image-upload":
			l.Info("Image already published", "label", image.Label, "key", image.Key, "path", operation.LocalPath, "reason", operation.Reason)
		case "download-image":
			l.Info("Would download published image", "label", image.Label, "key", image.Key, "path", operation.LocalPath, "reason", operation.Reason)
		case "upload-image":
			if operation.RemoteExists == "true" {
				l.Info("Would overwrite remote image", "label", image.Label, "key", image.Key, "path", operation.LocalPath, "reason", operation.Reason)
			} else {
				l.Info("Would publish local image", "label", image.Label, "key", image.Key, "path", operation.LocalPath, "reason", operation.Reason)
			}
		}
	}
	return nil
}

func prepareEpisodeAssetsForEncode(ctx context.Context, atom *model.Podcast, episode *model.Episode, client assetInfoClient) error {
	if atom == nil || episode == nil {
		return fmt.Errorf("podcast and episode are required")
	}
	buckets := []string{atom.Config.Aws.Buckets.Input, atom.Config.Aws.Buckets.Output}
	if err := syncLocalAssetFromBuckets(ctx, atom, client, buckets, episode.Input, fmt.Sprintf("episode %d input", episode.UID)); err != nil {
		return err
	}
	if err := syncLocalAssetFromBuckets(ctx, atom, client, buckets, atom.Encoding.Coverfront, "cover image"); err != nil {
		return err
	}
	if err := syncLocalAssetFromBuckets(ctx, atom, client, buckets, spec.EffectiveEpisodeImage(atom, episode), fmt.Sprintf("episode %d image", episode.UID)); err != nil {
		return err
	}
	return nil
}

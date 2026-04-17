package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	key = normalizeStorageKey(atom, key)
	if key == "" {
		return nil
	}
	localPath := localAssetPath(atom, key)
	if _, err := os.Stat(localPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to access local %s %s: %w", label, localPath, err)
	}
	if client == nil {
		return fmt.Errorf("missing local %s %s and no remote storage client is configured", label, localPath)
	}

	for _, bucket := range buckets {
		bucket = strings.TrimSpace(bucket)
		if bucket == "" {
			continue
		}
		info, err := client.GetFileInfo(ctx, bucket, key)
		if err != nil {
			return fmt.Errorf("failed to check remote %s %s in bucket %s: %w", label, key, bucket, err)
		}
		if info != nil && info.Exists {
			if err := client.DownloadFile(ctx, bucket, key); err != nil {
				return fmt.Errorf("failed to download remote %s %s from bucket %s: %w", label, key, bucket, err)
			}
			return nil
		}
	}

	return fmt.Errorf("missing %s %s locally and in configured S3 buckets", label, key)
}

func collectReferencedImages(atom *model.Podcast) []referencedImage {
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
	appendImage(atom.Encoding.Coverfront, "cover")
	for _, episode := range atom.Episodes {
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
		localPath := localAssetPath(atom, image.Key)
		fi, err := os.Stat(localPath)
		localExists := err == nil
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to access local %s image %s: %w", image.Label, localPath, err)
		}

		if client == nil {
			if !localExists {
				return fmt.Errorf("missing local %s image %s", image.Label, localPath)
			}
			l.Info("Would publish local image", "label", image.Label, "key", image.Key, "path", localPath)
			continue
		}

		info, err := client.GetFileInfo(ctx, atom.Config.Aws.Buckets.Output, image.Key)
		if err != nil {
			return fmt.Errorf("failed to check remote %s image %s in bucket %s: %w", image.Label, image.Key, atom.Config.Aws.Buckets.Output, err)
		}

		if !localExists {
			if info != nil && info.Exists {
				if err := client.DownloadFile(ctx, atom.Config.Aws.Buckets.Output, image.Key); err != nil {
					return fmt.Errorf("failed to download remote %s image %s from bucket %s: %w", image.Label, image.Key, atom.Config.Aws.Buckets.Output, err)
				}
				continue
			}
			return fmt.Errorf("missing %s image %s locally and in output bucket %s", image.Label, image.Key, atom.Config.Aws.Buckets.Output)
		}

		remoteMatchesLocal := info != nil && info.Exists && info.Size == fi.Size()
		if remoteMatchesLocal {
			continue
		}
		if !prompter.Ask(ctx, "Upload %s image %s to S3?", image.Label, image.Key) {
			return fmt.Errorf("required %s image %s was not uploaded to bucket %s", image.Label, image.Key, atom.Config.Aws.Buckets.Output)
		}
		if err := client.UploadFile(ctx, atom.Config.Aws.Buckets.Output, image.Key, localPath, &s3store.UploadOptions{ContentType: image.ContentType}); err != nil {
			return fmt.Errorf("failed to upload %s image %s to bucket %s: %w", image.Label, image.Key, atom.Config.Aws.Buckets.Output, err)
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

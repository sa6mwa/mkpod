package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
)

func TestDecidePublishImageSkipsMatchingRemoteImage(t *testing.T) {
	workdir := t.TempDir()
	image := referencedImage{Key: filepath.ToSlash(filepath.Join("artwork", "cover.jpg")), Label: "podcast", ContentType: "image/jpeg"}
	localPath := filepath.Join(workdir, filepath.FromSlash(image.Key))
	mkdirAll(t, filepath.Dir(localPath))
	writeFile(t, localPath, []byte("jpeg"))
	atom := feedDecisionPodcast(workdir)
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		image.Key: {Exists: true, Size: int64(len("jpeg"))},
	}}

	operation, err := decidePublishImage(context.Background(), atom, image, storage)
	if err != nil {
		t.Fatalf("decidePublishImage() error = %v", err)
	}
	if operation.Kind != "skip-image-upload" || operation.RequiresPrompt {
		t.Fatalf("operation = %+v, want skip without prompt", operation)
	}
}

func TestDecidePublishImageUploadsDifferentRemoteImage(t *testing.T) {
	workdir := t.TempDir()
	image := referencedImage{Key: filepath.ToSlash(filepath.Join("artwork", "cover.png")), Label: "podcast", ContentType: "image/png"}
	localPath := filepath.Join(workdir, filepath.FromSlash(image.Key))
	mkdirAll(t, filepath.Dir(localPath))
	writeFile(t, localPath, []byte("png"))
	atom := feedDecisionPodcast(workdir)
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		image.Key: {Exists: true, Size: 99},
	}}

	operation, err := decidePublishImage(context.Background(), atom, image, storage)
	if err != nil {
		t.Fatalf("decidePublishImage() error = %v", err)
	}
	if operation.Kind != "upload-image" || !operation.RequiresPrompt {
		t.Fatalf("operation = %+v, want prompted image upload", operation)
	}
}

func TestDecidePublishImageDownloadsMissingLocalImage(t *testing.T) {
	workdir := t.TempDir()
	image := referencedImage{Key: filepath.ToSlash(filepath.Join("artwork", "cover.jpg")), Label: "podcast", ContentType: "image/jpeg"}
	atom := feedDecisionPodcast(workdir)
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		image.Key: {Exists: true, Size: 99},
	}}

	operation, err := decidePublishImage(context.Background(), atom, image, storage)
	if err != nil {
		t.Fatalf("decidePublishImage() error = %v", err)
	}
	if operation.Kind != "download-image" || operation.Bucket != "output" || operation.Key != image.Key {
		t.Fatalf("operation = %+v, want image download", operation)
	}
}

func TestDecideFeedUploadAlwaysRequiresPrompt(t *testing.T) {
	atom := feedDecisionPodcast(t.TempDir())
	atom.FeedFile = "podcast.rss"
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		"podcast.rss": {Exists: true, Size: 123},
	}}

	operation, err := decideFeedUpload(context.Background(), atom, "/tmp/podcast.rss", storage)
	if err != nil {
		t.Fatalf("decideFeedUpload() error = %v", err)
	}
	if operation.Kind != "upload-feed" || !operation.RequiresPrompt || operation.RemoteExists != "true" {
		t.Fatalf("operation = %+v, want prompted feed upload", operation)
	}
}

func feedDecisionPodcast(workdir string) *model.Podcast {
	return &model.Podcast{
		FeedFile: "podcast.rss",
		Config: model.Config{
			LocalStorageDir: workdir,
			Aws: model.AwsConfig{
				Buckets: model.Buckets{
					Output: "output",
				},
			},
		},
	}
}

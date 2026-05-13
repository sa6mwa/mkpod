package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
)

func TestPrepareEpisodeAssetsForEncodeDownloadsMissingRemoteInput(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	coverPath := filepath.Join(workdir, "artwork", "cover.jpg")
	imagePath := filepath.Join(workdir, "artwork", "episode.jpg")
	mkdirAll(t, filepath.Dir(coverPath))
	writeFile(t, coverPath, []byte("jpeg"))
	writeFile(t, imagePath, []byte("jpeg"))

	atom := &model.Podcast{
		Config: model.Config{
			LocalStorageDir: workdir,
			Aws:             model.AwsConfig{Buckets: model.Buckets{Input: "input", Output: "output"}},
		},
	}
	atom.Encoding.Coverfront = filepath.ToSlash(filepath.Join("artwork", "cover.jpg"))
	atom.Config.DefaultPodImage = filepath.ToSlash(filepath.Join("artwork", "episode.jpg"))
	episode := &model.Episode{UID: 1, Title: "Episode", Input: filepath.ToSlash(filepath.Join("masters", "episode.wav"))}
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		episode.Input: {Exists: true},
	}}

	if err := prepareEpisodeAssetsForEncode(ctx, atom, episode, storage); err != nil {
		t.Fatalf("prepareEpisodeAssetsForEncode() error = %v", err)
	}
	if len(storage.downloads) != 1 || storage.downloads[0] != episode.Input {
		t.Fatalf("downloads = %v, want [%s]", storage.downloads, episode.Input)
	}
}

func TestSyncReferencedImagesForPublishDownloadsRemoteEpisodeImage(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	atom := &model.Podcast{
		Config: model.Config{
			BaseURL:         "https://bucket.s3.us-east-1.amazonaws.com",
			Image:           "https://bucket.s3.us-east-1.amazonaws.com/artwork/show.jpg",
			DefaultPodImage: "artwork/episode.jpg",
			LocalStorageDir: workdir,
			Aws:             model.AwsConfig{Region: "us-east-1", Buckets: model.Buckets{Output: "bucket"}},
		},
		Episodes: []model.Episode{{UID: 1, Image: "artwork/episode.jpg"}},
	}
	mkdirAll(t, filepath.Join(workdir, "artwork"))
	writeFile(t, filepath.Join(workdir, "artwork", "show.jpg"), []byte("jpeg"))
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		"artwork/show.jpg":    {Exists: true, Size: int64(len("jpeg"))},
		"artwork/episode.jpg": {Exists: true, Size: 123},
	}}

	if err := syncReferencedImagesForPublish(ctx, atom, &parseTestAsker{answer: true}, storage); err != nil {
		t.Fatalf("syncReferencedImagesForPublish() error = %v", err)
	}
	if len(storage.downloads) != 1 || storage.downloads[0] != "artwork/episode.jpg" {
		t.Fatalf("downloads = %v, want [artwork/episode.jpg]", storage.downloads)
	}
}

func TestSyncReferencedImagesForPublishFailsWhenImageMissingEverywhere(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	atom := &model.Podcast{
		Config: model.Config{
			BaseURL:         "https://bucket.s3.us-east-1.amazonaws.com",
			Image:           "https://bucket.s3.us-east-1.amazonaws.com/artwork/show.jpg",
			LocalStorageDir: workdir,
			Aws:             model.AwsConfig{Region: "us-east-1", Buckets: model.Buckets{Output: "bucket"}},
		},
		Episodes: []model.Episode{{UID: 1, Image: "artwork/missing.jpg"}},
	}
	mkdirAll(t, filepath.Join(workdir, "artwork"))
	writeFile(t, filepath.Join(workdir, "artwork", "show.jpg"), []byte("jpeg"))
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		"artwork/show.jpg":    {Exists: true, Size: int64(len("jpeg"))},
		"artwork/missing.jpg": {Exists: false},
	}}

	if err := syncReferencedImagesForPublish(ctx, atom, &parseTestAsker{answer: true}, storage); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPreviewReferencedImagesForPublishChecksRemoteWithoutMutating(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	atom := &model.Podcast{
		Config: model.Config{
			Image:           "artwork/show.jpg",
			LocalStorageDir: workdir,
			Aws:             model.AwsConfig{Buckets: model.Buckets{Output: "bucket"}},
		},
	}
	mkdirAll(t, filepath.Join(workdir, "artwork"))
	writeFile(t, filepath.Join(workdir, "artwork", "show.jpg"), []byte("jpeg"))
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		"artwork/show.jpg": {Exists: true, Size: int64(len("jpeg")), ETag: `"ab4f3ccba74857c5f2ba0d5b7dbf65e1"`},
	}}

	if err := previewReferencedImagesForPublish(ctx, atom, storage); err != nil {
		t.Fatalf("previewReferencedImagesForPublish() error = %v", err)
	}
	if len(storage.checkedKeys) != 1 || storage.checkedKeys[0] != "artwork/show.jpg" {
		t.Fatalf("checkedKeys = %v, want remote image check", storage.checkedKeys)
	}
	if len(storage.uploads) != 0 {
		t.Fatalf("uploads = %v, want dry-run preview to avoid uploads", storage.uploads)
	}
	if len(storage.downloads) != 0 {
		t.Fatalf("downloads = %v, want dry-run preview to avoid downloads", storage.downloads)
	}
}

func TestLocalAssetPathNormalizesSlashes(t *testing.T) {
	atom := &model.Podcast{Config: model.Config{LocalStorageDir: t.TempDir()}}
	got := localAssetPath(atom, "artwork/cover.jpg")
	if _, err := os.Stat(filepath.Dir(got)); err == nil {
		// nothing to assert about filesystem state; keep the helper test path-only
	}
	if filepath.Base(got) != "cover.jpg" {
		t.Fatalf("localAssetPath() = %q", got)
	}
}

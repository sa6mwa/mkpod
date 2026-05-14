package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		Author: "Host",
		Episodes: []model.Episode{{
			UID:         1,
			Title:       "Published",
			PubDate:     model.ItunesTime{Time: time.Now().Add(-time.Hour)},
			Link:        "https://example.com/1",
			Duration:    model.ItunesDuration{Duration: time.Minute},
			Description: "Published episode",
			Type:        "audio/mpeg",
			Length:      10,
			Image:       "artwork/episode.jpg",
			Output:      "episode.mp3",
		}},
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
			Image:           "https://bucket.s3.us-east-1.amazonaws.com/artwork/missing-show.jpg",
			LocalStorageDir: workdir,
			Aws:             model.AwsConfig{Region: "us-east-1", Buckets: model.Buckets{Output: "bucket"}},
		},
	}
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		"artwork/missing-show.jpg": {Exists: false},
	}}

	if err := syncReferencedImagesForPublish(ctx, atom, &parseTestAsker{answer: true}, storage); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCollectReferencedImagesMatchesGeneratedRSSReferences(t *testing.T) {
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	atom := &model.Podcast{
		Config: model.Config{
			Image:           "artwork/show.jpg",
			DefaultPodImage: "artwork/default.jpg",
		},
	}
	atom.Encoding.Coverfront = "artwork/coverfront.jpg"
	atom.Episodes = []model.Episode{
		{
			UID:         1,
			Title:       "Published",
			PubDate:     model.ItunesTime{Time: now.Add(-time.Hour)},
			Link:        "https://example.com/1",
			Duration:    model.ItunesDuration{Duration: time.Minute},
			Author:      "Host",
			Description: "Published description",
			Type:        "audio/mpeg",
			Length:      10,
			Image:       "artwork/published.jpg",
			Output:      "published.mp3",
		},
		{
			UID:         2,
			Title:       "Future",
			PubDate:     model.ItunesTime{Time: now.Add(time.Hour)},
			Link:        "https://example.com/2",
			Duration:    model.ItunesDuration{Duration: time.Minute},
			Author:      "Host",
			Description: "Future description",
			Type:        "audio/mpeg",
			Length:      10,
			Image:       "artwork/future.jpg",
			Output:      "future.mp3",
		},
		{
			UID:         3,
			Title:       "Published With Default Image",
			PubDate:     model.ItunesTime{Time: now.Add(-time.Hour)},
			Link:        "https://example.com/3",
			Duration:    model.ItunesDuration{Duration: time.Minute},
			Author:      "Host",
			Description: "Published default image description",
			Type:        "audio/mpeg",
			Length:      10,
			Output:      "published-default.mp3",
		},
		{
			UID:         4,
			Title:       "Invalid",
			PubDate:     model.ItunesTime{Time: now.Add(-time.Hour)},
			Author:      "Host",
			Description: "Invalid description",
			Image:       "artwork/invalid.jpg",
		},
	}

	images := collectReferencedImagesAt(atom, now)
	got := make([]string, 0, len(images))
	for _, image := range images {
		got = append(got, image.Key)
	}
	want := []string{"artwork/show.jpg", "artwork/published.jpg", "artwork/default.jpg"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("referenced image keys = %v, want %v", got, want)
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

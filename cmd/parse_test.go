package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
)

type parseTestAsker struct {
	answer bool
}

func (a *parseTestAsker) Ask(_ context.Context, _ string, _ ...any) bool {
	return a.answer
}

type fakeStorageClient struct {
	existsResponses map[string]bool
	checkedKeys     []string
	uploads         []*s3store.UploadRequest
}

func (f *fakeStorageClient) FileExists(_ context.Context, request *s3store.ObjectRequest) (bool, error) {
	f.checkedKeys = append(f.checkedKeys, request.Key)
	return f.existsResponses[request.Key], nil
}

func (f *fakeStorageClient) UploadFile(_ context.Context, request *s3store.UploadRequest) error {
	f.uploads = append(f.uploads, request)
	return nil
}

func TestCheckAndUploadPodcastImageSkipsExistingRemoteImage(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	imageRelPath := filepath.Join("artwork", "podcast-cover.jpg")
	imagePath := filepath.Join(workdir, imageRelPath)
	mkdirAll(t, filepath.Dir(imagePath))
	writeFile(t, imagePath, []byte("jpeg"))

	atom := &model.Podcast{
		Config: model.Config{
			BaseURL:         "https://example.com/show",
			Image:           "https://bucket.s3.us-east-1.amazonaws.com/artwork/podcast-cover.jpg",
			LocalStorageDir: workdir,
			Aws: model.AwsConfig{
				Region: "us-east-1",
				Buckets: model.Buckets{
					Output: "bucket",
				},
			},
		},
	}
	storage := &fakeStorageClient{
		existsResponses: map[string]bool{
			imageRelPath: true,
		},
	}

	err := checkAndUploadPodcastImage(ctx, atom, &parseTestAsker{answer: true}, storage)
	if err != nil {
		t.Fatalf("checkAndUploadPodcastImage() error = %v", err)
	}
	if len(storage.uploads) != 0 {
		t.Fatalf("expected no upload when remote image exists, got %d", len(storage.uploads))
	}
	if len(storage.checkedKeys) != 1 || storage.checkedKeys[0] != imageRelPath {
		t.Fatalf("checkedKeys = %v, want [%s]", storage.checkedKeys, imageRelPath)
	}
}

func TestCheckAndUploadPodcastImageUploadsMissingRemoteImage(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	imageRelPath := filepath.Join("artwork", "podcast-cover.png")
	imagePath := filepath.Join(workdir, imageRelPath)
	mkdirAll(t, filepath.Dir(imagePath))
	writeFile(t, imagePath, []byte("png"))

	atom := &model.Podcast{
		Config: model.Config{
			BaseURL:         "https://example.com/show",
			Image:           "https://bucket.s3.us-east-1.amazonaws.com/artwork/podcast-cover.png",
			LocalStorageDir: workdir,
			Aws: model.AwsConfig{
				Region: "us-east-1",
				Buckets: model.Buckets{
					Output: "bucket",
				},
			},
		},
	}
	storage := &fakeStorageClient{
		existsResponses: map[string]bool{
			imageRelPath: false,
		},
	}

	err := checkAndUploadPodcastImage(ctx, atom, &parseTestAsker{answer: true}, storage)
	if err != nil {
		t.Fatalf("checkAndUploadPodcastImage() error = %v", err)
	}
	if len(storage.uploads) != 1 {
		t.Fatalf("expected one upload, got %d", len(storage.uploads))
	}

	upload := storage.uploads[0]
	if upload.Store != "bucket" {
		t.Fatalf("upload.Store = %q, want bucket", upload.Store)
	}
	if upload.Key != imageRelPath {
		t.Fatalf("upload.Key = %q, want %q", upload.Key, imageRelPath)
	}
	if upload.Filename != imagePath {
		t.Fatalf("upload.Filename = %q, want %q", upload.Filename, imagePath)
	}
	if upload.ContentType != "image/png" {
		t.Fatalf("upload.ContentType = %q, want image/png", upload.ContentType)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

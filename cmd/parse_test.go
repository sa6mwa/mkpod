package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	existsErr       error
	infoResponses   map[string]*s3store.FileInfo
	downloadErr     error
	checkedKeys     []string
	downloads       []string
	uploads         []fakeUpload
}

type fakeUpload struct {
	Store        string
	Key          string
	Filename     string
	ContentType  string
	StorageClass string
}

func (f *fakeStorageClient) FileExists(_ context.Context, bucket, key string) (bool, error) {
	f.checkedKeys = append(f.checkedKeys, key)
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.existsResponses[key], nil
}

func (f *fakeStorageClient) GetFileInfo(_ context.Context, bucket, key string) (*s3store.FileInfo, error) {
	f.checkedKeys = append(f.checkedKeys, key)
	if f.existsErr != nil {
		return nil, f.existsErr
	}
	if f.infoResponses != nil {
		if info, ok := f.infoResponses[key]; ok {
			return info, nil
		}
	}
	return &s3store.FileInfo{Exists: f.existsResponses[key]}, nil
}

func (f *fakeStorageClient) DownloadFile(_ context.Context, bucket, key string) error {
	f.downloads = append(f.downloads, key)
	return f.downloadErr
}

func (f *fakeStorageClient) UploadFile(_ context.Context, bucket, key, filename string, options *s3store.UploadOptions) error {
	f.uploads = append(f.uploads, fakeUpload{Store: bucket, Key: key, Filename: filename, ContentType: func() string {
		if options == nil {
			return ""
		}
		return options.ContentType
	}(), StorageClass: func() string {
		if options == nil {
			return ""
		}
		return options.StorageClass
	}()})
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
		infoResponses: map[string]*s3store.FileInfo{
			imageRelPath: {Exists: true, Size: int64(len("jpeg")), ETag: `"ab4f3ccba74857c5f2ba0d5b7dbf65e1"`},
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

func TestCheckAndUploadPodcastImageIncludesRemoteContextInErrors(t *testing.T) {
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
				Region:  "us-east-1",
				Buckets: model.Buckets{Output: "bucket"},
			},
		},
	}
	storage := &fakeStorageClient{existsErr: errors.New("boom")}

	err := checkAndUploadPodcastImage(ctx, atom, &parseTestAsker{answer: true}, storage)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "remote podcast image") || !strings.Contains(err.Error(), "bucket") || !strings.Contains(err.Error(), imageRelPath) {
		t.Fatalf("unexpected error: %v", err)
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

func TestJoinBaseURLPath(t *testing.T) {
	tests := []struct {
		baseURL string
		relPath string
		want    string
	}{
		{baseURL: "https://example.com/show", relPath: "artwork/cover.jpg", want: "https://example.com/show/artwork/cover.jpg"},
		{baseURL: "https://example.com/show/", relPath: "/artwork/cover.jpg", want: "https://example.com/show/artwork/cover.jpg"},
		{baseURL: "", relPath: "artwork/cover.jpg", want: "artwork/cover.jpg"},
	}

	for _, tt := range tests {
		if got := joinBaseURLPath(tt.baseURL, tt.relPath); got != tt.want {
			t.Fatalf("joinBaseURLPath(%q, %q) = %q, want %q", tt.baseURL, tt.relPath, got, tt.want)
		}
	}
}

func TestCheckAndUploadPodcastImageUsesExpandedLocalStorageDir(t *testing.T) {
	ctx := context.Background()
	home := os.Getenv("HOME")
	if home == "" {
		t.Fatal("HOME is not set")
	}
	workdir := filepath.Join(home, "mkpod-parse-test-home")
	t.Cleanup(func() { _ = os.RemoveAll(workdir) })
	imageRelPath := filepath.Join("artwork", "podcast-cover.jpg")
	imagePath := filepath.Join(workdir, imageRelPath)
	mkdirAll(t, filepath.Dir(imagePath))
	writeFile(t, imagePath, []byte("jpeg"))

	atom := &model.Podcast{
		Config: model.Config{
			BaseURL:         "https://example.com/show/",
			Image:           "https://bucket.s3.us-east-1.amazonaws.com/artwork/podcast-cover.jpg",
			LocalStorageDir: "~/mkpod-parse-test-home",
			Aws:             model.AwsConfig{Region: "us-east-1", Buckets: model.Buckets{Output: "bucket"}},
		},
	}
	storage := &fakeStorageClient{existsResponses: map[string]bool{imageRelPath: false}}

	err := checkAndUploadPodcastImage(ctx, atom, &parseTestAsker{answer: true}, storage)
	if err != nil {
		t.Fatalf("checkAndUploadPodcastImage() error = %v", err)
	}
	if len(storage.uploads) != 1 {
		t.Fatalf("expected one upload, got %d", len(storage.uploads))
	}
	if storage.uploads[0].Filename != imagePath {
		t.Fatalf("upload.Filename = %q, want %q", storage.uploads[0].Filename, imagePath)
	}
}

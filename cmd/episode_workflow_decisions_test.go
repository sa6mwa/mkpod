package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
)

func TestDecideLocalAssetSyncSkipsExistingLocalAsset(t *testing.T) {
	workdir := t.TempDir()
	key := filepath.ToSlash(filepath.Join("masters", "episode.wav"))
	localPath := filepath.Join(workdir, filepath.FromSlash(key))
	mkdirAll(t, filepath.Dir(localPath))
	writeFile(t, localPath, []byte("audio"))
	atom := episodeDecisionPodcast(workdir)
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		key: {Exists: true, Size: 100},
	}}

	operation, err := decideLocalAssetSync(context.Background(), atom, storage, []string{"input"}, key, "episode input")
	if err != nil {
		t.Fatalf("decideLocalAssetSync() error = %v", err)
	}
	if operation.Kind != "skip-asset" || operation.Reason != "local asset exists" {
		t.Fatalf("operation = %+v, want local skip", operation)
	}
	if len(storage.checkedKeys) != 0 {
		t.Fatalf("checkedKeys = %v, want no remote checks for existing local asset", storage.checkedKeys)
	}
}

func TestDecideLocalAssetSyncDownloadsMissingRemoteAsset(t *testing.T) {
	workdir := t.TempDir()
	key := filepath.ToSlash(filepath.Join("masters", "episode.wav"))
	atom := episodeDecisionPodcast(workdir)
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		key: {Exists: true, Size: 100},
	}}

	operation, err := decideLocalAssetSync(context.Background(), atom, storage, []string{"input"}, key, "episode input")
	if err != nil {
		t.Fatalf("decideLocalAssetSync() error = %v", err)
	}
	if operation.Kind != "download-asset" || operation.Bucket != "input" || operation.Key != key {
		t.Fatalf("operation = %+v, want download from input bucket", operation)
	}
}

func TestDecideOutputUploadPromptsForMissingRemoteOutput(t *testing.T) {
	workdir := t.TempDir()
	atom := episodeDecisionPodcast(workdir)
	episode := &model.Episode{Output: "episode.mp3"}
	localPath := filepath.Join(workdir, episode.Output)
	writeFile(t, localPath, []byte("mp3"))
	storage := &fakeStorageClient{existsResponses: map[string]bool{
		episode.Output: false,
	}}

	operation, err := decideOutputUpload(context.Background(), atom, episode, storage, false)
	if err != nil {
		t.Fatalf("decideOutputUpload() error = %v", err)
	}
	if operation.Kind != "upload-output" || !operation.RequiresPrompt || operation.RemoteExists != "false" {
		t.Fatalf("operation = %+v, want prompted upload for missing remote output", operation)
	}
}

func TestDecideOutputUploadSkipsExistingRemoteOutputWithoutReencode(t *testing.T) {
	workdir := t.TempDir()
	atom := episodeDecisionPodcast(workdir)
	episode := &model.Episode{Output: "episode.mp3"}
	writeFile(t, filepath.Join(workdir, episode.Output), []byte("mp3"))
	storage := &fakeStorageClient{existsResponses: map[string]bool{
		episode.Output: true,
	}}

	operation, err := decideOutputUpload(context.Background(), atom, episode, storage, false)
	if err != nil {
		t.Fatalf("decideOutputUpload() error = %v", err)
	}
	if operation.Kind != "skip-output-upload" || operation.RequiresPrompt {
		t.Fatalf("operation = %+v, want skip without prompt", operation)
	}
}

func TestDecideRemoteMasterRemovalRequiresAllowedSafetyAndPrompt(t *testing.T) {
	workdir := t.TempDir()
	atom := episodeDecisionPodcast(workdir)
	episode := &model.Episode{Input: filepath.ToSlash(filepath.Join("masters", "episode.wav"))}
	localPath := filepath.Join(workdir, filepath.FromSlash(episode.Input))
	mkdirAll(t, filepath.Dir(localPath))
	writeFile(t, localPath, []byte("0123456789"))
	storage := &fakeStorageClient{infoResponses: map[string]*s3store.FileInfo{
		episode.Input: {Exists: true, Size: 10},
	}}

	operation, err := decideRemoteMasterRemoval(context.Background(), atom, episode, storage)
	if err != nil {
		t.Fatalf("decideRemoteMasterRemoval() error = %v", err)
	}
	if operation.Kind != "delete-remote-master" || operation.SafetyStatus != s3store.RemovalAllowed || !operation.RequiresPrompt {
		t.Fatalf("operation = %+v, want allowed prompted delete", operation)
	}
}

func episodeDecisionPodcast(workdir string) *model.Podcast {
	return &model.Podcast{
		Config: model.Config{
			LocalStorageDir: workdir,
			Aws: model.AwsConfig{
				Buckets: model.Buckets{
					Input:  "input",
					Output: "output",
				},
			},
		},
	}
}

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	workflow "github.com/sa6mwa/mkpod/internal/workflow"
	"github.com/spf13/cobra"
)

func TestPlannedContentType(t *testing.T) {
	tests := map[string]string{
		"episode.mp3": "audio/mpeg",
		"episode.m4a": "audio/mp4",
		"episode.m4b": "audio/mp4",
		"episode.mp4": "video/mp4",
		"episode.wav": "",
	}
	for filename, want := range tests {
		if got := plannedContentType(filename); got != want {
			t.Fatalf("plannedContentType(%q) = %q, want %q", filename, got, want)
		}
	}
}

type fakeRemoteChecker struct {
	exists bool
	err    error
}

func (f fakeRemoteChecker) FileExists(context.Context, string, string) (bool, error) {
	return f.exists, f.err
}

type fakeApplyStorage struct {
	infos     map[string]*s3store.FileInfo
	uploads   []string
	downloads []string
}

func (f *fakeApplyStorage) GetFileInfo(_ context.Context, bucket, key string) (*s3store.FileInfo, error) {
	if f.infos == nil {
		return &s3store.FileInfo{Exists: false}, nil
	}
	if info, ok := f.infos[bucket+"/"+key]; ok {
		return info, nil
	}
	return &s3store.FileInfo{Exists: false}, nil
}

func (f *fakeApplyStorage) DownloadFile(_ context.Context, bucket, key string) error {
	f.downloads = append(f.downloads, bucket+"/"+key)
	return nil
}

func (f *fakeApplyStorage) FileExists(_ context.Context, bucket, key string) (bool, error) {
	info, err := f.GetFileInfo(context.Background(), bucket, key)
	if err != nil {
		return false, err
	}
	return info.Exists, nil
}

func (f *fakeApplyStorage) UploadFile(_ context.Context, bucket, key, filename string, _ *s3store.UploadOptions) error {
	f.uploads = append(f.uploads, bucket+"/"+key+"="+filename)
	return nil
}

func TestFillRemoteObjectPlan(t *testing.T) {
	plan := remoteObjectPlan{Bucket: "bucket", Key: "episode.m4a"}
	fillRemoteObjectPlan(context.Background(), fakeRemoteChecker{exists: true}, &plan)
	if plan.Exists != "true" || plan.Error != "" {
		t.Fatalf("remote plan = %+v, want exists true without error", plan)
	}

	plan = remoteObjectPlan{Bucket: "bucket", Key: "episode.m4a"}
	fillRemoteObjectPlan(context.Background(), fakeRemoteChecker{}, &plan)
	if plan.Exists != "false" || plan.Error != "" {
		t.Fatalf("remote plan = %+v, want exists false without error", plan)
	}

	plan = remoteObjectPlan{Bucket: "bucket", Key: "episode.m4a"}
	fillRemoteObjectPlan(context.Background(), fakeRemoteChecker{err: errors.New("boom")}, &plan)
	if plan.Exists != "unknown" || plan.Error != "boom" {
		t.Fatalf("remote plan = %+v, want unknown with error", plan)
	}
}

func TestDefaultPlanPath(t *testing.T) {
	if got, want := defaultPlanPath("preprocess", ""), filepath.Join(".", "preprocess.plan.json"); got != want {
		t.Fatalf("defaultPlanPath() = %q, want %q", got, want)
	}
	if got, want := defaultPlanPath("episode", filepath.Join("show", "podspec.yaml")), filepath.Join("show", "episode.plan.json"); got != want {
		t.Fatalf("defaultPlanPath() = %q, want %q", got, want)
	}
}

func TestApplyCommandHasNoWorkflowSubcommands(t *testing.T) {
	if got := len(applyCmd.Commands()); got != 0 {
		t.Fatalf("apply subcommands = %d, want none", got)
	}
	if got, want := applyCmd.Use, "apply <plan.json>"; got != want {
		t.Fatalf("apply Use = %q, want %q", got, want)
	}
	for _, flag := range []string{"just-master", "reencode", "yes", "force"} {
		if applyCmd.Flags().Lookup(flag) == nil {
			t.Fatalf("apply flag %q is missing", flag)
		}
	}
	if help := applyCmd.Flags().Lookup("yes").Usage; !strings.Contains(help, "destructive overwrite") || !strings.Contains(help, "--force") {
		t.Fatalf("apply --yes help = %q, want destructive/force semantics", help)
	}
}

func TestApplyOptionsRejectJustMasterWithReencode(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("just-master", true, "")
	cmd.Flags().Bool("reencode", true, "")
	cmd.Flags().BoolP("yes", "y", false, "")
	cmd.Flags().BoolP("force", "f", false, "")

	_, err := applySavedPlanOptionsFromCommand(cmd)
	if err == nil {
		t.Fatal("applySavedPlanOptionsFromCommand() error = nil, want invalid flag combination")
	}
	if !strings.Contains(err.Error(), "--reencode cannot be combined with --just-master") {
		t.Fatalf("applySavedPlanOptionsFromCommand() error = %v", err)
	}
}

func TestPlanApplyPathHintPreservesCurrentPlanPath(t *testing.T) {
	planPath := filepath.Join(t.TempDir(), "renew-34.plan.json")
	if got := planApplyPathHint(&newEpisodePlan{SpecFile: "podspec.yaml"}, planPath); got != planPath {
		t.Fatalf("planApplyPathHint() = %q, want %q", got, planPath)
	}
}

func TestInspectSavedNewEpisodePlan(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	planPath := filepath.Join(t.TempDir(), "new.plan.json")
	writeSavedPlanFixture(t, planPath, "new", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	})

	if err := inspectSavedPlan(planPath); err != nil {
		t.Fatalf("inspectSavedPlan() error = %v", err)
	}
}

func TestApplySavedNewEpisodePlanJustMasterWritesMetadataAndSyncsAssets(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	writeFile(t, filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.wav"), []byte("master"))
	planPath := filepath.Join(t.TempDir(), "new.plan.json")
	writeSavedPlanFixture(t, planPath, "new", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	})
	storage := &fakeApplyStorage{}

	if err := applySavedPlanWithOptions(context.Background(), planPath, applySavedPlanOptions{JustMaster: true, Yes: true, Storage: storage}); err != nil {
		t.Fatalf("applySavedPlanWithOptions() error = %v", err)
	}
	atom, err = specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("reload spec fixture: %v", err)
	}
	if got := atom.ContainsEpisode(2); got != 0 {
		t.Fatalf("new episode index = %d, want 0", got)
	}
	if atom.Episodes[0].Output != "" {
		t.Fatalf("just-master output = %q, want empty before encode", atom.Episodes[0].Output)
	}
	joinedUploads := strings.Join(storage.uploads, "\n")
	for _, want := range []string{"input/masters/new.wav=", "input/artwork/cover.jpg="} {
		if !strings.Contains(joinedUploads, want) {
			t.Fatalf("uploads = %v, want %q", storage.uploads, want)
		}
	}
	if _, err := os.Stat(atom.FeedFilePath()); !os.IsNotExist(err) {
		t.Fatalf("just-master RSS stat error = %v, want missing RSS", err)
	}
}

func TestApplySavedNewEpisodePlanJustMasterForceMissingLocalMasterDoesNotWriteMetadata(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	planPath := filepath.Join(t.TempDir(), "new.plan.json")
	writeSavedPlanFixture(t, planPath, "new", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	})
	storage := &fakeApplyStorage{infos: map[string]*s3store.FileInfo{
		"input/masters/new.wav": {Exists: true, Size: 100},
	}}

	err := applySavedPlanWithOptions(context.Background(), planPath, applySavedPlanOptions{JustMaster: true, Force: true, Storage: storage})
	if err == nil {
		t.Fatal("applySavedPlanWithOptions() error = nil, want forced missing local master error")
	}
	if !strings.Contains(err.Error(), "forced master sync requires local master") {
		t.Fatalf("applySavedPlanWithOptions() error = %v, want forced local master error", err)
	}
	atom, loadErr := specStoreLoadForTest(t, specFile)
	if loadErr != nil {
		t.Fatalf("reload spec fixture: %v", loadErr)
	}
	if atom.ContainsEpisode(2) >= 0 {
		t.Fatal("new episode metadata was written despite blocked preflight")
	}
}

func TestApplyNewEpisodeFullDoesNotUploadProductionOutsideDecision(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	writeSilentWaveForNewEpisodeTest(t, filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.wav"))
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	decision := workflow.Decision{
		State: workflow.StateRSSReady,
		Operations: []workflow.Operation{
			{
				Kind:   workflow.OperationWriteMetadata,
				Label:  "podspec metadata",
				Reason: "planned metadata is not yet present",
			},
			{
				Kind:       workflow.OperationEncode,
				ObjectKind: workflow.ObjectProductionAudio,
				Label:      "production audio",
				Reason:     "production audio is missing",
			},
		},
	}
	storage := &fakeApplyStorage{}

	if err := applyNewEpisodeFull(context.Background(), plan, decision, applySavedPlanOptions{Yes: true}, storage, false); err != nil {
		t.Fatalf("applyNewEpisodeFull() error = %v", err)
	}
	if len(storage.uploads) != 0 {
		t.Fatalf("uploads = %v, want no uploads without explicit upload-production operation", storage.uploads)
	}
}

func TestApplySavedUnsupportedWorkflowRejects(t *testing.T) {
	planPath := filepath.Join(t.TempDir(), "plan.json")
	writeSavedPlanFixture(t, planPath, "preprocess", map[string]string{"input": "raw.wav"})

	err := applySavedPlan(context.Background(), planPath)
	if err == nil {
		t.Fatal("applySavedPlan() error = nil, want unsupported workflow error")
	}
	if !strings.Contains(err.Error(), "not supported by apply") {
		t.Fatalf("applySavedPlan() error = %v, want unsupported workflow error", err)
	}
}

func TestValidateSavedEpisodePlanRejectsStalePlan(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	plan, err := buildEpisodePlanFromOptions(context.Background(), specFile, false, false, 1)
	if err != nil {
		t.Fatalf("buildEpisodePlanFromOptions() error = %v", err)
	}
	plan.Title = "Changed"

	err = validateSavedEpisodePlan(context.Background(), plan)
	if err == nil {
		t.Fatal("validateSavedEpisodePlan() error = nil, want stale plan error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("validateSavedEpisodePlan() error = %v, want stale plan error", err)
	}
}

func TestValidateSavedFeedPlanRejectsStalePlan(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	plan, err := buildFeedPlanFromOptions(context.Background(), specFile, false, false)
	if err != nil {
		t.Fatalf("buildFeedPlanFromOptions() error = %v", err)
	}
	plan.ValidEpisodes++

	err = validateSavedFeedPlan(context.Background(), plan)
	if err == nil {
		t.Fatal("validateSavedFeedPlan() error = nil, want stale plan error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("validateSavedFeedPlan() error = %v, want stale plan error", err)
	}
}

func writeSavedPlanFixture(t *testing.T, path, workflow string, plan any) {
	t.Helper()
	planContent, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("Marshal(plan): %v", err)
	}
	content, err := json.Marshal(savedPlan{Workflow: workflow, Plan: planContent})
	if err != nil {
		t.Fatalf("Marshal(savedPlan): %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func writeWorkflowSpecFixture(t *testing.T) string {
	t.Helper()
	workdir := t.TempDir()
	mkdirAll(t, filepath.Join(workdir, "masters"))
	mkdirAll(t, filepath.Join(workdir, "artwork"))
	writeFile(t, filepath.Join(workdir, "masters", "episode.wav"), []byte("RIFF\x24\x00\x00\x00WAVEfmt "))
	writeFile(t, filepath.Join(workdir, "episode.mp3"), []byte("ID3"))
	writeFile(t, filepath.Join(workdir, "artwork", "cover.jpg"), []byte("jpeg"))
	specFile := filepath.Join(workdir, "podspec.yaml")
	content := `config:
  baseURL: https://example.com/podcast
  image: https://example.com/podcast/artwork/cover.jpg
  defaultPodImage: artwork/cover.jpg
  aws:
    region: us-east-1
    buckets:
      input: input
      output: output
  localStorageDir: ` + workdir + `
atom: podcast.rss
title: Test Podcast
ttl: 60
language: en
copyright: Copyright Test
webMaster: webmaster@example.com
description: Test description
subtitle: Test subtitle
ownerName: Owner
ownerEmail: owner@example.com
author: Host
encoding:
  coverfront: artwork/cover.jpg
episodes:
- uid: 1
  title: Episode
  input: masters/episode.wav
  output: episode.mp3
  image: artwork/cover.jpg
`
	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", specFile, err)
	}
	return specFile
}

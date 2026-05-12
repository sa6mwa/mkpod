package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sa6mwa/mkpod/internal/blenderaddon"
	"github.com/sa6mwa/mkpod/internal/media/preprocess"
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

func TestApplySavedPreprocessPlanRejectsStalePlan(t *testing.T) {
	plan := preprocessPlanFixture(t)
	plan.Prefix = "changed-"
	planPath := filepath.Join(t.TempDir(), "plan.json")
	writeSavedPlanFixture(t, planPath, "preprocess", plan)

	err := applySavedPlan(context.Background(), planPath)
	if err == nil {
		t.Fatal("applySavedPlan() error = nil, want stale plan error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("applySavedPlan() error = %v, want stale plan error", err)
	}
}

func TestApplySavedBlenderPlanRunsInstaller(t *testing.T) {
	tool, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("test requires sh on PATH: %v", err)
	}
	plan, err := blenderaddon.BuildPlan(blenderaddon.Options{Blender: tool})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	writeSavedPlanFixture(t, planPath, "blender", plan)
	runner := &savedBlenderRunner{}

	if err := applySavedPlanWithBlenderRunner(context.Background(), planPath, runner); err != nil {
		t.Fatalf("applySavedPlanWithBlenderRunner() error = %v", err)
	}
	if runner.name != tool {
		t.Fatalf("runner name = %q, want %q", runner.name, tool)
	}
	wantPrefix := []string{"--command", "extension", "install-file", "-r", "user_default", "-e"}
	if len(runner.args) != len(wantPrefix)+1 {
		t.Fatalf("runner args = %v, want %v <package>", runner.args, wantPrefix)
	}
	for i := range wantPrefix {
		if runner.args[i] != wantPrefix[i] {
			t.Fatalf("runner args = %v, want %v <package>", runner.args, wantPrefix)
		}
	}
	if !strings.HasSuffix(runner.args[len(runner.args)-1], ".zip") {
		t.Fatalf("runner package arg = %q, want zip package", runner.args[len(runner.args)-1])
	}
}

func TestApplySavedBlenderPlanRejectsStalePlan(t *testing.T) {
	tool, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("test requires sh on PATH: %v", err)
	}
	plan, err := blenderaddon.BuildPlan(blenderaddon.Options{Blender: tool})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	plan.AddonModule = "changed"
	planPath := filepath.Join(t.TempDir(), "plan.json")
	writeSavedPlanFixture(t, planPath, "blender", plan)

	err = applySavedPlanWithBlenderRunner(context.Background(), planPath, &savedBlenderRunner{})
	if err == nil {
		t.Fatal("applySavedPlanWithBlenderRunner() error = nil, want stale plan error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("applySavedPlanWithBlenderRunner() error = %v, want stale plan error", err)
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

func TestApplySavedFeedPlanWritesFeed(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	plan, err := buildFeedPlanFromOptions(context.Background(), specFile, false, false)
	if err != nil {
		t.Fatalf("buildFeedPlanFromOptions() error = %v", err)
	}
	planPath := filepath.Join(t.TempDir(), "feed.plan.json")
	writeSavedPlanFixture(t, planPath, "feed", plan)

	if err := applySavedPlan(context.Background(), planPath); err != nil {
		t.Fatalf("applySavedPlan() error = %v", err)
	}
	if _, err := os.Stat(plan.FeedPath); err != nil {
		t.Fatalf("expected feed to be written at %s: %v", plan.FeedPath, err)
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

type savedBlenderRunner struct {
	name string
	args []string
}

func (r *savedBlenderRunner) Run(_ context.Context, name string, args ...string) error {
	r.name = name
	r.args = append([]string(nil), args...)
	return nil
}

func preprocessPlanFixture(t *testing.T) *preprocess.Plan {
	t.Helper()
	plan, err := preprocess.New(&preprocess.Config{Tool: "sh", Preset: "sm7b", Prefix: "pre-"}).Plan([]string{"raw.wav"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	return plan
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

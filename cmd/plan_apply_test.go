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

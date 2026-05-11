package blenderaddon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) error {
	r.name = name
	r.args = append([]string(nil), args...)
	return nil
}

func TestBuildPlanRequiresBlender(t *testing.T) {
	_, err := BuildPlan(Options{Blender: "mkpod-definitely-missing-blender"})
	if err == nil {
		t.Fatal("BuildPlan() error = nil, want missing tool error")
	}
	if !strings.Contains(err.Error(), "required tool not found") {
		t.Fatalf("BuildPlan() error = %v, want missing tool message", err)
	}
}

func TestBuildPlanDescribesMarkerExporterInstall(t *testing.T) {
	tool, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("test requires sh on PATH: %v", err)
	}

	plan, err := BuildPlan(Options{Blender: tool})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if plan.AddonModule != "mkpod_export_markers" {
		t.Fatalf("AddonModule = %q, want mkpod_export_markers", plan.AddonModule)
	}
	if plan.Repo != "user_default" {
		t.Fatalf("Repo = %q, want user_default", plan.Repo)
	}
	if len(plan.Actions) == 0 {
		t.Fatal("Actions is empty")
	}
}

func TestApplyRunsBlenderExtensionInstallFileCommand(t *testing.T) {
	tool, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("test requires sh on PATH: %v", err)
	}
	runner := &recordingRunner{}

	plan, err := Apply(context.Background(), Options{Blender: tool}, runner)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if plan.BlenderPath != tool {
		t.Fatalf("BlenderPath = %q, want %q", plan.BlenderPath, tool)
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

func TestExtensionZipContainsManifestAndAddon(t *testing.T) {
	extensionPackage, err := extensionZip()
	if err != nil {
		t.Fatalf("extensionZip() error = %v", err)
	}
	if len(extensionPackage) == 0 {
		t.Fatal("extensionZip() returned empty package")
	}
}

func TestExtensionZipValidatesWithBlender(t *testing.T) {
	blender, err := exec.LookPath("blender")
	if err != nil {
		t.Skipf("blender unavailable: %v", err)
	}

	extensionPackage, err := extensionZip()
	if err != nil {
		t.Fatalf("extensionZip() error = %v", err)
	}
	packagePath := filepath.Join(t.TempDir(), extensionFilename)
	if err := os.WriteFile(packagePath, extensionPackage, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", packagePath, err)
	}

	cmd := exec.Command(blender, "--command", "extension", "validate", packagePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("blender extension validate failed: %v\nOutput: %s", err, output)
	}
}

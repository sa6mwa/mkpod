package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sa6mwa/mkpod/internal/spec"
)

func TestInitWorkspaceCreatesStarterProject(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "podcast")

	createdDir, err := initWorkspace(target)
	if err != nil {
		t.Fatalf("initWorkspace() error = %v", err)
	}

	if createdDir != target {
		t.Fatalf("initWorkspace() dir = %q, want %q", createdDir, target)
	}

	for _, path := range []string{
		target,
		filepath.Join(target, "artwork"),
		filepath.Join(target, "masters"),
		filepath.Join(target, spec.DefaultSpecfile),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	store := spec.New(filepath.Join(target, spec.DefaultSpecfile))
	spec, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("generated podspec should load: %v", err)
	}

	if spec.Config.LocalStorageDir != target {
		t.Fatalf("LocalStorageDir = %q, want %q", spec.Config.LocalStorageDir, target)
	}
	if spec.Encoding.FFmpegPath != "ffmpeg" {
		t.Fatalf("FFmpegPath = %q, want ffmpeg", spec.Encoding.FFmpegPath)
	}
	if spec.Encoding.Lamepath != "lame" {
		t.Fatalf("Lamepath = %q, want lame", spec.Encoding.Lamepath)
	}
}

func TestInitWorkspaceRejectsNonEmptyDir(t *testing.T) {
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := initWorkspace(target)
	if !errors.Is(err, ErrTargetNotEmpty) {
		t.Fatalf("initWorkspace() error = %v, want %v", err, ErrTargetNotEmpty)
	}
}

func TestResolveWorkspacePathRequiresArgument(t *testing.T) {
	_, err := resolveWorkspacePath("")
	if !errors.Is(err, ErrTargetDirRequired) {
		t.Fatalf("resolveWorkspacePath(\"\") error = %v, want %v", err, ErrTargetDirRequired)
	}
}

func TestResolveWorkspacePathExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("UserHomeDir unavailable: %v", err)
	}

	got, err := resolveWorkspacePath("~/mkpod-test-home")
	if err != nil {
		t.Fatalf("resolveWorkspacePath() error = %v", err)
	}

	want := filepath.Join(home, "mkpod-test-home")
	if got != want {
		t.Fatalf("resolveWorkspacePath() = %q, want %q", got, want)
	}
}

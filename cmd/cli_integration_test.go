package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommandInitCreatesWorkspace(t *testing.T) {
	target := filepath.Join(t.TempDir(), "show")

	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"init", target})

	if _, err := rootCmd.ExecuteC(); err != nil {
		t.Fatalf("ExecuteC() error = %v, stderr = %s", err, stderr.String())
	}

	if _, err := os.Stat(filepath.Join(target, "podspec.yaml")); err != nil {
		t.Fatalf("expected podspec.yaml to exist: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Initialized mkpod workspace") {
		t.Fatalf("stdout %q does not include init confirmation", output)
	}
	if !strings.Contains(output, "Next steps:") {
		t.Fatalf("stdout %q does not include next steps", output)
	}
}

func TestRootCommandInitForceAllowsExistingWorkspace(t *testing.T) {
	target := t.TempDir()
	existingFile := filepath.Join(target, "notes.txt")
	if err := os.WriteFile(existingFile, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"init", "--force", target})

	if _, err := rootCmd.ExecuteC(); err != nil {
		t.Fatalf("ExecuteC() error = %v, stderr = %s", err, stderr.String())
	}

	for _, path := range []string{
		existingFile,
		filepath.Join(target, "podspec.yaml"),
		filepath.Join(target, "artwork"),
		filepath.Join(target, "masters"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist after forced init: %v", path, err)
		}
	}
}

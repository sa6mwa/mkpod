package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLICommands tests that the main commands are available and respond correctly
func TestCLICommands(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "help shows available commands",
			args:     []string{"--help"},
			expected: "Available Commands:",
		},
		{
			name:     "preprocess help",
			args:     []string{"preprocess", "--help"},
			expected: "mkpod preprocess is intended to be used before editing",
		},
		{
			name:     "parse help",
			args:     []string{"parse", "--help"},
			expected: "Parse the podcast specification file",
		},
		{
			name:     "encode help",
			args:     []string{"encode", "--help"},
			expected: "Encode and upload episode media defined in the podcast specification",
		},
		{
			name:     "encode help shows remove-remote-master flag",
			args:     []string{"encode", "--help"},
			expected: "--remove-remote-master",
		},
		{
			name:     "init help",
			args:     []string{"init", "--help"},
			expected: "Initialize a new mkpod workspace",
		},
		{
			name:     "root help includes examples",
			args:     []string{"--help"},
			expected: "Examples:",
		},
		{
			name:     "parse help includes upload example",
			args:     []string{"parse", "--help"},
			expected: "mkpod parse --spec ./podcast/podspec.yaml --upload",
		},
		{
			name:     "encode help includes remove-remote-master example",
			args:     []string{"encode", "--help"},
			expected: "mkpod encode --spec ./podcast/podspec.yaml --all --remove-remote-master",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", append([]string{"run", "."}, tt.args...)...)
			output, err := cmd.CombinedOutput()
			if err != nil && !strings.Contains(string(output), tt.expected) {
				t.Fatalf("Command failed: %v\nOutput: %s", err, output)
			}

			if !strings.Contains(string(output), tt.expected) {
				t.Errorf("Expected output to contain %q, got: %s", tt.expected, output)
			}
		})
	}
}

// TestCommandAliases tests that command aliases work correctly
func TestCommandAliases(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "preprocess alias pre",
			args:     []string{"pre", "--help"},
			expected: "mkpod preprocess is intended to be used before editing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", append([]string{"run", "."}, tt.args...)...)
			output, err := cmd.CombinedOutput()
			if err != nil && !strings.Contains(string(output), tt.expected) {
				t.Fatalf("Command failed: %v\nOutput: %s", err, output)
			}

			if !strings.Contains(string(output), tt.expected) {
				t.Errorf("Expected output to contain %q, got: %s", tt.expected, output)
			}
		})
	}
}

func TestInitThenParseDryRun(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if _, err := cmdTest(target, "init", target); err != nil {
		t.Fatalf("mkpod init failed: %v", err)
	}

	output, err := cmdTest(target, "parse", "--spec", filepath.Join(target, "podspec.yaml"), "--dry-run")
	if err != nil {
		t.Fatalf("mkpod parse --dry-run failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "<rss") {
		t.Fatalf("expected RSS output, got: %s", output)
	}
	if !strings.Contains(output, "<title>"+filepath.Base(target)+"</title>") {
		t.Fatalf("expected podcast title in RSS output, got: %s", output)
	}

	specPath := filepath.Join(target, "podspec.yaml")
	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec before second dry-run: %v", err)
	}

	if _, err := cmdTest("", "parse", "--spec", specPath, "--dry-run"); err != nil {
		t.Fatalf("second mkpod parse --dry-run failed: %v", err)
	}

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec after second dry-run: %v", err)
	}

	if string(before) != string(after) {
		t.Fatalf("parse --dry-run modified podspec.yaml")
	}
}

func TestEncodeFailsForEpisodeMissingTitle(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if _, err := cmdTest("", "init", target); err != nil {
		t.Fatalf("mkpod init failed: %v", err)
	}

	specPath := filepath.Join(target, "podspec.yaml")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	updated := strings.Replace(string(specContent), "episodes: []", `episodes:
    - uid: 1
      pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
      link: "https://example.com/my-podcast/episodes/1"
      subtitle: "Episode without title"
      description: "This episode is intentionally invalid for testing"
      image: "artwork/podcast-cover.jpg"
      input: "masters/episode.wav"
`, 1)

	if err := os.WriteFile(specPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write updated spec: %v", err)
	}

	output, err := cmdTest("", "encode", "--spec", specPath, "1")
	if err == nil {
		t.Fatalf("mkpod encode unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "episode title is required for encoding") {
		t.Fatalf("expected missing title error, got: %s", output)
	}
}

func TestEncodeRequiresUIDOrAll(t *testing.T) {
	output, err := cmdTest("", "encode")
	if err == nil {
		t.Fatalf("mkpod encode unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "select one or more episode UIDs or use --all") {
		t.Fatalf("expected encode usage error, got: %s", output)
	}
}

func TestParseRejectsPositionalArguments(t *testing.T) {
	output, err := cmdTest("", "parse", "extra")
	if err == nil {
		t.Fatalf("mkpod parse unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "parse does not take positional arguments") {
		t.Fatalf("expected parse argument error, got: %s", output)
	}
}
func TestPreprocessFailsWithoutInputFiles(t *testing.T) {
	output, err := cmdTest("", "preprocess")
	if err == nil {
		t.Fatalf("mkpod preprocess unexpectedly succeeded\nOutput: %s", output)
	}
	if !strings.Contains(output, "provide one or more audio files to preprocess") {
		t.Fatalf("expected preprocess argument error, got: %s", output)
	}
}

func TestParseSkipsInvalidEpisodesButSucceeds(t *testing.T) {
	target := filepath.Join(t.TempDir(), "podcast")

	if _, err := cmdTest("", "init", target); err != nil {
		t.Fatalf("mkpod init failed: %v", err)
	}

	specPath := filepath.Join(target, "podspec.yaml")
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	updated := strings.Replace(string(specContent), "episodes: []", `episodes:
    - uid: 1
      title: "Good Episode"
      pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
      link: "https://example.com/my-podcast/episodes/1"
      duration: "00:05:00"
      subtitle: "Valid episode"
      description: "This one should render"
      type: "audio/mpeg"
      length: 12345
      image: "artwork/podcast-cover.jpg"
      output: "episode1.mp3"
    - uid: 2
      title: "Bad Episode"
      pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
      link: "https://example.com/my-podcast/episodes/2"
      subtitle: "Invalid episode"
      description: "Missing output, duration, type, length, image"
`, 1)

	if err := os.WriteFile(specPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write updated spec: %v", err)
	}

	output, err := cmdTest("", "parse", "--spec", specPath, "--dry-run")
	if err != nil {
		t.Fatalf("mkpod parse unexpectedly failed\nOutput: %s", output)
	}
	if !strings.Contains(output, "Excluding episode from RSS due to missing required fields") {
		t.Fatalf("expected warning about skipped invalid episode, got: %s", output)
	}
	if !strings.Contains(output, "<title>Good Episode</title>") {
		t.Fatalf("expected valid episode in RSS output, got: %s", output)
	}
	if strings.Contains(output, "<title>Bad Episode</title>") {
		t.Fatalf("invalid episode should not be rendered in RSS output: %s", output)
	}
}

func cmdTest(_ string, args ...string) (string, error) {
	command := exec.Command("go", append([]string{"run", "."}, args...)...)
	output, err := command.CombinedOutput()
	return string(output), err
}

package main

import (
	"os/exec"
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
			expected: "Encode and upload single or all output files",
		},
		{
			name:     "encode help shows remove-remote-master flag",
			args:     []string{"encode", "--help"},
			expected: "--remove-remote-master",
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

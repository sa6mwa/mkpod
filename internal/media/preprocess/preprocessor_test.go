package preprocess

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sa6mwa/mkpod/internal/media"
)

func TestProcessFailsWhenToolMissing(t *testing.T) {
	p := New(&Config{
		Tool:   "mkpod-definitely-missing-ffmpeg",
		Preset: "sm7b",
		Prefix: "pre-",
	})

	err := p.Process(context.Background(), []string{"input.wav"})
	if !errors.Is(err, media.ErrToolNotFound) {
		t.Fatalf("Process() error = %v, want ErrToolNotFound", err)
	}
}

func TestFilterForPresetIncludesStereoPan(t *testing.T) {
	filter, err := filterForPreset("sm7b")
	if err != nil {
		t.Fatalf("filterForPreset() error = %v", err)
	}
	if !strings.HasPrefix(filter, "pan=stereo") {
		t.Fatalf("filter = %q, want stereo pan prefix", filter)
	}
}

func TestSM7BOriginalBacksUpBaseFilter(t *testing.T) {
	filter, err := filterForPreset("sm7b-original")
	if err != nil {
		t.Fatalf("filterForPreset() error = %v", err)
	}
	if strings.Contains(filter, "adeclick") || strings.Contains(filter, "afftdn") {
		t.Fatalf("sm7b-original filter = %q, want backup without cleanup filters", filter)
	}
}

func TestSM7BIncludesCleanupBeforeCompression(t *testing.T) {
	filter, err := filterForPreset("sm7b")
	if err != nil {
		t.Fatalf("filterForPreset() error = %v", err)
	}

	adeclickIndex := strings.Index(filter, "adeclick")
	afftdnIndex := strings.Index(filter, "afftdn")
	compandIndex := strings.Index(filter, "compand")
	if adeclickIndex < 0 || afftdnIndex < 0 {
		t.Fatalf("sm7b filter = %q, want cleanup filters", filter)
	}
	if compandIndex < 0 || adeclickIndex > compandIndex || afftdnIndex > compandIndex {
		t.Fatalf("sm7b filter = %q, want cleanup before compression", filter)
	}
}

func TestFilterForPresetRejectsUnknownPreset(t *testing.T) {
	_, err := filterForPreset("definitely-not-a-preset")
	if err == nil {
		t.Fatal("filterForPreset() error = nil, want error")
	}
}

func TestPlanDescribesPreprocessOperations(t *testing.T) {
	p := New(&Config{
		Tool:   "sh",
		Preset: "sm7b",
		Prefix: "clean-",
	})

	plan, err := p.Plan([]string{filepath.Join("masters", "raw.wav")})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Preset != "sm7b" {
		t.Fatalf("Preset = %q, want sm7b", plan.Preset)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("Operations length = %d, want 1", len(plan.Operations))
	}
	operation := plan.Operations[0]
	if operation.Input != filepath.Join("masters", "raw.wav") {
		t.Fatalf("Input = %q, want masters/raw.wav", operation.Input)
	}
	if operation.Output != filepath.Join("masters", "clean-raw.wav") {
		t.Fatalf("Output = %q, want masters/clean-raw.wav", operation.Output)
	}
	if operation.Tool != "sh" {
		t.Fatalf("Tool = %q, want sh", operation.Tool)
	}
	if len(operation.Args) == 0 {
		t.Fatal("Args is empty")
	}
	if !strings.Contains(strings.Join(operation.Args, " "), "-filter_complex") {
		t.Fatalf("Args = %v, want ffmpeg filter args", operation.Args)
	}
}

func TestOutputPathPrefixesBaseNameInSameDirectory(t *testing.T) {
	input := filepath.Join("masters", "raw.wav")
	got := outputPath(input, "preprocessed-")
	want := filepath.Join("masters", "preprocessed-raw.wav")
	if got != want {
		t.Fatalf("outputPath() = %q, want %q", got, want)
	}
}

func TestOutputPathPrefixesRelativeBaseName(t *testing.T) {
	got := outputPath("raw.wav", "preprocessed-")
	want := "preprocessed-raw.wav"
	if got != want {
		t.Fatalf("outputPath() = %q, want %q", got, want)
	}
}

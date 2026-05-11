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

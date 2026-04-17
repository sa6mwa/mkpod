package preprocess

import (
	"context"
	"errors"
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

func TestFilterForPresetRejectsUnknownPreset(t *testing.T) {
	_, err := filterForPreset("definitely-not-a-preset")
	if err == nil {
		t.Fatal("filterForPreset() error = nil, want error")
	}
}

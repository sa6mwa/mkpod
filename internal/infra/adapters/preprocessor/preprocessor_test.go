package preprocessor

import (
	"context"
	"errors"
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

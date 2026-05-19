package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTildeExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("UserHomeDir unavailable: %v", err)
	}

	got := resolvetilde("~/podcast")
	want := filepath.Join(home, "podcast")
	if got != want {
		t.Fatalf("resolvetilde() = %q, want %q", got, want)
	}
}

func TestPodcastExpandedToolPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("UserHomeDir unavailable: %v", err)
	}

	atom := &Podcast{}
	atom.Config.LocalStorageDir = "~/show"
	atom.Encoding.FFmpegPath = "~/bin/ffmpeg"
	atom.Encoding.Lamepath = "~/bin/lame"

	if got, want := atom.LocalStorageDirExpanded(), filepath.Join(home, "show"); got != want {
		t.Fatalf("LocalStorageDirExpanded() = %q, want %q", got, want)
	}
	if got, want := atom.FFmpegPathExpanded(), filepath.Join(home, "bin/ffmpeg"); got != want {
		t.Fatalf("FFmpegPathExpanded() = %q, want %q", got, want)
	}
	if got, want := atom.LamepathExpanded(), filepath.Join(home, "bin/lame"); got != want {
		t.Fatalf("LamepathExpanded() = %q, want %q", got, want)
	}
}

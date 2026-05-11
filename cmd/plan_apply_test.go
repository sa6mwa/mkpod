package cmd

import "testing"

func TestPlannedContentType(t *testing.T) {
	tests := map[string]string{
		"episode.mp3": "audio/mpeg",
		"episode.m4a": "audio/mp4",
		"episode.m4b": "audio/mp4",
		"episode.mp4": "video/mp4",
		"episode.wav": "",
	}
	for filename, want := range tests {
		if got := plannedContentType(filename); got != want {
			t.Fatalf("plannedContentType(%q) = %q, want %q", filename, got, want)
		}
	}
}

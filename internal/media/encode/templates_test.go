package encoder

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestAACArgsUseBuiltInFFmpegEncoder(t *testing.T) {
	podcast := &model.Podcast{
		Config: model.Config{LocalStorageDir: "/tmp/pod"},
		Encoding: struct {
			PreferredFormat string `yaml:"preferredFormat,omitempty"`
			Bitrate         int    `yaml:"bitrate"`
			Lamepath        string `yaml:"lamepath"`
			FFmpegPath      string `yaml:"ffmpegpath"`
			CRF             int    `yaml:"crf"`
			ABR             string `yaml:"abr"`
			Coverfront      string `yaml:"coverfront"`
			Genre           string `yaml:"genre"`
			Language        string `yaml:"language"`
		}{
			Lamepath:   "lame",
			FFmpegPath: "ffmpeg",
			CRF:        28,
			ABR:        "128k",
			Coverfront: "artwork/cover.jpg",
			Genre:      "Podcast",
			Language:   "eng",
		},
	}
	episode := &model.Episode{
		UID:    1,
		Title:  "Episode 1",
		Input:  "masters/episode.wav",
		Output: "episode.m4a",
	}

	videoArgs := buildMP4Args(podcast, &model.Episode{Input: episode.Input, Output: "episode.mp4"})
	audioArgs := buildFFmpegAudioArgs(podcast, episode, "/tmp/mkpod-ffmetadata.txt")

	assertAACArgs := func(t *testing.T, args []string) {
		t.Helper()
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "-c:a aac") {
			t.Fatalf("args %q do not use built-in AAC encoder", joined)
		}
		if strings.Contains(joined, "libfdk_aac") {
			t.Fatalf("args %q still reference libfdk_aac", joined)
		}
	}

	assertAACArgs(t, videoArgs)
	assertAACArgs(t, audioArgs)

	if got, want := audioArgs[len(audioArgs)-1], filepath.Join("/tmp/pod", "episode.m4a"); got != want {
		t.Fatalf("audio output path = %q, want %q", got, want)
	}
}

func TestLamePipeArgsIncludeExpectedMetadata(t *testing.T) {
	podcast := &model.Podcast{
		Author: "Podcast Author",
		Title:  "Podcast Title",
		Link:   "https://example.com/show",
		Config: model.Config{LocalStorageDir: "/tmp/pod"},
		Encoding: struct {
			PreferredFormat string `yaml:"preferredFormat,omitempty"`
			Bitrate         int    `yaml:"bitrate"`
			Lamepath        string `yaml:"lamepath"`
			FFmpegPath      string `yaml:"ffmpegpath"`
			CRF             int    `yaml:"crf"`
			ABR             string `yaml:"abr"`
			Coverfront      string `yaml:"coverfront"`
			Genre           string `yaml:"genre"`
			Language        string `yaml:"language"`
		}{
			Bitrate:    128,
			Coverfront: "artwork/cover.jpg",
			Genre:      "Podcast",
			Language:   "eng",
		},
	}
	episode := &model.Episode{
		UID:              7,
		Title:            "Episode 7",
		Subtitle:         "Subtitle",
		Input:            "masters/episode.wav",
		Output:           "episode.mp3",
		EncodingLanguage: "swe",
	}
	episode.PubDate.Time = time.Date(2024, time.January, 2, 15, 4, 0, 0, time.UTC)

	args := buildLamePipeArgs(podcast, episode)
	joined := strings.Join(args, " ")
	for _, expected := range []string{"--tt Episode 7", "--ta Podcast Author", "--tl Podcast Title", "--tv TLAN=swe", "--tv WOAR=https://example.com/show"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("args %q missing %q", joined, expected)
		}
	}
}

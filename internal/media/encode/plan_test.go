package encode

import (
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestPlanEpisodePredictsFFmpegAudioOutput(t *testing.T) {
	atom := &model.Podcast{}
	atom.Encoding.PreferredFormat = "m4a"
	episode := &model.Episode{Input: "masters/episode.flac"}

	plan, err := PlanEpisode(atom, episode, "audio/flac")
	if err != nil {
		t.Fatalf("PlanEpisode() error = %v", err)
	}
	if plan.Mode != "ffmpeg-audio" {
		t.Fatalf("Mode = %q, want ffmpeg-audio", plan.Mode)
	}
	if plan.Output != "episode.m4a" {
		t.Fatalf("Output = %q, want episode.m4a", plan.Output)
	}
}

func TestPlanEpisodePredictsVideoAudioOutput(t *testing.T) {
	atom := &model.Podcast{}
	episode := &model.Episode{Input: "masters/episode.mov", Format: "audio"}

	plan, err := PlanEpisode(atom, episode, "video/quicktime")
	if err != nil {
		t.Fatalf("PlanEpisode() error = %v", err)
	}
	if plan.Mode != "mp3-via-ffmpeg" {
		t.Fatalf("Mode = %q, want mp3-via-ffmpeg", plan.Mode)
	}
	if plan.Output != "episode.mp3" {
		t.Fatalf("Output = %q, want episode.mp3", plan.Output)
	}
}

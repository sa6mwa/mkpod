package encode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

type stubPrompter struct {
	answer bool
}

func (s stubPrompter) Ask(context.Context, string, ...any) bool {
	return s.answer
}

func TestShouldEncodeUsesExplicitOptions(t *testing.T) {
	tempDir := t.TempDir()
	existing := filepath.Join(tempDir, "episode.mp3")
	if err := os.WriteFile(existing, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := New(stubPrompter{answer: true})

	if !service.shouldEncode(context.Background(), EncodeOptions{ForceReencode: true}, existing) {
		t.Fatal("ForceReencode should re-encode existing files")
	}
	if service.shouldEncode(context.Background(), EncodeOptions{NeverReencode: true}, existing) {
		t.Fatal("NeverReencode should skip existing files")
	}
	if !service.shouldEncode(context.Background(), EncodeOptions{NeverReencode: true}, filepath.Join(tempDir, "missing.mp3")) {
		t.Fatal("NeverReencode should still encode missing output files")
	}
	if service.shouldEncode(context.Background(), EncodeOptions{All: true}, existing) {
		t.Fatal("All without force should skip existing files")
	}
	if !service.shouldEncode(context.Background(), EncodeOptions{}, existing) {
		t.Fatal("interactive path should respect prompter answer")
	}
	if !service.shouldEncode(context.Background(), EncodeOptions{}, filepath.Join(tempDir, "missing.mp3")) {
		t.Fatal("missing output file should be encoded")
	}
}

func TestSelectEpisodeIndexes(t *testing.T) {
	uid1 := int64(1)
	uid99 := int64(99)
	atom := &model.Podcast{
		Episodes: []model.Episode{
			{UID: 1},
			{UID: 2},
		},
	}

	tests := []struct {
		name    string
		options EncodeOptions
		want    []int
		wantErr error
	}{
		{name: "all episodes", options: EncodeOptions{All: true}, want: []int{0, 1}},
		{name: "specific episode", options: EncodeOptions{EpisodeUID: &uid1}, want: []int{0}},
		{name: "missing episode", options: EncodeOptions{EpisodeUID: &uid99}, want: nil},
		{name: "missing uid", options: EncodeOptions{}, wantErr: errors.New("episode UID is required when encoding without --all")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectEpisodeIndexes(atom, tt.options)
			if tt.wantErr != nil {
				if err == nil || err.Error() != tt.wantErr.Error() {
					t.Fatalf("selectEpisodeIndexes() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectEpisodeIndexes() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("selectEpisodeIndexes() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("selectEpisodeIndexes() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestApplyEpisodeDefaults(t *testing.T) {
	atom := &model.Podcast{
		Author: "Host",
		Config: model.Config{
			DefaultPodImage: "artwork/default.jpg",
		},
	}

	episode := &model.Episode{Title: "Episode"}
	if err := applyEpisodeDefaults(atom, episode); err != nil {
		t.Fatalf("applyEpisodeDefaults() error = %v", err)
	}
	if episode.Author != "Host" {
		t.Fatalf("Author = %q, want Host", episode.Author)
	}
	if episode.Image != "artwork/default.jpg" {
		t.Fatalf("Image = %q, want artwork/default.jpg", episode.Image)
	}
}

func TestApplyEpisodeDefaultsValidatesRequiredFields(t *testing.T) {
	atom := &model.Podcast{}
	if err := applyEpisodeDefaults(atom, &model.Episode{}); !errors.Is(err, ErrMissingTitle) {
		t.Fatalf("applyEpisodeDefaults() error = %v, want %v", err, ErrMissingTitle)
	}

	episode := &model.Episode{Title: "Episode"}
	if err := applyEpisodeDefaults(atom, episode); !errors.Is(err, ErrMissingImage) {
		t.Fatalf("applyEpisodeDefaults() error = %v, want %v", err, ErrMissingImage)
	}
}

func TestSelectEncodeMode(t *testing.T) {
	tests := []struct {
		name          string
		inputType     string
		episodeFormat string
		preferred     string
		wantMode      encodeMode
		wantFormat    string
		wantErr       string
	}{
		{name: "video defaults to mp4", inputType: "video/mp4", wantMode: modeMP4},
		{name: "video audio preferred m4a", inputType: "video/mp4", episodeFormat: "audio", preferred: "m4a", wantMode: modeFFmpegAudio, wantFormat: "m4a"},
		{name: "video audio defaults to mp3 via ffmpeg", inputType: "video/mp4", episodeFormat: "audio", wantMode: modeMP3ViaFFmpeg},
		{name: "video explicit mp3", inputType: "video/mp4", episodeFormat: "mp3", wantMode: modeMP3ViaFFmpeg},
		{name: "audio default mp3", inputType: "audio/wav", wantMode: modeMP3},
		{name: "audio preferred m4b", inputType: "audio/wav", preferred: "m4b", wantMode: modeFFmpegAudio, wantFormat: "m4b"},
		{name: "audio explicit m4a", inputType: "audio/wav", episodeFormat: "m4a", wantMode: modeFFmpegAudio, wantFormat: "m4a"},
		{name: "invalid format", inputType: "audio/wav", episodeFormat: "flac", wantErr: `invalid or unsupported format "flac"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotFormat, err := selectEncodeMode(tt.inputType, tt.episodeFormat, tt.preferred)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("selectEncodeMode() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectEncodeMode() error = %v", err)
			}
			if gotMode != tt.wantMode || gotFormat != tt.wantFormat {
				t.Fatalf("selectEncodeMode() = (%q, %q), want (%q, %q)", gotMode, gotFormat, tt.wantMode, tt.wantFormat)
			}
		})
	}
}

func TestEncodeReturnsEmptyResultForMissingEpisode(t *testing.T) {
	service := New(stubPrompter{answer: true})
	uid := int64(99)

	result, err := service.Encode(context.Background(), &model.Podcast{}, EncodeOptions{
		EpisodeUID: &uid,
	}, nil)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if result == nil {
		t.Fatal("Encode() returned nil result")
	}
	if len(result.SelectedIndexes) != 0 {
		t.Fatalf("SelectedIndexes = %v, want none", result.SelectedIndexes)
	}
	if len(result.EncodedOutputs) != 0 {
		t.Fatalf("EncodedOutputs = %v, want none", result.EncodedOutputs)
	}
}

func TestEncodeAllRepairsExistingOutputMetadataWithoutReencode(t *testing.T) {
	requireTools(t, "lame")

	workdir := t.TempDir()
	coverPath := filepath.Join(workdir, "artwork", "cover.jpg")
	inputPath := filepath.Join(workdir, "masters", "episode.wav")
	writeCoverJPEG(t, coverPath)
	writeSilentWAV(t, inputPath, 44100, 1)

	atom := testAtom(workdir)
	episode := model.Episode{
		UID:      1,
		Title:    "Episode One",
		Author:   "Host",
		PubDate:  model.ItunesTime{},
		Link:     "https://example.com/show/episodes/1",
		Subtitle: "Subtitle",
		Input:    filepath.ToSlash(filepath.Join("masters", "episode.wav")),
		Image:    filepath.ToSlash(filepath.Join("artwork", "cover.jpg")),
		Format:   "mp3",
		Explicit: model.ItunesExplicit{S: "no"},
	}
	atom.Episodes = []model.Episode{episode}

	if err := EncodeMP3(context.Background(), atom, &atom.Episodes[0]); err != nil {
		t.Fatalf("EncodeMP3() error = %v", err)
	}
	atom.Episodes[0].Type = ""
	atom.Episodes[0].Length = 0
	atom.Episodes[0].Duration = model.ItunesDuration{}
	if err := os.Remove(inputPath); err != nil {
		t.Fatalf("Remove(%q): %v", inputPath, err)
	}

	service := New(stubPrompter{answer: false})
	result, err := service.Encode(context.Background(), atom, EncodeOptions{All: true}, nil)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(result.EncodedOutputs) != 0 {
		t.Fatalf("EncodedOutputs = %v, want none", result.EncodedOutputs)
	}
	if atom.Episodes[0].Type != "audio/mpeg" {
		t.Fatalf("Type = %q, want audio/mpeg", atom.Episodes[0].Type)
	}
	if atom.Episodes[0].Length <= 0 {
		t.Fatalf("Length = %d, want > 0", atom.Episodes[0].Length)
	}
	if atom.Episodes[0].Duration.Duration <= 0 {
		t.Fatalf("Duration = %s, want > 0", atom.Episodes[0].Duration.Duration)
	}
	if atom.Episodes[0].PubDate.IsZero() {
		t.Fatal("PubDate should be set when repairing existing output metadata")
	}
}

package spec

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestValidateRejectsMissingRequiredFields(t *testing.T) {
	err := Validate(&model.Podcast{})
	if err == nil {
		t.Fatal("Validate() unexpectedly returned nil")
	}

	for _, field := range []string{
		"author",
		"config.baseURL",
		"config.image",
		"config.defaultPodImage",
		"atom",
		"title",
		"ttl",
		"language",
		"copyright",
		"webMaster",
		"description",
		"subtitle",
		"ownerName",
		"ownerEmail",
	} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("Validate() error %q does not include missing field %q", err, field)
		}
	}
}

func TestValidateRejectsNilAtom(t *testing.T) {
	err := Validate(nil)
	if !errors.Is(err, ErrNilPodcast) {
		t.Fatalf("Validate(nil) error = %v, want %v", err, ErrNilPodcast)
	}
}

func TestApplyDefaultsSetsMediaDefaultsAndPubDate(t *testing.T) {
	now := time.Date(2026, 4, 16, 19, 15, 0, 0, time.UTC)
	atom := &model.Podcast{}

	if err := ApplyDefaults(atom, now); err != nil {
		t.Fatalf("ApplyDefaults() error = %v", err)
	}
	if atom.Encoding.CRF != 28 {
		t.Fatalf("CRF = %d, want 28", atom.Encoding.CRF)
	}
	if atom.Encoding.ABR != "128k" {
		t.Fatalf("ABR = %q, want 128k", atom.Encoding.ABR)
	}
	if atom.Encoding.FFmpegPath != "ffmpeg" {
		t.Fatalf("FFmpegPath = %q, want ffmpeg", atom.Encoding.FFmpegPath)
	}
	if atom.Encoding.Lamepath != "lame" {
		t.Fatalf("Lamepath = %q, want lame", atom.Encoding.Lamepath)
	}
	if !atom.PubDate.Time.Equal(now) {
		t.Fatalf("PubDate = %v, want %v", atom.PubDate.Time, now)
	}
}

func TestApplyDefaultsPreservesExplicitValues(t *testing.T) {
	now := time.Date(2026, 4, 16, 19, 20, 0, 0, time.UTC)
	pubDate := time.Date(2022, 3, 25, 16, 0, 13, 0, time.UTC)
	atom := &model.Podcast{}
	atom.Encoding.CRF = 21
	atom.Encoding.ABR = "96k"
	atom.Encoding.FFmpegPath = "/usr/bin/ffmpeg"
	atom.Encoding.Lamepath = "/usr/bin/lame"
	atom.PubDate.Time = pubDate

	if err := ApplyDefaults(atom, now); err != nil {
		t.Fatalf("ApplyDefaults() error = %v", err)
	}
	if atom.Encoding.CRF != 21 {
		t.Fatalf("CRF = %d, want 21", atom.Encoding.CRF)
	}
	if atom.Encoding.ABR != "96k" {
		t.Fatalf("ABR = %q, want 96k", atom.Encoding.ABR)
	}
	if atom.Encoding.FFmpegPath != "/usr/bin/ffmpeg" {
		t.Fatalf("FFmpegPath = %q", atom.Encoding.FFmpegPath)
	}
	if atom.Encoding.Lamepath != "/usr/bin/lame" {
		t.Fatalf("Lamepath = %q", atom.Encoding.Lamepath)
	}
	if !atom.PubDate.Time.Equal(pubDate) {
		t.Fatalf("PubDate = %v, want %v", atom.PubDate.Time, pubDate)
	}
}

func TestApplyEpisodeDefaultsForEncoding(t *testing.T) {
	atom := &model.Podcast{
		Author: "Host",
		Config: model.Config{
			DefaultPodImage: "artwork/default.jpg",
		},
	}
	episode := &model.Episode{Title: "Episode"}

	if err := ApplyEpisodeDefaultsForEncoding(atom, episode); err != nil {
		t.Fatalf("ApplyEpisodeDefaultsForEncoding() error = %v", err)
	}
	if episode.Author != "Host" {
		t.Fatalf("Author = %q, want Host", episode.Author)
	}
	if episode.Image != "artwork/default.jpg" {
		t.Fatalf("Image = %q, want artwork/default.jpg", episode.Image)
	}
}

func TestEffectiveEpisodeDefaults(t *testing.T) {
	atom := &model.Podcast{
		Author:   "Host",
		Explicit: model.ItunesExplicit{S: "yes"},
		Config: model.Config{
			DefaultPodImage: "artwork/default.jpg",
		},
	}
	episode := &model.Episode{}

	if got := EffectiveEpisodeAuthor(atom, episode); got != "Host" {
		t.Fatalf("EffectiveEpisodeAuthor() = %q, want Host", got)
	}
	if got := EffectiveEpisodeExplicit(atom, episode); got != "yes" {
		t.Fatalf("EffectiveEpisodeExplicit() = %q, want yes", got)
	}
	if got := EffectiveEpisodeImage(atom, episode); got != "artwork/default.jpg" {
		t.Fatalf("EffectiveEpisodeImage() = %q, want artwork/default.jpg", got)
	}

	episode.Author = "Guest"
	episode.Explicit = model.ItunesExplicit{S: "no"}
	episode.Image = "artwork/custom.jpg"
	if got := EffectiveEpisodeAuthor(atom, episode); got != "Guest" {
		t.Fatalf("EffectiveEpisodeAuthor() = %q, want Guest", got)
	}
	if got := EffectiveEpisodeExplicit(atom, episode); got != "no" {
		t.Fatalf("EffectiveEpisodeExplicit() = %q, want no", got)
	}
	if got := EffectiveEpisodeImage(atom, episode); got != "artwork/custom.jpg" {
		t.Fatalf("EffectiveEpisodeImage() = %q, want artwork/custom.jpg", got)
	}
}

func TestApplyEpisodeDefaultsForEncodingValidatesRequiredFields(t *testing.T) {
	if err := ApplyEpisodeDefaultsForEncoding(&model.Podcast{}, &model.Episode{}); !errors.Is(err, ErrMissingEpisodeTitle) {
		t.Fatalf("ApplyEpisodeDefaultsForEncoding() error = %v, want %v", err, ErrMissingEpisodeTitle)
	}

	if err := ApplyEpisodeDefaultsForEncoding(&model.Podcast{}, &model.Episode{Title: "Episode"}); !errors.Is(err, ErrMissingEpisodeImage) {
		t.Fatalf("ApplyEpisodeDefaultsForEncoding() error = %v, want %v", err, ErrMissingEpisodeImage)
	}
}

func TestMissingFieldsForRSS(t *testing.T) {
	now := time.Now().UTC()
	atom := &model.Podcast{Author: "Host"}

	valid := &model.Episode{
		Title:    "Episode",
		PubDate:  model.ItunesTime{Time: now},
		Output:   "episode.mp3",
		Duration: model.ItunesDuration{Duration: time.Minute},
		Length:   123,
		Type:     "audio/mpeg",
		Image:    "artwork/episode.jpg",
	}
	if got := MissingFieldsForRSS(atom, valid); len(got) != 0 {
		t.Fatalf("MissingFieldsForRSS(valid) = %v, want none", got)
	}

	missing := &model.Episode{}
	got := MissingFieldsForRSS(&model.Podcast{}, missing)
	for _, field := range []string{"title", "output", "duration", "length", "type", "image", "pubDate", "author"} {
		if !containsString(got, field) {
			t.Fatalf("MissingFieldsForRSS() = %v, expected %q", got, field)
		}
	}
}

func TestRenderableEpisodesSkipsInvalidEpisodes(t *testing.T) {
	now := time.Now().UTC()
	atom := &model.Podcast{Author: "Host"}
	episodes := []model.Episode{
		{
			UID:      1,
			Title:    "Valid",
			PubDate:  model.ItunesTime{Time: now},
			Output:   "valid.mp3",
			Duration: model.ItunesDuration{Duration: time.Minute},
			Length:   123,
			Type:     "audio/mpeg",
			Image:    "cover.jpg",
		},
		{
			UID:      2,
			Title:    "Invalid",
			PubDate:  model.ItunesTime{Time: now},
			Duration: model.ItunesDuration{Duration: time.Minute},
			Length:   123,
			Type:     "audio/mpeg",
			Image:    "cover.jpg",
		},
	}

	renderable, issues := RenderableEpisodes(atom, episodes)
	if len(renderable) != 1 || renderable[0].UID != 1 {
		t.Fatalf("RenderableEpisodes() renderable = %v, want only UID 1", renderable)
	}
	if len(issues) != 1 {
		t.Fatalf("RenderableEpisodes() issues = %v, want 1 issue", issues)
	}
	if issues[0].UID != 2 || !containsString(issues[0].MissingFields, "output") {
		t.Fatalf("RenderableEpisodes() issue = %+v, want UID 2 missing output", issues[0])
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestStorePubDateHandling(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		expectCurrent bool
		shouldError   bool
	}{
		{
			name: "missing pubDate should be set to current time",
			yamlContent: `
atom: "podcast.rss"
title: "Test Podcast"
link: "https://example.com"
ttl: 60
language: "en"
copyright: "Test Copyright"
webMaster: "test@example.com"
description: "Test Description"
subtitle: "Test Subtitle"
ownerName: "Test Owner"
ownerEmail: "owner@example.com"
author: "Test Author"
config:
  baseURL: "https://example.com"
  image: "https://example.com/image.jpg"
  defaultPodImage: "default.jpg"
  localStorageDir: "./test"
`,
			expectCurrent: true,
		},
		{
			name: "empty pubDate should be set to current time",
			yamlContent: `
atom: "podcast.rss"
title: "Test Podcast"
link: "https://example.com"
pubDate: ""
ttl: 60
language: "en"
copyright: "Test Copyright"
webMaster: "test@example.com"
description: "Test Description"
subtitle: "Test Subtitle"
ownerName: "Test Owner"
ownerEmail: "owner@example.com"
author: "Test Author"
config:
  baseURL: "https://example.com"
  image: "https://example.com/image.jpg"
  defaultPodImage: "default.jpg"
  localStorageDir: "./test"
`,
			expectCurrent: true,
		},
		{
			name: "zero date pubDate should be set to current time",
			yamlContent: `
atom: "podcast.rss"
title: "Test Podcast"
link: "https://example.com"
pubDate: "Mon, 01 Jan 0001 00:00:00 +0000"
ttl: 60
language: "en"
copyright: "Test Copyright"
webMaster: "test@example.com"
description: "Test Description"
subtitle: "Test Subtitle"
ownerName: "Test Owner"
ownerEmail: "owner@example.com"
author: "Test Author"
config:
  baseURL: "https://example.com"
  image: "https://example.com/image.jpg"
  defaultPodImage: "default.jpg"
  localStorageDir: "./test"
`,
			expectCurrent: true,
		},
		{
			name: "valid pubDate should be preserved",
			yamlContent: `
atom: "podcast.rss"
title: "Test Podcast"
link: "https://example.com"
pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"
ttl: 60
language: "en"
copyright: "Test Copyright"
webMaster: "test@example.com"
description: "Test Description"
subtitle: "Test Subtitle"
ownerName: "Test Owner"
ownerEmail: "owner@example.com"
author: "Test Author"
config:
  baseURL: "https://example.com"
  image: "https://example.com/image.jpg"
  defaultPodImage: "default.jpg"
  localStorageDir: "./test"
`,
			expectCurrent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-podspec-*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(strings.TrimSpace(tt.yamlContent)); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			store := New(tmpFile.Name())
			atom, err := store.Load(context.Background())
			if tt.shouldError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if atom == nil {
				t.Fatal("atom is nil")
			}

			if tt.expectCurrent {
				if atom.PubDate.IsZero() {
					t.Error("expected pubDate to be set to current time, but it's still zero")
				} else {
					now := time.Now().UTC()
					diff := now.Sub(atom.PubDate.Time)
					if diff < 0 {
						diff = -diff
					}
					if diff > 5*time.Second {
						t.Errorf("expected current time, but got %v (diff: %v)", atom.PubDate.Time, diff)
					}
				}
			} else {
				expectedTime, _ := time.Parse(time.RFC1123Z, "Fri, 25 Mar 2022 16:00:13 +0000")
				if !atom.PubDate.Time.Equal(expectedTime) {
					t.Errorf("expected %v, got %v", expectedTime, atom.PubDate.Time)
				}
			}
		})
	}
}

func TestStoreSaveUpdatesLastBuildDate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-podspec-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	yamlContent := `
atom: "podcast.rss"
title: "Test Podcast"
link: "https://example.com"
ttl: 60
language: "en"
copyright: "Test Copyright"
webMaster: "test@example.com"
description: "Test Description"
subtitle: "Test Subtitle"
ownerName: "Test Owner"
ownerEmail: "owner@example.com"
author: "Test Author"
config:
  baseURL: "https://example.com"
  image: "https://example.com/image.jpg"
  defaultPodImage: "default.jpg"
  localStorageDir: "./test"
`

	if _, err := tmpFile.WriteString(strings.TrimSpace(yamlContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	store := New(tmpFile.Name())
	ctx := context.Background()
	atom, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	beforeSave := time.Now().UTC()
	err = store.Save(ctx, atom)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}
	afterSave := time.Now().UTC()

	atom2, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("unexpected error on second load: %v", err)
	}

	if atom2.LastBuildDate.Time.Before(beforeSave.Add(-1*time.Second)) || atom2.LastBuildDate.Time.After(afterSave.Add(1*time.Second)) {
		t.Errorf("lastBuildDate %v should be between %v and %v", atom2.LastBuildDate.Time, beforeSave, afterSave)
	}
}

func TestStoreDefaultsMediaToolsToPATH(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-podspec-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	yamlContent := `
atom: "podcast.rss"
title: "Test Podcast"
link: "https://example.com"
ttl: 60
language: "en"
copyright: "Test Copyright"
webMaster: "test@example.com"
description: "Test Description"
subtitle: "Test Subtitle"
ownerName: "Test Owner"
ownerEmail: "owner@example.com"
author: "Test Author"
config:
  baseURL: "https://example.com"
  image: "https://example.com/image.jpg"
  defaultPodImage: "default.jpg"
  localStorageDir: "./test"
encoding:
  bitrate: 128
  crf: 28
  abr: "128k"
  coverfront: "default.jpg"
  genre: "Podcast"
  language: "eng"
`

	if _, err := tmpFile.WriteString(strings.TrimSpace(yamlContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	store := New(tmpFile.Name())
	atom, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atom.Encoding.FFmpegPath != "ffmpeg" {
		t.Fatalf("FFmpegPath = %q, want %q", atom.Encoding.FFmpegPath, "ffmpeg")
	}
	if atom.Encoding.Lamepath != "lame" {
		t.Fatalf("Lamepath = %q, want %q", atom.Encoding.Lamepath, "lame")
	}
}

package rss

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestWriteRSSRendersValidEpisodesOnly(t *testing.T) {
	renderer := New()
	now := time.Now().UTC()

	atom := &model.Atom{
		Config: model.Config{
			BaseURL: "https://example.com/podcast",
			Image:   "https://example.com/podcast/artwork/show.jpg",
		},
		Atom:          "podcast.rss",
		Title:         "Example Show",
		Link:          "https://example.com/show",
		PubDate:       model.ItunesTime{Time: now},
		LastBuildDate: model.ItunesTime{Time: now},
		TTL:           60,
		Language:      "en",
		Copyright:     "Copyright Example",
		WebMaster:     "webmaster@example.com",
		Description:   "Show description",
		Subtitle:      "Show subtitle",
		OwnerName:     "Owner",
		OwnerEmail:    "owner@example.com",
		Author:        "Host",
		Explicit:      model.ItunesExplicit{S: "no"},
		Keywords:      "podcast,test",
		Categories: []model.Category{
			{Name: "Technology", Subcategories: []string{}},
		},
		Episodes: []model.Episode{
			{
				UID:         1,
				Title:       "Valid Episode",
				PubDate:     model.ItunesTime{Time: now},
				Link:        "https://example.com/show/episodes/1",
				Duration:    model.ItunesDuration{Duration: 5 * time.Minute},
				Subtitle:    "Episode subtitle",
				Description: "Episode description",
				Type:        "audio/mpeg",
				Length:      12345,
				Image:       "artwork/episode1.jpg",
				Output:      "episode1.mp3",
			},
			{
				UID:         2,
				Title:       "Invalid Episode",
				PubDate:     model.ItunesTime{Time: now},
				Link:        "https://example.com/show/episodes/2",
				Description: "Missing output/type/length/image/duration",
			},
		},
	}

	var buf bytes.Buffer
	if err := writeRSS(context.Background(), &buf, renderer, atom); err != nil {
		t.Fatalf("writeRSS() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "<title>Example Show</title>") {
		t.Fatalf("expected feed title in RSS output: %s", output)
	}
	if !strings.Contains(output, "<title>Valid Episode</title>") {
		t.Fatalf("expected valid episode in RSS output: %s", output)
	}
	if strings.Contains(output, "Invalid Episode") {
		t.Fatalf("invalid episode should not be present in RSS output: %s", output)
	}
	if !strings.Contains(output, "<itunes:author>Host</itunes:author>") {
		t.Fatalf("expected author to be rendered in RSS output: %s", output)
	}
}

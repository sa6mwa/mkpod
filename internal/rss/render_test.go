package rss

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestWriteRSSRendersValidEpisodesOnly(t *testing.T) {
	renderer := New()
	now := time.Now().UTC()

	atom := &model.Podcast{
		Config: model.Config{
			BaseURL: "https://example.com/podcast",
			Image:   "https://example.com/podcast/artwork/show.jpg",
		},
		FeedFile:      "podcast.rss",
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
	assertWellFormedRSSXML(t, []byte(output))
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

func TestWriteRSSRendersInheritedEpisodeImage(t *testing.T) {
	renderer := New()
	now := time.Now().UTC()

	atom := &model.Podcast{
		Config: model.Config{
			BaseURL:         "https://example.com/podcast",
			Image:           "https://example.com/podcast/artwork/show.jpg",
			DefaultPodImage: "artwork/default.jpg",
		},
		FeedFile:      "podcast.rss",
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
		Episodes: []model.Episode{{
			UID:         1,
			Title:       "Inherited Image Episode",
			PubDate:     model.ItunesTime{Time: now},
			Link:        "https://example.com/show/episodes/1",
			Duration:    model.ItunesDuration{Duration: 5 * time.Minute},
			Description: "Episode description",
			Type:        "audio/mpeg",
			Length:      12345,
			Output:      "episode1.mp3",
		}},
	}

	var buf bytes.Buffer
	if err := writeRSS(context.Background(), &buf, renderer, atom); err != nil {
		t.Fatalf("writeRSS() error = %v", err)
	}

	output := buf.String()
	assertWellFormedRSSXML(t, []byte(output))
	if !strings.Contains(output, `<itunes:image href="https://example.com/podcast/artwork/default.jpg"/>`) {
		t.Fatalf("expected inherited default episode image in RSS output: %s", output)
	}
}

func TestWriteRSSMatchesGoldenFile(t *testing.T) {
	renderer := New()
	pubDate := time.Date(2022, 3, 25, 16, 0, 13, 0, time.UTC)

	atom := &model.Podcast{
		Config: model.Config{
			BaseURL: "https://example.com/podcast",
			Image:   "https://example.com/podcast/artwork/show.jpg",
		},
		FeedFile:      "podcast.rss",
		Title:         "Example Show",
		Link:          "https://example.com/show",
		PubDate:       model.ItunesTime{Time: pubDate},
		LastBuildDate: model.ItunesTime{Time: pubDate},
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
			{Name: "Technology"},
		},
		Episodes: []model.Episode{
			{
				UID:         1,
				Title:       "Episode One",
				PubDate:     model.ItunesTime{Time: pubDate},
				Link:        "https://example.com/show/episodes/1",
				Duration:    model.ItunesDuration{Duration: 5*time.Minute + 4*time.Second},
				Author:      "Guest Host",
				Subtitle:    "Episode subtitle",
				Description: "Episode description",
				Type:        "audio/mpeg",
				Length:      12345,
				Image:       "artwork/episode1.jpg",
				Output:      "episode1.mp3",
			},
		},
	}

	var buf bytes.Buffer
	if err := writeRSS(context.Background(), &buf, renderer, atom); err != nil {
		t.Fatalf("writeRSS() error = %v", err)
	}

	goldenPath := filepath.Join("testdata", "representative.rss")
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", goldenPath, err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace(string(golden))
	if got != want {
		t.Fatalf("RSS output mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func assertWellFormedRSSXML(t *testing.T, content []byte) {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(content))
	for {
		if _, err := decoder.Token(); err != nil {
			if err == io.EOF {
				return
			}
			t.Fatalf("invalid RSS XML: %v\n%s", err, string(content))
		}
	}
}

type parsedRSSFeed struct {
	Channel struct {
		Items []parsedRSSItem `xml:"item"`
	} `xml:"channel"`
}

type parsedRSSItem struct {
	Title       string `xml:"title"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Subtitle    string `xml:"subtitle"`
	Enclosure   struct {
		Type string `xml:"type,attr"`
		URL  string `xml:"url,attr"`
	} `xml:"enclosure"`
	Image struct {
		Href string `xml:"href,attr"`
	} `xml:"image"`
}

func parseRSSFeed(t *testing.T, content []byte) parsedRSSFeed {
	t.Helper()
	var feed parsedRSSFeed
	if err := xml.Unmarshal(content, &feed); err != nil {
		t.Fatalf("unmarshal RSS XML: %v\n%s", err, string(content))
	}
	return feed
}

func TestWriteRSSExcludesFutureEpisodesAndKeepsInputOrder(t *testing.T) {
	renderer := New()
	now := time.Now().UTC().Truncate(time.Second)

	atom := &model.Podcast{
		Config: model.Config{
			BaseURL: "https://example.com/podcast",
			Image:   "https://example.com/podcast/artwork/show.jpg",
		},
		FeedFile:      "podcast.rss",
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
		Episodes: []model.Episode{
			{
				UID:         2,
				Title:       "Older Published Episode",
				PubDate:     model.ItunesTime{Time: now.Add(-2 * time.Hour)},
				Link:        "https://example.com/show/episodes/2",
				Duration:    model.ItunesDuration{Duration: 5 * time.Minute},
				Description: "Older published description",
				Type:        "audio/mpeg",
				Length:      12345,
				Image:       "artwork/older.jpg",
				Output:      "older.mp3",
			},
			{
				UID:         3,
				Title:       "Future Episode",
				PubDate:     model.ItunesTime{Time: now.Add(2 * time.Hour)},
				Link:        "https://example.com/show/episodes/3",
				Duration:    model.ItunesDuration{Duration: 5 * time.Minute},
				Description: "Future description",
				Type:        "audio/mpeg",
				Length:      67890,
				Image:       "artwork/future.jpg",
				Output:      "future.mp3",
			},
			{
				UID:         1,
				Title:       "Newest Published Episode",
				PubDate:     model.ItunesTime{Time: now.Add(-1 * time.Hour)},
				Link:        "https://example.com/show/episodes/1",
				Duration:    model.ItunesDuration{Duration: 5 * time.Minute},
				Description: "Newest published description",
				Type:        "audio/mpeg",
				Length:      99999,
				Image:       "artwork/newest.jpg",
				Output:      "newest.mp3",
			},
		},
	}

	var buf bytes.Buffer
	if err := writeRSS(context.Background(), &buf, renderer, atom); err != nil {
		t.Fatalf("writeRSS() error = %v", err)
	}

	content := buf.Bytes()
	assertWellFormedRSSXML(t, content)
	feed := parseRSSFeed(t, content)
	if len(feed.Channel.Items) != 2 {
		t.Fatalf("expected 2 published items, got %d", len(feed.Channel.Items))
	}
	if feed.Channel.Items[0].Title != "Older Published Episode" {
		t.Fatalf("expected first item to preserve input order, got %q", feed.Channel.Items[0].Title)
	}
	if feed.Channel.Items[1].Title != "Newest Published Episode" {
		t.Fatalf("expected second item to preserve input order, got %q", feed.Channel.Items[1].Title)
	}
	for _, item := range feed.Channel.Items {
		if item.Title == "Future Episode" {
			t.Fatalf("future episode should not be published")
		}
		if item.Description == "" {
			t.Fatalf("expected description for item %q", item.Title)
		}
	}
}

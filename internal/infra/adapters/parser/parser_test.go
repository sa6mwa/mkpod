package parser

import (
	"context"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestFilterValidEpisodes(t *testing.T) {
	parser := &forParsing{}
	ctx := context.Background()

	atom := &model.Atom{
		Author: "Test Author",
		Explicit: model.ItunesExplicit{S: "no"},
	}

	// Create test episodes with various missing fields
	episodes := []model.Episode{
		{
			UID:      1,
			Title:    "Valid Episode",
			PubDate:  model.ItunesTime{Time: time.Now().UTC()},
			Output:   "episode1.mp3",
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			Type:     "audio/mpeg",
			Image:    "episode1.jpg",
			Author:   "Episode Author",
		},
		{
			UID:    2,
			Title:  "Missing Output",
			// Output: missing
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			Type:     "audio/mpeg",
			Image:    "episode2.jpg",
			Author:   "Episode Author",
		},
		{
			UID:    3,
			Title:  "Missing Duration",
			Output: "episode3.mp3",
			// Duration: zero value
			Length: 1000000,
			Type:   "audio/mpeg",
			Image:  "episode3.jpg",
			Author: "Episode Author",
		},
		{
			UID:      4,
			Title:    "Missing Length",
			Output:   "episode4.mp3",
			Duration: model.ItunesDuration{Duration: time.Hour},
			// Length: zero value
			Type:   "audio/mpeg",
			Image:  "episode4.jpg",
			Author: "Episode Author",
		},
		{
			UID:      5,
			Title:    "Missing Type",
			Output:   "episode5.mp3",
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			// Type: missing
			Image:  "episode5.jpg",
			Author: "Episode Author",
		},
		{
			UID:      6,
			Title:    "Missing Image",
			Output:   "episode6.mp3",
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			Type:     "audio/mpeg",
			// Image: missing
			Author: "Episode Author",
		},
		{
			UID:      7,
			Title:    "Missing Author (uses default)",
			PubDate:  model.ItunesTime{Time: time.Now().UTC()},
			Output:   "episode7.mp3",
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			Type:     "audio/mpeg",
			Image:    "episode7.jpg",
			// Author: missing, should use atom.Author
		},
		{
			UID:      8,
			Title:    "Missing Author (no default)",
			PubDate:  model.ItunesTime{Time: time.Now().UTC()},
			Output:   "episode8.mp3",
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			Type:     "audio/mpeg",
			Image:    "episode8.jpg",
			// Author: missing, and atom.Author will be empty
		},
		{
			UID:      9,
			Title:    "", // Empty title - should be excluded
			PubDate:  model.ItunesTime{Time: time.Now().UTC()},
			Output:   "episode9.mp3",
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			Type:     "audio/mpeg",
			Image:    "episode9.jpg",
			Author:   "Episode Author",
		},
	}

	t.Run("Filter episodes with missing fields", func(t *testing.T) {
		validEpisodes := parser.filterValidEpisodes(ctx, atom, episodes)

		// Episodes 1, 7, and 8 should be valid (8 gets default author from atom)
		expectedValid := 3
		if len(validEpisodes) != expectedValid {
			t.Errorf("Expected %d valid episodes, got %d", expectedValid, len(validEpisodes))
		}

		// Check that the correct episodes were kept
		if len(validEpisodes) >= 1 && validEpisodes[0].UID != 1 {
			t.Errorf("Expected first valid episode to have UID 1, got %d", validEpisodes[0].UID)
		}
		if len(validEpisodes) >= 2 && validEpisodes[1].UID != 7 {
			t.Errorf("Expected second valid episode to have UID 7, got %d", validEpisodes[1].UID)
		}
		if len(validEpisodes) >= 3 && validEpisodes[2].UID != 8 {
			t.Errorf("Expected third valid episode to have UID 8, got %d", validEpisodes[2].UID)
		}
	})

	t.Run("Filter episodes when atom author is missing", func(t *testing.T) {
		atomNoAuthor := &model.Atom{
			Author: "", // Empty author
			Explicit: model.ItunesExplicit{S: "no"},
		}

		validEpisodes := parser.filterValidEpisodes(ctx, atomNoAuthor, episodes)

		// Episodes 7 and 8 now have pubDate but no author and no atom default
		// Only episode 1 should be valid (has its own author and pubDate)
		expectedValid := 1
		if len(validEpisodes) != expectedValid {
			t.Errorf("Expected %d valid episodes, got %d", expectedValid, len(validEpisodes))
		}

		if len(validEpisodes) >= 1 && validEpisodes[0].UID != 1 {
			t.Errorf("Expected valid episode to have UID 1, got %d", validEpisodes[0].UID)
		}
	})
}

func TestEpisodeTemplateHelpers(t *testing.T) {
	parser := &forParsing{}
	ctx := context.Background()

	atom := &model.Atom{
		Author: "Default Author",
		Explicit: model.ItunesExplicit{S: "yes"},
	}

	funcMap := parser.mkFuncMapWithContext(ctx, atom)

	t.Run("episodeAuthor helper", func(t *testing.T) {
		episodeAuthorFunc, exists := funcMap["episodeAuthor"]
		if !exists {
			t.Fatal("episodeAuthor function not found in funcMap")
		}

		// Test episode with author
		episode1 := model.Episode{Author: "Episode Author"}
		author1 := episodeAuthorFunc.(func(model.Episode) string)(episode1)
		if author1 != "Episode Author" {
			t.Errorf("Expected 'Episode Author', got '%s'", author1)
		}

		// Test episode without author (should use default)
		episode2 := model.Episode{Author: ""}
		author2 := episodeAuthorFunc.(func(model.Episode) string)(episode2)
		if author2 != "Default Author" {
			t.Errorf("Expected 'Default Author', got '%s'", author2)
		}
	})

	t.Run("episodeExplicit helper", func(t *testing.T) {
		episodeExplicitFunc, exists := funcMap["episodeExplicit"]
		if !exists {
			t.Fatal("episodeExplicit function not found in funcMap")
		}

		// Test episode with explicit set
		episode1 := model.Episode{Explicit: model.ItunesExplicit{S: "yes"}}
		explicit1 := episodeExplicitFunc.(func(model.Episode) string)(episode1)
		if explicit1 != "yes" {
			t.Errorf("Expected 'yes', got '%s'", explicit1)
		}

		// Test episode without explicit (should use atom default)
		episode2 := model.Episode{Explicit: model.ItunesExplicit{S: ""}}
		explicit2 := episodeExplicitFunc.(func(model.Episode) string)(episode2)
		if explicit2 != "yes" {
			t.Errorf("Expected 'yes' (from atom), got '%s'", explicit2)
		}
	})

	t.Run("episodeExplicit helper with no atom default", func(t *testing.T) {
		atomNoExplicit := &model.Atom{
			Author: "Default Author",
			Explicit: model.ItunesExplicit{S: ""},
		}

		funcMapNoExplicit := parser.mkFuncMapWithContext(ctx, atomNoExplicit)
		episodeExplicitFunc := funcMapNoExplicit["episodeExplicit"].(func(model.Episode) string)

		// Test episode without explicit and no atom default (should default to "no")
		episode := model.Episode{Explicit: model.ItunesExplicit{S: ""}}
		explicit := episodeExplicitFunc(episode)
		if explicit != "no" {
			t.Errorf("Expected 'no' (default), got '%s'", explicit)
		}
	})
}

func TestValidEpisodesTemplateFunction(t *testing.T) {
	parser := &forParsing{}
	ctx := context.Background()

	atom := &model.Atom{
		Author: "Test Author",
		Explicit: model.ItunesExplicit{S: "no"},
	}

	episodes := []model.Episode{
		{
			UID:      1,
			Title:    "Valid Episode",
			PubDate:  model.ItunesTime{Time: time.Now().UTC()},
			Output:   "episode1.mp3",
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			Type:     "audio/mpeg",
			Image:    "episode1.jpg",
			Author:   "Episode Author",
		},
		{
			UID:    2,
			Title:  "Invalid Episode - Missing Output",
			// Missing output
			Duration: model.ItunesDuration{Duration: time.Hour},
			Length:   1000000,
			Type:     "audio/mpeg",
			Image:    "episode2.jpg",
			Author:   "Episode Author",
		},
	}

	funcMap := parser.mkFuncMapWithContext(ctx, atom)
	validEpisodesFunc, exists := funcMap["validEpisodes"]
	if !exists {
		t.Fatal("validEpisodes function not found in funcMap")
	}

	validEpisodes := validEpisodesFunc.(func([]model.Episode) []model.Episode)(episodes)

	if len(validEpisodes) != 1 {
		t.Errorf("Expected 1 valid episode, got %d", len(validEpisodes))
	}

	if len(validEpisodes) > 0 && validEpisodes[0].UID != 1 {
		t.Errorf("Expected valid episode to have UID 1, got %d", validEpisodes[0].UID)
	}
}

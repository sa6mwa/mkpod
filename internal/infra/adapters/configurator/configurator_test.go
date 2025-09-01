package configurator

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfiguratorPubDateHandling(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		expectCurrent  bool
		shouldError    bool
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
			// Create a temporary file
			tmpFile, err := os.CreateTemp("", "test-podspec-*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write test YAML content
			_, err = tmpFile.WriteString(strings.TrimSpace(tt.yamlContent))
			if err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Create configurator with temp file
			configurator := New(tmpFile.Name())
			
			// Load the atom
			ctx := context.Background()
			atom, err := configurator.Load(ctx)
			
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
				// Should not be zero
				if atom.PubDate.IsZero() {
					t.Error("expected pubDate to be set to current time, but it's still zero")
				} else {
					// Check that the time is recent (within the last 5 seconds)
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
				// Should be the specific time from the YAML
				expectedTime, _ := time.Parse(time.RFC1123Z, "Fri, 25 Mar 2022 16:00:13 +0000")
				if !atom.PubDate.Time.Equal(expectedTime) {
					t.Errorf("expected %v, got %v", expectedTime, atom.PubDate.Time)
				}
			}
		})
	}
}

func TestConfiguratorSaveUpdatesLastBuildDate(t *testing.T) {
	// Create a temporary file
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

	// Write initial content
	_, err = tmpFile.WriteString(strings.TrimSpace(yamlContent))
	if err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create configurator
	configurator := New(tmpFile.Name())
	ctx := context.Background()

	// Load the atom
	atom, err := configurator.Load(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Save the atom
	beforeSave := time.Now().UTC()
	err = configurator.Save(ctx, atom)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}
	afterSave := time.Now().UTC()

	// Load again to check lastBuildDate was updated
	atom2, err := configurator.Load(ctx)
	if err != nil {
		t.Fatalf("unexpected error on second load: %v", err)
	}

	// LastBuildDate should be between beforeSave and afterSave (with 1 second tolerance for precision)
	if atom2.LastBuildDate.Time.Before(beforeSave.Add(-1*time.Second)) || atom2.LastBuildDate.Time.After(afterSave.Add(1*time.Second)) {
		t.Errorf("lastBuildDate %v should be between %v and %v", atom2.LastBuildDate.Time, beforeSave, afterSave)
	}
}

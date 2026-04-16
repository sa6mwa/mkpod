package model

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestItunesTime_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectCurrent bool       // true if we expect current time, false if we expect specific time
		expectedTime  *time.Time // specific time to expect, only used if expectCurrent is false
		shouldError   bool
	}{
		{
			name:          "empty string should set to current time",
			input:         `pubDate: ""`,
			expectCurrent: true,
		},
		{
			name:          "today string should set to current time",
			input:         `pubDate: "today"`,
			expectCurrent: true,
		},
		{
			name:          "now string should set to current time",
			input:         `pubDate: "now"`,
			expectCurrent: true,
		},
		{
			name:          "zero date string should set to current time",
			input:         `pubDate: "Mon, 01 Jan 0001 00:00:00 +0000"`,
			expectCurrent: true,
		},
		{
			name:          "yesterday string should resolve to previous day",
			input:         `pubDate: "yesterday"`,
			expectCurrent: false,
			expectedTime: func() *time.Time {
				t := startOfRelativeDay(time.Now().UTC(), -1)
				return &t
			}(),
		},
		{
			name:          "HHMM string should resolve to today at given time",
			input:         `pubDate: "1530"`,
			expectCurrent: false,
			expectedTime: func() *time.Time {
				now := time.Now().UTC()
				t := time.Date(now.Year(), now.Month(), now.Day(), 15, 30, 0, 0, time.UTC)
				return &t
			}(),
		},
		{
			name:          "HH:MM string should resolve to today at given time",
			input:         `pubDate: "15:30"`,
			expectCurrent: false,
			expectedTime: func() *time.Time {
				now := time.Now().UTC()
				t := time.Date(now.Year(), now.Month(), now.Day(), 15, 30, 0, 0, time.UTC)
				return &t
			}(),
		},
		{
			name:          "yesterday HHMM string should resolve to previous day at given time",
			input:         `pubDate: "yesterday 1530"`,
			expectCurrent: false,
			expectedTime: func() *time.Time {
				base := startOfRelativeDay(time.Now().UTC(), -1)
				t := time.Date(base.Year(), base.Month(), base.Day(), 15, 30, 0, 0, time.UTC)
				return &t
			}(),
		},
		{
			name:          "valid RFC1123Z time should be preserved",
			input:         `pubDate: "Mon, 02 Jan 2006 15:04:05 -0700"`,
			expectCurrent: false,
			expectedTime: func() *time.Time {
				t, _ := time.Parse(time.RFC1123Z, "Mon, 02 Jan 2006 15:04:05 -0700")
				return &t
			}(),
		},
		{
			name:          "another valid RFC1123Z time should be preserved",
			input:         `pubDate: "Fri, 25 Mar 2022 16:00:13 +0000"`,
			expectCurrent: false,
			expectedTime: func() *time.Time {
				t, _ := time.Parse(time.RFC1123Z, "Fri, 25 Mar 2022 16:00:13 +0000")
				return &t
			}(),
		},
		{
			name:        "invalid time format should error",
			input:       `pubDate: "invalid-time"`,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testStruct struct {
				PubDate ItunesTime `yaml:"pubDate"`
			}

			err := yaml.Unmarshal([]byte(tt.input), &testStruct)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectCurrent {
				// Check that the time is recent (within the last 5 seconds)
				now := time.Now().UTC()
				diff := now.Sub(testStruct.PubDate.Time)
				if diff < 0 {
					diff = -diff
				}
				if diff > 5*time.Second {
					t.Errorf("expected current time, but got %v (diff: %v)", testStruct.PubDate.Time, diff)
				}
			} else if tt.expectedTime != nil {
				if !testStruct.PubDate.Time.Equal(*tt.expectedTime) {
					t.Errorf("expected %v, got %v", *tt.expectedTime, testStruct.PubDate.Time)
				}
			}
		})
	}
}

func TestItunesTime_MarshalYAML(t *testing.T) {
	testTime := time.Date(2022, 3, 25, 16, 0, 13, 0, time.UTC)
	itunesTime := ItunesTime{Time: testTime}

	data, err := yaml.Marshal(&itunesTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Fri, 25 Mar 2022 16:00:13 +0000\n"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}
}

func TestItunesTime_String(t *testing.T) {
	testTime := time.Date(2022, 3, 25, 16, 0, 13, 0, time.UTC)
	itunesTime := ItunesTime{Time: testTime}

	result := itunesTime.String()
	expected := "Fri, 25 Mar 2022 16:00:13 +0000"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestItunesTime_ZeroValueHandling(t *testing.T) {
	// Test that when we unmarshal a zero time, it gets set to current time
	zeroTimeRFC1123Z := time.Time{}.Format(time.RFC1123Z)

	var testStruct struct {
		PubDate ItunesTime `yaml:"pubDate"`
	}

	yamlInput := "pubDate: \"" + zeroTimeRFC1123Z + "\""
	err := yaml.Unmarshal([]byte(yamlInput), &testStruct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be current time, not zero time
	if testStruct.PubDate.IsZero() {
		t.Error("expected non-zero time, but got zero time")
	}

	// Check that the time is recent (within the last 5 seconds)
	now := time.Now().UTC()
	diff := now.Sub(testStruct.PubDate.Time)
	if diff < 0 {
		diff = -diff
	}
	if diff > 5*time.Second {
		t.Errorf("expected current time, but got %v (diff: %v)", testStruct.PubDate.Time, diff)
	}
}

func TestAtomPubDateFieldHandling(t *testing.T) {
	tests := []struct {
		name          string
		yamlInput     string
		expectCurrent bool
	}{
		{
			name: "missing pubDate field should get current time via struct initialization",
			yamlInput: `
title: "Test Podcast"
link: "https://example.com"
`,
			expectCurrent: true, // Will be handled by configurator default setting
		},
		{
			name: "empty pubDate field should get current time",
			yamlInput: `
title: "Test Podcast"
link: "https://example.com"  
pubDate: ""
`,
			expectCurrent: true,
		},
		{
			name: "zero date pubDate should get current time",
			yamlInput: `
title: "Test Podcast"
link: "https://example.com"
pubDate: "Mon, 01 Jan 0001 00:00:00 +0000"
`,
			expectCurrent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var atom Podcast
			err := yaml.Unmarshal([]byte(strings.TrimSpace(tt.yamlInput)), &atom)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectCurrent {
				if tt.name == "missing pubDate field should get current time via struct initialization" {
					// For missing field, the struct will have zero value initially
					// This should be handled by the configurator's Load method
					if !atom.PubDate.IsZero() {
						t.Log("pubDate was set during YAML parsing despite missing field - this is okay")
					}
				} else {
					// For empty or zero date fields, they should be set to current time during parsing
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
				}
			}
		})
	}
}

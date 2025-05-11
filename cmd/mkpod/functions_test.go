package main

import (
	"strings"
	"testing"

	"github.com/sa6mwa/id3v24"
)

func TestSpotifyChapters(t *testing.T) {
	expected1 := `
(01:30) Introduction
(02:00) Hello world
(13:37) Famous last words
`

	expected2 := `
(00:00:29) First chapter
(00:01:02) Second chapter
(00:49:31) Third chapter
(01:00:00) Fourth chapter
`

	expected3 := `
(00:00:00) Introduction
(00:59:59) First chapter
(01:00:01) Second chapter
(01:59:59) Third chapter
(02:39:01) Final chapter
`

	chapters1 := []id3v24.Chapter{
		{
			Title: "Introduction",
			Start: "00:01:30.999",
		},
		{
			Title: "Hello world",
			Start: "00:02:00.000",
		},
		{
			Title: "Famous last words",
			Start: "00:13:37.666",
		},
	}

	chapters2 := []id3v24.Chapter{
		{
			Title: "First chapter",
			Start: "00:00:29.500",
		},
		{
			Title: "Second chapter",
			Start: "00:01:02.000",
		},
		{
			Title: "Third chapter",
			Start: "00:49:31.999",
		},
		{
			Title: "Fourth chapter",
			Start: "01:00:00.000",
		},
	}

	chapters3 := []id3v24.Chapter{
		{
			Title: "Introduction",
			Start: "00:00:00.000",
		},
		{
			Title: "First chapter",
			Start: "00:59:59.999",
		},
		{
			Title: "Second chapter",
			Start: "01:00:01.000",
		},
		{
			Title: "Third chapter",
			Start: "01:59:59.999",
		},
		{
			Title: "Final chapter",
			Start: "02:39:01.000",
		},
	}

	if got, expected := SpotifyChapters(chapters1), expected1; strings.Compare(got, expected) != 0 {
		t.Errorf("expected: %q\ngot: %q", expected, got)
	}
	if got, expected := SpotifyChapters(chapters2), expected2; strings.Compare(got, expected) != 0 {
		t.Errorf("expected: %q\ngot: %q", expected, got)
	}
	if got, expected := SpotifyChapters(chapters3), expected3; strings.Compare(got, expected) != 0 {
		t.Errorf("expected: %q\ngot: %q", expected, got)
	}
}

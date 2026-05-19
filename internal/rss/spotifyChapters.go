package rss

import (
	"fmt"
	"strings"
	"time"

	"github.com/sa6mwa/id3v24"
)

func SpotifyChapters(chapters []id3v24.Chapter) string {
	if len(chapters) == 0 {
		return ""
	}
	oneHour, err := time.Parse("15:04:05", "01:00:00")
	if err != nil {
		return ""
	}
	type spotifyChapter struct {
		title string
		start time.Time
	}
	var schaps []spotifyChapter
	var longTimeFormat bool
	for _, c := range chapters {
		s, err := id3v24.StringTimeToTime(c.Start)
		if err != nil {
			return ""
		}
		chap := spotifyChapter{
			title: c.Title,
			start: s,
		}
		schaps = append(schaps, chap)
		if !s.Before(oneHour) {
			longTimeFormat = true
		}
	}
	var output string
	for _, c := range schaps {
		format := "(%s) %s\n"
		if longTimeFormat {
			output += fmt.Sprintf(format, c.start.Format("15:04:05"), strings.TrimSpace(c.title))
		} else {
			output += fmt.Sprintf(format, c.start.Format("04:05"), strings.TrimSpace(c.title))
		}
	}
	return output
}

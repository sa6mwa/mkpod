package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/sa6mwa/id3v24"
)

// SpotifyChapters returns a list of chapters from a slice of
// id3v24.Chapters (the chapter key for each episode in the
// podspec.yaml). If there are no chapters or an error occurs during
// parsing, it returns an empty string. The format is (HH:MM:SS) if
// duration is over 59m59s, otherwise it is (MM:SS). See the following
// link for spec:
// https://support.spotify.com/us/creators/article/creating-and-managing-chapters/
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

package parser

import (
	"context"
	_ "embed"
	"text/template"
	"time"

	"github.com/alessio/shellescape"
	"github.com/sa6mwa/id3v24"
	"github.com/sa6mwa/mkpod/internal/app/model"
)

//go:embed template.rss
var rssTemplate string

var funcMap template.FuncMap

// forParsing implements the ports.ForParsing port (interface).
type forParsing struct {
	funcMap template.FuncMap
}

func (p *forParsing) WriteRSS(_ context.Context, atom *model.Atom) error {
	t, err := template.New("template.rss").Funcs(p.funcMap).Parse(rssTemplate)
	if err != nil {
		return err
	}
}

// Functions...

func mkFuncMap() template.FuncMap {
	return template.FuncMap{
		"escape": func(s string) string {
			return shellescape.Quote(s)
		},
		"timeNow": func() time.Time {
			return time.Now()
		},
		"isAfter": func(t1 time.Time, t2 time.Time) bool {
			if t1.IsZero() || t2.IsZero() {
				return false
			}
			return (t1 == t2 || t1.After(t2))
		},
		"markdown": func(s string) string {
			return MarkdownToHTML(s)
		},
		"spotifyChapters": func(chapters []id3v24.Chapter) string {
			var output string
			chaps := SpotifyChapters(chapters)
			if len([]rune(chaps)) > 0 {
				output = "\n<pre>\n"
				output += chaps
				output += "</pre>\n"
			}
			return output
		},
	}
}

package rss

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"al.essio.dev/pkg/shellescape"
	"github.com/sa6mwa/id3v24"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
)

//go:embed template.rss
var rssTemplate string

var (
	ErrNilPointerAtom error = errors.New("received nil pointer to atom")
)

func New() *Renderer {
	return &Renderer{
		funcMap: mkFuncMap(),
	}
}

type Renderer struct {
	funcMap template.FuncMap
}

func (p *Renderer) WriteRSS(ctx context.Context, atom *model.Atom) error {
	if atom == nil {
		return ErrNilPointerAtom
	}
	f, err := os.Create(atom.Atom)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeRSS(ctx, f, p, atom)
}

func (p *Renderer) WriteRSSToStdout(ctx context.Context, atom *model.Atom) error {
	return writeRSS(ctx, os.Stdout, p, atom)
}

func writeRSS(ctx context.Context, w io.Writer, p *Renderer, atom *model.Atom) error {
	funcMapWithContext := p.mkFuncMapWithContext(ctx, atom)

	t, err := template.New("template.rss").Funcs(funcMapWithContext).Parse(rssTemplate)
	if err != nil {
		return err
	}
	return t.Execute(w, atom)
}

func (p *Renderer) mkFuncMapWithContext(ctx context.Context, atom *model.Atom) template.FuncMap {
	l := logger.FromContext(ctx)
	if l == nil {
		l = logger.DefaultLogger()
	}

	funcMap := mkFuncMap()

	funcMap["validEpisodes"] = func(episodes []model.Episode) []model.Episode {
		return p.filterValidEpisodes(ctx, atom, episodes)
	}

	funcMap["episodeAuthor"] = func(episode model.Episode) string {
		if strings.TrimSpace(episode.Author) == "" {
			return atom.Author
		}
		return episode.Author
	}

	funcMap["episodeExplicit"] = func(episode model.Episode) string {
		if episode.Explicit.S == "" {
			if atom.Explicit.S != "" {
				return atom.Explicit.S
			}
			return "no"
		}
		return episode.Explicit.S
	}

	return funcMap
}

func (p *Renderer) filterValidEpisodes(ctx context.Context, atom *model.Atom, episodes []model.Episode) []model.Episode {
	l := logger.FromContext(ctx)
	if l == nil {
		l = logger.DefaultLogger()
	}

	validEpisodes := make([]model.Episode, 0, len(episodes))

	for i, episode := range episodes {
		var missingFields []string

		if strings.TrimSpace(episode.Title) == "" {
			missingFields = append(missingFields, "title")
		}
		if strings.TrimSpace(episode.Output) == "" {
			missingFields = append(missingFields, "output")
		}
		if episode.Duration.Duration == 0 {
			missingFields = append(missingFields, "duration")
		}
		if episode.Length == 0 {
			missingFields = append(missingFields, "length")
		}
		if strings.TrimSpace(episode.Type) == "" {
			missingFields = append(missingFields, "type")
		}
		if strings.TrimSpace(episode.Image) == "" {
			missingFields = append(missingFields, "image")
		}
		if episode.PubDate.IsZero() {
			missingFields = append(missingFields, "pubDate")
		}

		effectiveAuthor := episode.Author
		if strings.TrimSpace(effectiveAuthor) == "" {
			effectiveAuthor = atom.Author
		}
		if strings.TrimSpace(effectiveAuthor) == "" {
			missingFields = append(missingFields, "author")
		}

		if len(missingFields) > 0 {
			l.Warn("Excluding episode from RSS due to missing required fields",
				"episode", i+1,
				"uid", episode.UID,
				"title", episode.Title,
				"missingFields", strings.Join(missingFields, ", "),
				"message", "These fields can be resolved by encoding or re-encoding the episode")
			continue
		}

		validEpisodes = append(validEpisodes, episode)
	}

	if len(validEpisodes) != len(episodes) {
		excludedCount := len(episodes) - len(validEpisodes)
		l.Info("Episode filtering complete",
			"totalEpisodes", len(episodes),
			"validEpisodes", len(validEpisodes),
			"excludedEpisodes", excludedCount)
	}

	return validEpisodes
}

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
			return t1 == t2 || t1.After(t2)
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

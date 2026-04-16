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
	"github.com/sa6mwa/mkpod/internal/spec"
)

//go:embed template.rss
var rssTemplate string

var (
	ErrNilPointerPodcast error = errors.New("received nil pointer to podcast")
)

func New() *Renderer {
	return &Renderer{
		funcMap: mkFuncMap(),
	}
}

type Renderer struct {
	funcMap template.FuncMap
}

func (p *Renderer) WriteRSS(ctx context.Context, atom *model.Podcast) error {
	if atom == nil {
		return ErrNilPointerPodcast
	}
	f, err := os.Create(atom.FeedFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeRSS(ctx, f, p, atom)
}

func (p *Renderer) WriteRSSToStdout(ctx context.Context, atom *model.Podcast) error {
	return writeRSS(ctx, os.Stdout, p, atom)
}

func writeRSS(ctx context.Context, w io.Writer, p *Renderer, atom *model.Podcast) error {
	funcMapWithContext := p.mkFuncMapWithContext(ctx, atom)

	t, err := template.New("template.rss").Funcs(funcMapWithContext).Parse(rssTemplate)
	if err != nil {
		return err
	}
	return t.Execute(w, atom)
}

func (p *Renderer) mkFuncMapWithContext(ctx context.Context, atom *model.Podcast) template.FuncMap {
	l := logger.FromContext(ctx)
	if l == nil {
		l = logger.DefaultLogger()
	}

	funcMap := mkFuncMap()

	funcMap["validEpisodes"] = func(episodes []model.Episode) []model.Episode {
		return p.filterValidEpisodes(ctx, atom, episodes)
	}

	funcMap["episodeAuthor"] = func(episode model.Episode) string {
		return spec.EffectiveEpisodeAuthor(atom, &episode)
	}

	funcMap["episodeExplicit"] = func(episode model.Episode) string {
		return spec.EffectiveEpisodeExplicit(atom, &episode)
	}

	return funcMap
}

func (p *Renderer) filterValidEpisodes(ctx context.Context, atom *model.Podcast, episodes []model.Episode) []model.Episode {
	l := logger.FromContext(ctx)
	if l == nil {
		l = logger.DefaultLogger()
	}

	validEpisodes, issues := spec.RenderableEpisodes(atom, episodes)
	for _, issue := range issues {
		l.Warn("Excluding episode from RSS due to missing required fields",
			"episode", issue.Index+1,
			"uid", issue.UID,
			"title", issue.Title,
			"missingFields", strings.Join(issue.MissingFields, ", "),
			"message", "These fields can be resolved by encoding or re-encoding the episode")
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

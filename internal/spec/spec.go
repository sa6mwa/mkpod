package spec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"gopkg.in/yaml.v3"
)

var DefaultSpecfile = "podspec.yaml"

var ErrNilPodcast = errors.New("received nil pointer to podcast")

func New(filename string) *Store {
	if filename == "" {
		filename = DefaultSpecfile
	}
	return &Store{specFile: filename}
}

type Store struct {
	specFile string
}

func (s *Store) Load(ctx context.Context) (*model.Podcast, error) {
	f, err := os.Open(s.specFile)
	if err != nil {
		return nil, err
	}
	var atom model.Podcast
	if err := yaml.NewDecoder(f).Decode(&atom); err != nil {
		return nil, err
	}
	if err := Validate(&atom); err != nil {
		return nil, fmt.Errorf("validation failed for %s: %w", s.specFile, err)
	}
	ApplyDefaults(&atom, time.Now().UTC())
	return &atom, nil
}

func (s *Store) Save(ctx context.Context, atom *model.Podcast) error {
	atom.LastBuildDate.Time = time.Now().UTC()
	f, err := os.Create(s.specFile)
	if err != nil {
		return fmt.Errorf("unable to re-write %s: %w", s.specFile, err)
	}
	defer f.Close()
	if err := yaml.NewEncoder(f).Encode(atom); err != nil {
		return fmt.Errorf("unable to marshall yaml: %w", err)
	}
	return nil
}

func Validate(atom *model.Podcast) error {
	if atom == nil {
		return ErrNilPodcast
	}

	var missingFields []string

	for _, field := range RequiredTopLevelFields {
		if isMissingTopLevelField(atom, field) {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("required fields are missing or empty: %s", strings.Join(missingFields, ", "))
	}
	return nil
}

func isMissingTopLevelField(podcast *model.Podcast, field string) bool {
	switch field {
	case "author":
		return strings.TrimSpace(podcast.Author) == ""
	case "config.baseURL":
		return strings.TrimSpace(podcast.Config.BaseURL) == ""
	case "config.image":
		return strings.TrimSpace(podcast.Config.Image) == ""
	case "config.defaultPodImage":
		return strings.TrimSpace(podcast.Config.DefaultPodImage) == ""
	case "atom":
		return strings.TrimSpace(podcast.FeedFile) == ""
	case "title":
		return strings.TrimSpace(podcast.Title) == ""
	case "ttl":
		return podcast.TTL == 0
	case "language":
		return strings.TrimSpace(podcast.Language) == ""
	case "copyright":
		return strings.TrimSpace(podcast.Copyright) == ""
	case "webMaster":
		return strings.TrimSpace(podcast.WebMaster) == ""
	case "description":
		return strings.TrimSpace(podcast.Description) == ""
	case "subtitle":
		return strings.TrimSpace(podcast.Subtitle) == ""
	case "ownerName":
		return strings.TrimSpace(podcast.OwnerName) == ""
	case "ownerEmail":
		return strings.TrimSpace(podcast.OwnerEmail) == ""
	default:
		panic("unknown required top-level field: " + field)
	}
}

func ApplyDefaults(atom *model.Podcast, now time.Time) error {
	if atom == nil {
		return ErrNilPodcast
	}
	if atom.Encoding.CRF == 0 {
		atom.Encoding.CRF = 28
	}
	if strings.TrimSpace(atom.Encoding.ABR) == "" {
		atom.Encoding.ABR = "128k"
	}
	if strings.TrimSpace(atom.Encoding.FFmpegPath) == "" {
		atom.Encoding.FFmpegPath = "ffmpeg"
	}
	if strings.TrimSpace(atom.Encoding.Lamepath) == "" {
		atom.Encoding.Lamepath = "lame"
	}
	if atom.PubDate.IsZero() {
		if now.IsZero() {
			now = time.Now().UTC()
		}
		atom.PubDate.Time = now.UTC()
	}
	return nil
}

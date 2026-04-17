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

var (
	ErrNilPodcast           = errors.New("received nil pointer to podcast")
	ErrUnknownRequiredField = errors.New("unknown required top-level field")
)

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
		return nil, fmt.Errorf("unable to open %s: %w", s.specFile, err)
	}
	var atom model.Podcast
	if err := yaml.NewDecoder(f).Decode(&atom); err != nil {
		return nil, fmt.Errorf("invalid YAML in %s: %w", s.specFile, err)
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
		missing, err := isMissingTopLevelField(atom, field)
		if err != nil {
			return err
		}
		if missing {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("required fields are missing or empty: %s", strings.Join(missingFields, ", "))
	}
	return nil
}

func isMissingTopLevelField(podcast *model.Podcast, field string) (bool, error) {
	switch field {
	case "author":
		return strings.TrimSpace(podcast.Author) == "", nil
	case "config.baseURL":
		return strings.TrimSpace(podcast.Config.BaseURL) == "", nil
	case "config.image":
		return strings.TrimSpace(podcast.Config.Image) == "", nil
	case "config.defaultPodImage":
		return strings.TrimSpace(podcast.Config.DefaultPodImage) == "", nil
	case "atom":
		return strings.TrimSpace(podcast.FeedFile) == "", nil
	case "title":
		return strings.TrimSpace(podcast.Title) == "", nil
	case "ttl":
		return podcast.TTL == 0, nil
	case "language":
		return strings.TrimSpace(podcast.Language) == "", nil
	case "copyright":
		return strings.TrimSpace(podcast.Copyright) == "", nil
	case "webMaster":
		return strings.TrimSpace(podcast.WebMaster) == "", nil
	case "description":
		return strings.TrimSpace(podcast.Description) == "", nil
	case "subtitle":
		return strings.TrimSpace(podcast.Subtitle) == "", nil
	case "ownerName":
		return strings.TrimSpace(podcast.OwnerName) == "", nil
	case "ownerEmail":
		return strings.TrimSpace(podcast.OwnerEmail) == "", nil
	default:
		return false, fmt.Errorf("%w: %s", ErrUnknownRequiredField, field)
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

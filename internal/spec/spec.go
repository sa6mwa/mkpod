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

var ErrNilAtom = errors.New("received nil pointer to atom")

func New(filename string) *Store {
	if filename == "" {
		filename = DefaultSpecfile
	}
	return &Store{specFile: filename}
}

type Store struct {
	specFile string
}

func (s *Store) Load(ctx context.Context) (*model.Atom, error) {
	f, err := os.Open(s.specFile)
	if err != nil {
		return nil, err
	}
	var atom model.Atom
	if err := yaml.NewDecoder(f).Decode(&atom); err != nil {
		return nil, err
	}
	if err := Validate(&atom); err != nil {
		return nil, fmt.Errorf("validation failed for %s: %w", s.specFile, err)
	}
	ApplyDefaults(&atom, time.Now().UTC())
	return &atom, nil
}

func (s *Store) Save(ctx context.Context, atom *model.Atom) error {
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

func Validate(atom *model.Atom) error {
	if atom == nil {
		return ErrNilAtom
	}

	var missingFields []string

	if strings.TrimSpace(atom.Author) == "" {
		missingFields = append(missingFields, "author")
	}
	if strings.TrimSpace(atom.Config.BaseURL) == "" {
		missingFields = append(missingFields, "config.baseURL")
	}
	if strings.TrimSpace(atom.Config.Image) == "" {
		missingFields = append(missingFields, "config.image")
	}
	if strings.TrimSpace(atom.Config.DefaultPodImage) == "" {
		missingFields = append(missingFields, "config.defaultPodImage")
	}
	if strings.TrimSpace(atom.Atom) == "" {
		missingFields = append(missingFields, "atom")
	}
	if strings.TrimSpace(atom.Title) == "" {
		missingFields = append(missingFields, "title")
	}
	if atom.TTL == 0 {
		missingFields = append(missingFields, "ttl")
	}
	if strings.TrimSpace(atom.Language) == "" {
		missingFields = append(missingFields, "language")
	}
	if strings.TrimSpace(atom.Copyright) == "" {
		missingFields = append(missingFields, "copyright")
	}
	if strings.TrimSpace(atom.WebMaster) == "" {
		missingFields = append(missingFields, "webMaster")
	}
	if strings.TrimSpace(atom.Description) == "" {
		missingFields = append(missingFields, "description")
	}
	if strings.TrimSpace(atom.Subtitle) == "" {
		missingFields = append(missingFields, "subtitle")
	}
	if strings.TrimSpace(atom.OwnerName) == "" {
		missingFields = append(missingFields, "ownerName")
	}
	if strings.TrimSpace(atom.OwnerEmail) == "" {
		missingFields = append(missingFields, "ownerEmail")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("required fields are missing or empty: %s", strings.Join(missingFields, ", "))
	}
	return nil
}

func ApplyDefaults(atom *model.Atom, now time.Time) error {
	if atom == nil {
		return ErrNilAtom
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

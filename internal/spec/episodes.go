package spec

import (
	"errors"
	"strings"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

var (
	ErrMissingEpisodeImage = errors.New("episode image is required for encoding - no image specified and no defaultPodImage configured")
	ErrMissingEpisodeTitle = errors.New("episode title is required for encoding")
)

func ApplyEpisodeDefaultsForEncoding(atom *model.Atom, episode *model.Episode) error {
	if atom == nil || episode == nil {
		return ErrNilAtom
	}
	if strings.TrimSpace(episode.Title) == "" {
		return ErrMissingEpisodeTitle
	}
	if strings.TrimSpace(episode.Author) == "" {
		episode.Author = atom.Author
	}
	if strings.TrimSpace(episode.Image) == "" {
		episode.Image = atom.Config.DefaultPodImage
	}
	if strings.TrimSpace(episode.Image) == "" {
		return ErrMissingEpisodeImage
	}
	return nil
}

func MissingFieldsForRSS(atom *model.Atom, episode *model.Episode) []string {
	if atom == nil || episode == nil {
		return []string{"atom"}
	}

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

	return missingFields
}

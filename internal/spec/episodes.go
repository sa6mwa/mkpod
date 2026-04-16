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

// ParsePolicySkipInvalidEpisodes defines the current feed-generation policy:
// invalid episodes are excluded from RSS output and reported as warnings
// instead of failing the entire parse operation.
const ParsePolicySkipInvalidEpisodes = "skip-invalid-episodes-with-warnings"

type EpisodeValidationIssue struct {
	Index         int
	UID           int64
	Title         string
	MissingFields []string
}

func EffectiveEpisodeAuthor(atom *model.Atom, episode *model.Episode) string {
	if atom == nil || episode == nil {
		return ""
	}
	if strings.TrimSpace(episode.Author) != "" {
		return episode.Author
	}
	return atom.Author
}

func EffectiveEpisodeExplicit(atom *model.Atom, episode *model.Episode) string {
	if atom == nil || episode == nil {
		return "no"
	}
	if episode.Explicit.S != "" {
		return episode.Explicit.S
	}
	if atom.Explicit.S != "" {
		return atom.Explicit.S
	}
	return "no"
}

func EffectiveEpisodeImage(atom *model.Atom, episode *model.Episode) string {
	if atom == nil || episode == nil {
		return ""
	}
	if strings.TrimSpace(episode.Image) != "" {
		return episode.Image
	}
	return atom.Config.DefaultPodImage
}

func ApplyEpisodeDefaultsForEncoding(atom *model.Atom, episode *model.Episode) error {
	if atom == nil || episode == nil {
		return ErrNilAtom
	}
	if strings.TrimSpace(episode.Title) == "" {
		return ErrMissingEpisodeTitle
	}
	episode.Author = EffectiveEpisodeAuthor(atom, episode)
	episode.Image = EffectiveEpisodeImage(atom, episode)
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

	effectiveAuthor := EffectiveEpisodeAuthor(atom, episode)
	if strings.TrimSpace(effectiveAuthor) == "" {
		missingFields = append(missingFields, "author")
	}

	return missingFields
}

func RenderableEpisodes(atom *model.Atom, episodes []model.Episode) ([]model.Episode, []EpisodeValidationIssue) {
	validEpisodes := make([]model.Episode, 0, len(episodes))
	issues := make([]EpisodeValidationIssue, 0)

	for i, episode := range episodes {
		missingFields := MissingFieldsForRSS(atom, &episode)
		if len(missingFields) > 0 {
			issues = append(issues, EpisodeValidationIssue{
				Index:         i,
				UID:           episode.UID,
				Title:         episode.Title,
				MissingFields: missingFields,
			})
			continue
		}
		validEpisodes = append(validEpisodes, episode)
	}

	return validEpisodes, issues
}

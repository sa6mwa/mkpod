package encode

import (
	"fmt"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

type EpisodePlan struct {
	Mode          string
	Format        string
	Output        string
	ContentType   string
	Preferred     string
	EpisodeFormat string
}

func PlanEpisode(atom *model.Podcast, episode *model.Episode, inputContentType string) (*EpisodePlan, error) {
	mode, format, err := selectEncodeMode(inputContentType, episode.Format, atom.Encoding.PreferredFormat)
	if err != nil {
		return nil, err
	}

	output, err := plannedOutputForMode(mode, format, episode.Input)
	if err != nil {
		return nil, err
	}

	return &EpisodePlan{
		Mode:          string(mode),
		Format:        format,
		Output:        output,
		ContentType:   inputContentType,
		Preferred:     atom.Encoding.PreferredFormat,
		EpisodeFormat: episode.Format,
	}, nil
}

func plannedOutputForMode(mode encodeMode, format, input string) (string, error) {
	switch mode {
	case modeMP4:
		return ExtensionToBaseMp4(input), nil
	case modeMP3, modeMP3ViaFFmpeg:
		return ExtensionToBaseMp3(input), nil
	case modeFFmpegAudio:
		return ExtensionToBaseFormat(input, format), nil
	default:
		return "", fmt.Errorf("unsupported encode mode %q", mode)
	}
}

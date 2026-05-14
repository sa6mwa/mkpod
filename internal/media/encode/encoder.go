package encode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/sa6mwa/id3v24"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/media"
	"github.com/sa6mwa/mkpod/internal/prompt"
	"github.com/sa6mwa/mkpod/internal/spec"
	"github.com/sa6mwa/mp3duration"
)

var (
	ErrNilPointer   error = errors.New("received nil pointer")
	ErrMissingImage error = spec.ErrMissingEpisodeImage
	ErrMissingTitle error = spec.ErrMissingEpisodeTitle
)

type PostEncodeFunc func(atom *model.Podcast, episode *model.Episode, wasEncoded bool) error

type EncodeOptions struct {
	All            bool
	EpisodeUID     *int64
	ForceReencode  bool
	NeverReencode  bool
	PrepareEpisode PrepareEncodeFunc
}

type EncodeResult struct {
	SelectedIndexes []int
	EncodedOutputs  []string
	MetadataChanged bool
}

type encodeMode string

const (
	modeMP3          encodeMode = "mp3"
	modeMP3ViaFFmpeg encodeMode = "mp3-via-ffmpeg"
	modeFFmpegAudio  encodeMode = "ffmpeg-audio"
	modeMP4          encodeMode = "mp4"
)

type Service struct {
	prompter prompt.Prompter
}

func New(prompter prompt.Prompter) *Service {
	return &Service{
		prompter: prompter,
	}
}

// applyEpisodeDefaults ensures episode has required fields set with appropriate defaults
func applyEpisodeDefaults(atom *model.Podcast, episode *model.Episode) error {
	return spec.ApplyEpisodeDefaultsForEncoding(atom, episode)
}

func (e *Service) shouldEncode(ctx context.Context, options EncodeOptions, filename string) bool {
	// l := logger.FromContext(ctx)
	if options.NeverReencode {
		return false
	} else if len(strings.TrimSpace(filename)) < 5 {
		return true
	} else if options.ForceReencode {
		return true
	} else if _, err := os.Stat(filename); os.IsNotExist(err) {
		return true
	} else if options.All {
		return false
	}
	return e.prompter.Ask(ctx, "Re-encode %s?", filename)
}

func selectEpisodeIndexes(atom *model.Podcast, options EncodeOptions) ([]int, error) {
	if atom == nil {
		return nil, ErrNilPointer
	}
	if options.All {
		indexes := make([]int, 0, len(atom.Episodes))
		for i := range atom.Episodes {
			indexes = append(indexes, i)
		}
		return indexes, nil
	}
	if options.EpisodeUID == nil {
		return nil, errors.New("episode UID is required when encoding without --all")
	}
	if idx := atom.ContainsEpisode(*options.EpisodeUID); idx >= 0 {
		return []int{int(idx)}, nil
	}
	return nil, nil
}

func selectEncodeMode(inputContentType, episodeFormat, preferredFormat string) (encodeMode, string, error) {
	format := strings.TrimSpace(strings.ToLower(episodeFormat))
	preferred := strings.TrimSpace(strings.ToLower(preferredFormat))

	if strings.HasPrefix(inputContentType, "video/") {
		switch format {
		case "", "video", "mp4":
			return modeMP4, "", nil
		case "audio":
			if preferred == "m4a" || preferred == "m4b" {
				return modeFFmpegAudio, preferred, nil
			}
			return modeMP3ViaFFmpeg, "", nil
		case "mp3":
			return modeMP3ViaFFmpeg, "", nil
		case "m4a", "m4b":
			return modeFFmpegAudio, format, nil
		default:
			return "", "", fmt.Errorf("invalid or unsupported format %q", format)
		}
	}

	switch format {
	case "", "audio":
		if preferred == "m4a" || preferred == "m4b" {
			return modeFFmpegAudio, preferred, nil
		}
		return modeMP3, "", nil
	case "mp3":
		return modeMP3, "", nil
	case "m4a", "m4b":
		return modeFFmpegAudio, format, nil
	default:
		return "", "", fmt.Errorf("invalid or unsupported format %q", format)
	}
}

func (e *Service) encodeEpisode(ctx context.Context, atom *model.Podcast, episode *model.Episode, inputContentType string) error {
	mode, formatArg, err := selectEncodeMode(inputContentType, episode.Format, atom.Encoding.PreferredFormat)
	if err != nil {
		return err
	}
	switch mode {
	case modeMP4:
		return EncodeMP4(ctx, atom, episode)
	case modeMP3ViaFFmpeg:
		return EncodeMP3ViaFFmpeg(ctx, atom, episode)
	case modeFFmpegAudio:
		return EncodeFFmpegAudio(ctx, atom, episode, formatArg)
	case modeMP3:
		return EncodeMP3(ctx, atom, episode)
	default:
		return fmt.Errorf("unsupported encode mode %q", mode)
	}
}

func (e *Service) Encode(ctx context.Context, atom *model.Podcast, options EncodeOptions, postEncoding PostEncodeFunc) (*EncodeResult, error) {
	l := logger.FromContext(ctx)
	mimetype.SetLimit(1024 * 1024)

	indexes, err := selectEpisodeIndexes(atom, options)
	if err != nil {
		return nil, err
	}
	result := &EncodeResult{
		SelectedIndexes: indexes,
		EncodedOutputs:  make([]string, 0, len(indexes)),
	}
	if len(indexes) == 0 && options.EpisodeUID != nil {
		l.Warn("Episode does not exist in pod specification, skipping", "uid", *options.EpisodeUID)
		return result, nil
	}

	for _, i := range indexes {
		episode := &atom.Episodes[i]
		if err := applyEpisodeDefaults(atom, episode); err != nil {
			return nil, err
		}
		if ensureEpisodePubDate(episode) {
			result.MetadataChanged = true
		}

		outputPath := ""
		if strings.TrimSpace(episode.Output) != "" {
			outputPath = path.Join(atom.LocalStorageDirExpanded(), episode.Output)
		}
		if options.NeverReencode {
			if strings.TrimSpace(outputPath) == "" {
				return nil, fmt.Errorf("repair-only encode requires existing episode output metadata for episode %d", episode.UID)
			}
			if _, err := os.Stat(outputPath); err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("repair-only encode requires existing output file %s", outputPath)
				}
				return nil, fmt.Errorf("failed to access repair-only output file %s: %w", outputPath, err)
			}
		}
		if changed, err := repairOutputMetadata(ctx, episode, outputPath); err != nil {
			return nil, err
		} else if changed {
			result.MetadataChanged = true
		}

		wasEncoded := false
		if e.shouldEncode(ctx, options, outputPath) {
			if options.PrepareEpisode != nil {
				if err := options.PrepareEpisode(ctx, atom, episode); err != nil {
					return nil, err
				}
			}
			inputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Input)
			inputContentType, err := GetFileContentType(inputPath)
			if err != nil {
				return nil, err
			}
			wasEncoded = true
			if err := e.encodeEpisode(ctx, atom, episode, inputContentType); err != nil {
				return nil, err
			}

			result.EncodedOutputs = append(result.EncodedOutputs, episode.Output)
			result.MetadataChanged = true
		}

		if postEncoding != nil {
			if err := postEncoding(atom, episode, wasEncoded); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// EncodeMP4 encodes episode into an mp4 video (using ffmpeg)
func EncodeMP4(ctx context.Context, atom *model.Podcast, episode *model.Episode) error {
	l := logger.FromContext(ctx)
	if atom == nil || episode == nil {
		return ErrNilPointer
	}
	if err := media.EnsureToolAvailable(atom.FFmpegPathExpanded()); err != nil {
		return err
	}
	episode.Output = ExtensionToBaseMp4(episode.Input)
	args := buildMP4Args(atom, episode)
	l.Info("Executing encoder", "tool", atom.FFmpegPathExpanded(), "args", args)
	if err := runCommand(ctx, atom.FFmpegPathExpanded(), args); err != nil {
		return fmt.Errorf("unable to encode %q: %w", episode.Input, err)
	}
	// Update atom with the length and duration of the encoded mp4
	size, duration, err := Mp4Duration(path.Join(atom.LocalStorageDirExpanded(), episode.Output))
	if err != nil {
		return err
	}
	// Set content type based on output file
	outputContentType, err := GetFileContentType(path.Join(atom.LocalStorageDirExpanded(), episode.Output))
	if err != nil {
		return err
	}
	episode.Type = outputContentType

	l.Info(fmt.Sprintf("%s is %s long and %d bytes", episode.Output, duration, size), "output", episode.Output, "duration", duration, "size", size)
	episode.Length = size
	episode.Duration.Duration = duration
	return nil
}

// EncodeFFmpegAudio encodes episode.Input into an m4a or m4b file
// depending on the value of format.
func EncodeFFmpegAudio(ctx context.Context, atom *model.Podcast, episode *model.Episode, format string) error {
	l := logger.FromContext(ctx)
	if atom == nil || episode == nil {
		return ErrNilPointer
	}
	if err := media.EnsureToolAvailable(atom.FFmpegPathExpanded()); err != nil {
		return err
	}
	format = strings.TrimSpace(strings.ToLower(format))
	episode.Output = ExtensionToBaseFormat(episode.Input, format)
	rplcr := strings.NewReplacer("\n", " ", "\r", "")
	lang := atom.Encoding.Language
	if episode.EncodingLanguage != "" {
		lang = episode.EncodingLanguage
	}

	trackInfo := id3v24.TrackInfo{
		Title:       episode.Title,
		Album:       atom.Title,
		Artist:      episode.Author,
		Genre:       atom.Encoding.Genre,
		Year:        episode.PubDate.Format("2006"),
		Date:        episode.PubDate.Time,
		Track:       fmt.Sprintf("%d", episode.UID),
		Comment:     episode.Link,
		Description: rplcr.Replace(episode.Subtitle),
		Language:    strings.ToLower(lang),
		CoverJPEG:   path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront),
		Chapters:    episode.Chapters,
	}

	// Get duration of original input file
	duration, size, err := GetSizeAndDurationViaFFprobe(path.Join(atom.LocalStorageDirExpanded(), episode.Input))
	if err != nil {
		return fmt.Errorf("unable to get duration and size from input file: %w", err)
	}
	// Generate metadata /w chapters (if any)
	metadataFile, cleanupMetadata, err := writeFFmpegMetadataFile(duration, trackInfo)
	if err != nil {
		return fmt.Errorf("unable to generate ffmetadata file: %w", err)
	}
	defer cleanupMetadata()

	args := buildFFmpegAudioArgs(atom, episode, metadataFile)
	l.Info("Executing encoder", "tool", atom.FFmpegPathExpanded(), "args", args, "input", episode.Input, "output", episode.Output)
	if err := runCommand(ctx, atom.FFmpegPathExpanded(), args); err != nil {
		return fmt.Errorf("unable to encode %q: %w", episode.Input, err)
	}

	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	// Get correct duration and size of the output file
	duration, size, err = GetSizeAndDurationViaFFprobe(outputPath)
	if err != nil {
		return fmt.Errorf("unable to get duration and size from %s: %w", episode.Output, err)
	}
	// Set content type based on output file
	outputContentType, err := GetFileContentType(outputPath)
	if err != nil {
		return err
	}
	episode.Type = outputContentType

	// Update episode length and duration
	l.Info(fmt.Sprintf("%s is %s long and %d bytes", episode.Output, duration, size), "output", episode.Output, "duration", duration, "size", size)
	episode.Length = size
	episode.Duration.Duration = duration
	return nil
}

func writeFFmpegMetadataFile(duration time.Duration, trackInfo id3v24.TrackInfo) (string, func(), error) {
	metadataFile, err := id3v24.WriteFFmpegMetadataFile(duration, trackInfo)
	if err != nil {
		return "", func() {}, err
	}

	return metadataFile, func() {
		_ = os.Remove(metadataFile)
	}, nil
}

// EncodeMP3ViaFFmpeg encodes episode.Input through ffmpeg piped into
// lame as an mp3.
func EncodeMP3ViaFFmpeg(ctx context.Context, atom *model.Podcast, episode *model.Episode) error {
	l := logger.FromContext(ctx)
	if atom == nil || episode == nil {
		return ErrNilPointer
	}
	if err := media.EnsureToolAvailable(atom.FFmpegPathExpanded()); err != nil {
		return err
	}
	if err := media.EnsureToolAvailable(atom.LamepathExpanded()); err != nil {
		return err
	}
	episode.Output = ExtensionToBaseMp3(episode.Input)
	ffmpegArgs := buildFFmpegToWavArgs(atom, episode)
	lameArgs := buildLamePipeArgs(atom, episode)
	l.Info("Executing encoder pipeline", "ffmpeg", ffmpegArgs, "lame", lameArgs, "input", episode.Input, "output", episode.Output)
	if err := runPipeline(ctx, atom.FFmpegPathExpanded(), ffmpegArgs, atom.LamepathExpanded(), lameArgs); err != nil {
		return fmt.Errorf("unable to encode %q: %w", episode.Input, err)
	}

	// Add ID3v2.4 tag (artist, album, title, chapters, etc.).
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	l.Info("Adding ID3v2.4 tag", "file", outputPath)
	if err := id3v24.WriteID3v2Tag(outputPath, id3v24.TrackInfo{
		Title:     episode.Title,
		Album:     atom.Title,
		Artist:    episode.Author,
		Genre:     atom.Encoding.Genre,
		Year:      episode.PubDate.Format("2006"),
		CoverJPEG: path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront),
		Chapters:  episode.Chapters,
	}); err != nil {
		return err
	}
	// Get duration and length
	di, err := mp3duration.ReadFile(outputPath)
	if err != nil {
		return err
	}
	// Set content type based on output file
	outputContentType, err := GetFileContentType(outputPath)
	if err != nil {
		return err
	}
	episode.Type = outputContentType

	// Update atom with the length and duration of the encoded mp3.
	l.Info(fmt.Sprintf("%s is %s long and %d bytes", episode.Output, di.Duration, di.Length), "output", episode.Output, "duration", di.Duration, "size", di.Length)
	episode.Length = di.Length
	episode.Duration.Duration = di.TimeDuration
	return nil
}

// EncodeMP3 encodes an mp3 using lame.
func EncodeMP3(ctx context.Context, atom *model.Podcast, episode *model.Episode) error {
	l := logger.FromContext(ctx)
	if atom == nil || episode == nil {
		return ErrNilPointer
	}
	if err := media.EnsureToolAvailable(atom.LamepathExpanded()); err != nil {
		return err
	}
	episode.Output = ExtensionToBaseMp3(episode.Input)
	args := buildLameArgs(atom, episode)
	l.Info("Executing encoder", "tool", atom.LamepathExpanded(), "args", args, "input", episode.Input, "output", episode.Output)
	if err := runCommand(ctx, atom.LamepathExpanded(), args); err != nil {
		return fmt.Errorf("unable to encode %q: %w", episode.Input, err)
	}
	// Add ID3v2.4 tag (artist, album, title, chapters, etc.).
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	l.Info("Adding ID3v2.4 tag", "file", outputPath)
	if err := id3v24.WriteID3v2Tag(outputPath, id3v24.TrackInfo{
		Title:     episode.Title,
		Album:     atom.Title,
		Artist:    episode.Author,
		Genre:     atom.Encoding.Genre,
		Year:      episode.PubDate.Format("2006"),
		CoverJPEG: path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront),
		Chapters:  episode.Chapters,
	}); err != nil {
		return err
	}
	// Get duration and length
	di, err := mp3duration.ReadFile(outputPath)
	if err != nil {
		return err
	}
	// Set content type based on output file
	outputContentType, err := GetFileContentType(outputPath)
	if err != nil {
		return err
	}
	episode.Type = outputContentType

	// Update atom with the length and duration of the encoded mp3.
	l.Info(fmt.Sprintf("%s is %s long and %d bytes", episode.Output, di.Duration, di.Length), "output", episode.Output, "duration", di.Duration, "size", di.Length)
	episode.Length = di.Length
	episode.Duration.Duration = di.TimeDuration
	return nil
}

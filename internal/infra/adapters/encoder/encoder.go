// encoder is the default file-based encoder of master audio or video
// files into published mp3, m4a or mp4 outputs. Implements the
// ports.ForEncoding interface.
package encoder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/gabriel-vasile/mimetype"
	"github.com/sa6mwa/id3v24"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
	"github.com/sa6mwa/mp3duration"
)

var (
	ErrNilPointer error = errors.New("received nil pointer")
)

const shell = "/bin/sh"
const shellCommandOption = "-c"
const defaultStorageClass = "INTELLIGENT_TIERING"

type forEncoding struct {
	ports.ForAsking
	storageClass   string
	encodedOutputs []string
}

// Returns a new encoder adapter implementing the ForEncoding port
// interface. outputStorageClass is an AWS storage class for the
// production (output) audio and/or video files (defaults to
// INTELLIGENT_TIERING if empty).
func New(askerAdapter ports.ForAsking, outputStorageClass string) ports.ForEncoding {
	if strings.TrimSpace(outputStorageClass) == "" {
		outputStorageClass = defaultStorageClass
	}
	return &forEncoding{
		ForAsking:      askerAdapter,
		storageClass:   outputStorageClass,
		encodedOutputs: make([]string, 0),
	}
}

func (e *forEncoding) GetEncodedOutputs() []string {
	return e.encodedOutputs
}

func (e *forEncoding) Encode(ctx context.Context, atom *model.Atom, uid int64, uploaders ...ports.ForUploading) error {
	var indexes []int = make([]int, 0)

	l := logger.FromContext(ctx)
	mimetype.SetLimit(1024 * 1024)

	if uid < 0 {
		// Iterate all episodes into the indexes slice
		for i, _ := range atom.Episodes {
			indexes = append(indexes, i)
		}
	} else {
		if idx := atom.ContainsEpisode(uid); idx >= 0 {
			indexes = append(indexes, int(idx))
		} else {
			l.Warn("Episode does not exist in pod specification, skipping", "uid", uid)
			return nil
		}
	}

	// Iterate over one episode or all episodes depending on value of
	// uid.
	for _, i := range indexes {
		inputPath := path.Join(atom.LocalStorageDirExpanded(), atom.Episodes[i].Input)
		inputContentType, err := GetFileContentType(inputPath)
		if err != nil {
			return err
		}
		format := strings.TrimSpace(strings.ToLower(atom.Episodes[i].Format))

		// If input content type is video/* and format is not "audio",
		// we are to encode it using ffmpeg to an mp4. If format is
		// "audio", drop the video stream and encode an mp3 (audio
		// only).
		if strings.HasPrefix(inputContentType, "video/") {
			// If episode format is video or mp4, it's a video episode.
			switch format {
			case "", "video", "mp4":
				//continue here, implement EncodeMP4 function
				if err := EncodeMP4(ctx, atom, &atom.Episodes[i]); err != nil {
					return err
				}
			case "audio":
				if strings.EqualFold(atom.Encoding.PreferredFormat, "m4a") || strings.EqualFold(atom.Encoding.PreferredFormat, "m4b") {
					// Encode into m4a or m4b
					if err := EncodeFFmpegAudio(ctx, atom, &atom.Episodes[i], atom.Encoding.PreferredFormat); err != nil {
						return err
					}
				} else {
					// Encode mp3 via ffmpeg (piped into lame)
					if err := EncodeMP3ViaFFmpeg(ctx, atom, &atom.Episodes[i]); err != nil {
						return err
					}
				}
			case "mp3":
				// Encode mp3 via ffmpeg
				if err := EncodeMP3ViaFFmpeg(ctx, atom, &atom.Episodes[i]); err != nil {
					return err
				}
			case "m4a", "m4b":
				// Encode m4a or m4b
				if err := EncodeFFmpegAudio(ctx, atom, &atom.Episodes[i], format); err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid or unsupported format %q", format)
			}
		} else {
			// ...else, assume it's audio only and encode it to either
			// mp3 using lame or m4a/m4b using ffmpeg
			switch format {
			case "", "audio":
				if strings.EqualFold(atom.Encoding.PreferredFormat, "m4a") || strings.EqualFold(atom.Encoding.PreferredFormat, "m4b") {
					// Encode into m4a or m4b
					if err := EncodeFFmpegAudio(ctx, atom, &atom.Episodes[i], atom.Encoding.PreferredFormat); err != nil {
						return err
					}
				} else {
					// Encode mp3 using lame
					if err := EncodeMP3(ctx, atom, &atom.Episodes[i]); err != nil {
						return err
					}
				}
			case "mp3":
				// Encode mp3 using lame
				if err := EncodeMP3(ctx, atom, &atom.Episodes[i]); err != nil {
					return err
				}
			case "m4a", "m4b":
				// Encode m4a or m4b
				if err := EncodeFFmpegAudio(ctx, atom, &atom.Episodes[i], format); err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid or unsupported format %q", format)
			}
		}

		// Add episode.Output to e.encodedOutputs
		e.encodedOutputs = append(e.encodedOutputs, atom.Episodes[i].Output)

		// If one or more uploader is given, upload output mp4/mp3/m4a/m4b
		// using the ForUploading port (interface).
		for _, uploader := range uploaders {
			if err := uploader.Upload(ctx, &ports.ForUploadingRequest{
				Store:        atom.Config.Aws.Buckets.Output,
				To:           atom.Episodes[i].Output,
				StorageClass: e.storageClass,
			}); err != nil {
				return err
			}
		}

	}

}

// EncodeMP4 encodes episode into an mp4 video (using ffmpeg)
func EncodeMP4(ctx context.Context, atom *model.Atom, episode *model.Episode) error {
	l := logger.FromContext(ctx)
	if atom == nil || episode == nil {
		return ErrNilPointer
	}
	tmpl, err := template.New("ffmpeg").Funcs(defaultFuncMap()).Parse(ffmpegCommandTemplate)
	if err != nil {
		return err
	}
	episode.Output = ExtensionToBaseMp4(episode.Input)
	values := templateValues{
		Atom:    atom,
		Episode: episode,
	}
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, values); err != nil {
		return err
	}
	l.Info("Executing encoder", "command", buf.String())
	cmd := exec.CommandContext(ctx, shell, shellCommandOption, buf.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to encode %q: %w", episode.Input, err)
	}
	// Update atom with the length and duration of the encoded mp4
	size, duration, err := Mp4Duration(path.Join(atom.LocalStorageDirExpanded(), episode.Output))
	if err != nil {
		return err
	}
	l.Info(fmt.Sprintf("%s is %s long and %d bytes", episode.Output, duration, size), "output", episode.Output, "duration", duration, "size", size)
	episode.Length = size
	episode.Duration.Duration = duration
	return nil
}

// EncodeFFmpegAudio encodes episode.Input into an m4a or m4b file
// depending on the value of format.
func EncodeFFmpegAudio(ctx context.Context, atom *model.Atom, episode *model.Episode, format string) error {
	l := logger.FromContext(ctx)
	if atom == nil || episode == nil {
		return ErrNilPointer
	}
	tmpl, err := template.New("ffmpeg").Funcs(defaultFuncMap()).Parse(ffmpegToM4ACommandTemplate)
	if err != nil {
		return err
	}

	format = strings.TrimSpace(strings.ToLower(format))
	episode.Output = ExtensionToBaseFormat(episode.Input, format)
	values := templateValues{
		Atom:    atom,
		Episode: episode,
	}
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
	metadataFile, err := id3v24.WriteFFmpegMetadataFile(duration, trackInfo)
	if err != nil {
		return fmt.Errorf("unable to generate ffmetadata file: %w", err)
	}

	values.MetadataFile = metadataFile

	// Parse template (with metadatafile added to input values)
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, values); err != nil {
		return err
	}

	l.Info("Executing encoder", "command", buf.String(), "input", episode.Input, "output", episode.Output)
	cmd := exec.CommandContext(ctx, shell, shellCommandOption, buf.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to encode %q: %w", episode.Input, err)
	}

	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	// Get correct duration and size of the output file
	duration, size, err = GetSizeAndDurationViaFFprobe(outputPath)
	if err != nil {
		return fmt.Errorf("unable to get duration and size from %s: %w", episode.Output, err)
	}

	// Update episode length and duration
	l.Info(fmt.Sprintf("%s is %s long and %d bytes", episode.Output, duration, size), "output", episode.Output, "duration", duration, "size", size)
	episode.Length = size
	episode.Duration.Duration = duration
	return nil
}

// EncodeMP3ViaFFmpeg encodes episode.Input through ffmpeg piped into
// lame as an mp3.
func EncodeMP3ViaFFmpeg(ctx context.Context, atom *model.Atom, episode *model.Episode) error {
	l := logger.FromContext(ctx)
	if atom == nil || episode == nil {
		return ErrNilPointer
	}
	tmpl, err := template.New("ffmpegToLame").Funcs(defaultFuncMap()).Parse(ffmpegToAudioCommandTemplate)
	if err != nil {
		return err
	}

	episode.Output = ExtensionToBaseMp3(episode.Input)
	values := templateValues{
		Atom:    atom,
		Episode: episode,
	}

	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, values); err != nil {
		return err
	}

	l.Info("Executing encoder", "command", buf.String(), "input", episode.Input, "output", episode.Output)
	cmd := exec.CommandContext(ctx, shell, shellCommandOption, buf.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
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
	// Update atom with the length and duration of the encoded mp3.
	l.Info(fmt.Sprintf("%s is %s long and %d bytes", episode.Output, di.Duration, di.Length), "output", episode.Output, "duration", di.Duration, "size", di.Length)
	episode.Length = di.Length
	episode.Duration.Duration = di.TimeDuration
	return nil
}

// EncodeMP3 encodes an mp3 using lame.
func EncodeMP3(ctx context.Context, atom *model.Atom, episode *model.Episode) error {
	l := logger.FromContext(ctx)
	if atom == nil || episode == nil {
		return ErrNilPointer
	}
	tmpl, err := template.New("ffmpegToLame").Funcs(defaultFuncMap()).Parse(lameCommandTemplate)
	if err != nil {
		return err
	}
	episode.Output = ExtensionToBaseMp3(episode.Input)
	values := templateValues{
		Atom:    atom,
		Episode: episode,
	}
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, values); err != nil {
		return err
	}
	l.Info("Executing encoder", "command", buf.String(), "input", episode.Input, "output", episode.Output)
	cmd := exec.CommandContext(ctx, shell, shellCommandOption, buf.String())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
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
	// Update atom with the length and duration of the encoded mp3.
	l.Info(fmt.Sprintf("%s is %s long and %d bytes", episode.Output, di.Duration, di.Length), "output", episode.Output, "duration", di.Duration, "size", di.Length)
	episode.Length = di.Length
	episode.Duration.Duration = di.TimeDuration
	return nil
}

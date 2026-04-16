package encoder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func buildMP4Args(atom *model.Podcast, episode *model.Episode) []string {
	inputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Input)
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	return []string{
		"-y",
		"-i", inputPath,
		"-pix_fmt", "yuv420p",
		"-colorspace", "bt709",
		"-color_trc", "bt709",
		"-color_primaries", "bt709",
		"-color_range", "tv",
		"-c:v", "libx264",
		"-profile:v", "high",
		"-crf", fmt.Sprintf("%d", atom.Encoding.CRF),
		"-maxrate", "1M",
		"-bufsize", "2M",
		"-preset", "medium",
		"-coder", "1",
		"-movflags", "+faststart",
		"-x264-params", "open-gop=0",
		"-c:a", "aac",
		"-b:a", atom.Encoding.ABR,
		outputPath,
	}
}

func buildFFmpegAudioArgs(atom *model.Podcast, episode *model.Episode, metadataFile string) []string {
	lang := atom.Encoding.Language
	if episode.EncodingLanguage != "" {
		lang = episode.EncodingLanguage
	}
	inputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Input)
	coverPath := path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront)
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	return []string{
		"-y",
		"-i", inputPath,
		"-i", coverPath,
		"-i", metadataFile,
		"-map", "0:a",
		"-c:a", "aac",
		"-b:a", atom.Encoding.ABR,
		"-metadata:s:a:0", "language=" + strings.ToLower(lang),
		"-map", "1:v",
		"-c:v", "mjpeg",
		"-disposition:v:0", "attached_pic",
		"-metadata:s:v", "title=Cover",
		"-metadata:s:v", "comment=Cover (front)",
		"-map_metadata", "2",
		"-map_chapters", "2",
		"-movflags", "faststart",
		outputPath,
	}
}

func buildFFmpegToWavArgs(atom *model.Podcast, episode *model.Episode) []string {
	inputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Input)
	return []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-f", "wav",
		"-c:a", "pcm_s16le",
		"-ac", "2",
		"pipe:1",
	}
}

func buildLameArgs(atom *model.Podcast, episode *model.Episode) []string {
	inputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Input)
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	return []string{
		"-b", fmt.Sprintf("%d", atom.Encoding.Bitrate),
		inputPath,
		outputPath,
	}
}

func buildLamePipeArgs(atom *model.Podcast, episode *model.Episode) []string {
	lang := atom.Encoding.Language
	if episode.EncodingLanguage != "" {
		lang = episode.EncodingLanguage
	}
	outputPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
	coverPath := path.Join(atom.LocalStorageDirExpanded(), atom.Encoding.Coverfront)
	return []string{
		"-b", fmt.Sprintf("%d", atom.Encoding.Bitrate),
		"--add-id3v2",
		"--tv", "TLAN=" + lang,
		"--tt", episode.Title,
		"--ta", atom.Author,
		"--tl", atom.Title,
		"--ty", episode.PubDate.Format("2006"),
		"--tc", episode.Subtitle,
		"--tn", fmt.Sprintf("%d", episode.UID),
		"--tg", atom.Encoding.Genre,
		"--ti", coverPath,
		"--tv", "WOAR=" + atom.Link,
		"-",
		outputPath,
	}
}

func runCommand(ctx context.Context, command string, args []string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCommandOutput(command string, args []string) ([]byte, error) {
	cmd := exec.Command(command, args...)
	return cmd.Output()
}

func runPipeline(ctx context.Context, producerCommand string, producerArgs []string, consumerCommand string, consumerArgs []string) error {
	producer := exec.CommandContext(ctx, producerCommand, producerArgs...)
	consumer := exec.CommandContext(ctx, consumerCommand, consumerArgs...)

	reader, err := producer.StdoutPipe()
	if err != nil {
		return err
	}
	producer.Stdin = os.Stdin
	producer.Stderr = os.Stderr

	consumer.Stdin = reader
	consumer.Stdout = os.Stdout
	consumer.Stderr = os.Stderr

	if err := producer.Start(); err != nil {
		return err
	}
	if err := consumer.Start(); err != nil {
		_ = producer.Process.Kill()
		_ = producer.Wait()
		return err
	}

	consumerErr := consumer.Wait()
	producerErr := producer.Wait()
	if producerErr != nil {
		return producerErr
	}
	if consumerErr != nil {
		return consumerErr
	}
	return nil
}

package encoder

import (
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/media"
)

func TestEncodeMP3ProducesOutput(t *testing.T) {
	requireTools(t, "lame")

	workdir := t.TempDir()
	coverPath := filepath.Join(workdir, "artwork", "cover.jpg")
	inputPath := filepath.Join(workdir, "masters", "episode.wav")
	writeCoverJPEG(t, coverPath)
	writeSilentWAV(t, inputPath, 44100, 1)

	atom := testAtom(workdir)
	episode := &model.Episode{
		UID:      1,
		Title:    "Episode One",
		Author:   "Host",
		PubDate:  model.ItunesTime{Time: time.Date(2022, 3, 25, 16, 0, 13, 0, time.UTC)},
		Link:     "https://example.com/show/episodes/1",
		Subtitle: "Subtitle",
		Input:    filepath.ToSlash(filepath.Join("masters", "episode.wav")),
		Image:    filepath.ToSlash(filepath.Join("artwork", "cover.jpg")),
		Chapters: nil,
		Format:   "mp3",
		Duration: model.ItunesDuration{},
		Explicit: model.ItunesExplicit{S: "no"},
		Output:   "",
		Type:     "",
		Length:   0,
	}

	if err := EncodeMP3(context.Background(), atom, episode); err != nil {
		t.Fatalf("EncodeMP3() error = %v", err)
	}

	outputPath := filepath.Join(workdir, episode.Output)
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file %s: %v", outputPath, err)
	}
	if filepath.Ext(outputPath) != ".mp3" {
		t.Fatalf("output extension = %q, want .mp3", filepath.Ext(outputPath))
	}
	if episode.Length <= 0 {
		t.Fatalf("Length = %d, want > 0", episode.Length)
	}
	if episode.Duration.Duration <= 0 {
		t.Fatalf("Duration = %s, want > 0", episode.Duration.Duration)
	}
	if episode.Type != "audio/mpeg" {
		t.Fatalf("Type = %q, want audio/mpeg", episode.Type)
	}
}

func TestEncodeFFmpegAudioProducesOutput(t *testing.T) {
	requireTools(t, "ffmpeg", "ffprobe")

	workdir := t.TempDir()
	coverPath := filepath.Join(workdir, "artwork", "cover.jpg")
	inputPath := filepath.Join(workdir, "masters", "episode.wav")
	writeCoverJPEG(t, coverPath)
	writeSilentWAV(t, inputPath, 44100, 1)

	atom := testAtom(workdir)
	episode := &model.Episode{
		UID:      1,
		Title:    "Episode One",
		Author:   "Host",
		PubDate:  model.ItunesTime{Time: time.Date(2022, 3, 25, 16, 0, 13, 0, time.UTC)},
		Link:     "https://example.com/show/episodes/1",
		Subtitle: "Subtitle",
		Input:    filepath.ToSlash(filepath.Join("masters", "episode.wav")),
		Image:    filepath.ToSlash(filepath.Join("artwork", "cover.jpg")),
		Format:   "m4a",
		Explicit: model.ItunesExplicit{S: "no"},
	}

	if err := EncodeFFmpegAudio(context.Background(), atom, episode, "m4a"); err != nil {
		t.Fatalf("EncodeFFmpegAudio() error = %v", err)
	}

	outputPath := filepath.Join(workdir, episode.Output)
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file %s: %v", outputPath, err)
	}
	if filepath.Ext(outputPath) != ".m4a" {
		t.Fatalf("output extension = %q, want .m4a", filepath.Ext(outputPath))
	}
	if episode.Length <= 0 {
		t.Fatalf("Length = %d, want > 0", episode.Length)
	}
	if episode.Duration.Duration <= 0 {
		t.Fatalf("Duration = %s, want > 0", episode.Duration.Duration)
	}
	if !strings.HasPrefix(episode.Type, "audio/") {
		t.Fatalf("Type = %q, want audio/*", episode.Type)
	}
}

func requireTools(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if err := media.EnsureToolAvailable(tool); err != nil {
			t.Skipf("%s unavailable: %v", tool, err)
		}
	}
}

func testAtom(workdir string) *model.Atom {
	atom := &model.Atom{
		Config: model.Config{
			LocalStorageDir: workdir,
			DefaultPodImage: filepath.ToSlash(filepath.Join("artwork", "cover.jpg")),
		},
		Title: "Example Show",
	}
	atom.Encoding.Lamepath = "lame"
	atom.Encoding.FFmpegPath = "ffmpeg"
	atom.Encoding.Coverfront = filepath.ToSlash(filepath.Join("artwork", "cover.jpg"))
	atom.Encoding.Genre = "Podcast"
	atom.Encoding.Language = "eng"
	atom.Encoding.ABR = "128k"
	return atom
}

func writeSilentWAV(t *testing.T, filename string, sampleRate uint32, seconds uint32) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(filename), err)
	}

	const numChannels uint16 = 1
	const bitsPerSample uint16 = 16
	bytesPerSample := uint32(bitsPerSample / 8)
	numSamples := sampleRate * seconds
	dataSize := numSamples * uint32(numChannels) * bytesPerSample
	byteRate := sampleRate * uint32(numChannels) * bytesPerSample
	blockAlign := numChannels * bitsPerSample / 8
	riffSize := uint32(36) + dataSize

	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Create(%q): %v", filename, err)
	}
	defer file.Close()

	write := func(data any) {
		if err := binary.Write(file, binary.LittleEndian, data); err != nil {
			t.Fatalf("binary.Write(%q): %v", filename, err)
		}
	}

	if _, err := file.Write([]byte("RIFF")); err != nil {
		t.Fatalf("Write RIFF: %v", err)
	}
	write(riffSize)
	if _, err := file.Write([]byte("WAVEfmt ")); err != nil {
		t.Fatalf("Write WAVEfmt: %v", err)
	}
	write(uint32(16))
	write(uint16(1))
	write(numChannels)
	write(sampleRate)
	write(byteRate)
	write(blockAlign)
	write(bitsPerSample)
	if _, err := file.Write([]byte("data")); err != nil {
		t.Fatalf("Write data: %v", err)
	}
	write(dataSize)
	silence := make([]byte, dataSize)
	if _, err := file.Write(silence); err != nil {
		t.Fatalf("Write silence: %v", err)
	}
}

func writeCoverJPEG(t *testing.T, filename string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(filename), err)
	}

	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Create(%q): %v", filename, err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0x22, G: 0x66, B: 0xaa, A: 0xff})
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode(%q): %v", filename, err)
	}
}

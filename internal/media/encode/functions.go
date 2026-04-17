package encode

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/alfg/mp4"
	"github.com/gabriel-vasile/mimetype"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/media"
)

func GetFileContentType(filename string) (contentType string, err error) {
	mimetype.SetLimit(1024 * 1024)
	mimeType, err := mimetype.DetectFile(filename)
	if err != nil {
		return "", err
	}
	return mimeType.String(), nil
}

// Replaces or adds file extension.
func ReplaceExtension(filename string, newExtension string) (newFilename string) {
	ext := filepath.Ext(filename)
	newFilename = filename[0:len(filename)-len(ext)] + newExtension
	return
}

// Returns basename of filename with extension replaced with .mp3
func ExtensionToBaseMp3(filename string) string {
	baseName := path.Base(filename)
	return ReplaceExtension(baseName, ".mp3")
}

func ExtensionToBaseMp4(filename string) string {
	baseName := path.Base(filename)
	return ReplaceExtension(baseName, ".mp4")
}

// This can be insecure as format is used unescaped and is only used
// where format is known.
func ExtensionToBaseFormat(filename string, format string) string {
	baseName := path.Base(filename)
	return ReplaceExtension(baseName, "."+strings.TrimSpace(strings.ToLower(format)))
}

// Mp4Duration returns the length in bytes and the duration in
// time.Duration.
func Mp4Duration(filename string) (int64, time.Duration, error) {
	f, err := os.Open(filename)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, 0, err
	}
	mp4, err := mp4.OpenFromReader(f, info.Size())
	if err != nil {
		return 0, 0, err
	}
	if mp4 != nil && mp4.Moov != nil && mp4.Moov.Mvhd != nil {
		return info.Size(), time.Duration(mp4.Moov.Mvhd.Duration) * time.Millisecond, nil
	}
	return 0, 0, fmt.Errorf("%s does not contain a Moov Mvhd box (maybe not an mp4?)", filename)
}

// FFprobe runs ffprobe on filename and returns an FFprobeJSON with
// format filled in or returns error if something failed.
func FFprobe(filename string) (*model.FFprobeJSON, error) {
	if err := media.EnsureToolAvailable("ffprobe"); err != nil {
		return nil, err
	}
	out, err := runCommandOutput("ffprobe", []string{"-v", "error", "-show_format", "-print_format", "json", filename})
	if err != nil {
		return nil, err
	}
	var result model.FFprobeJSON
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSizeAndDurationViaFFprobe returns duration, size or error if
// something failed.
func GetSizeAndDurationViaFFprobe(filename string) (time.Duration, int64, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return 0, 0, err
	}
	ffprobejson, err := FFprobe(filename)
	if err != nil {
		return 0, fi.Size(), err
	}
	return ffprobejson.Format.Duration.Duration, fi.Size(), nil
}

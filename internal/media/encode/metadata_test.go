package encode

import (
	"os"
	"testing"
	"time"

	"github.com/sa6mwa/id3v24"
)

func TestWriteFFmpegMetadataFileReturnsCleanup(t *testing.T) {
	metadataFile, cleanup, err := writeFFmpegMetadataFile(30*time.Second, id3v24.TrackInfo{
		Title:  "Episode",
		Artist: "Host",
		Date:   time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("writeFFmpegMetadataFile() error = %v", err)
	}

	if _, err := os.Stat(metadataFile); err != nil {
		t.Fatalf("expected metadata file to exist before cleanup: %v", err)
	}

	cleanup()

	if _, err := os.Stat(metadataFile); !os.IsNotExist(err) {
		t.Fatalf("expected metadata file to be removed, got err = %v", err)
	}
}

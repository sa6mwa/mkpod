package encode

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mp3duration"
)

type PrepareEncodeFunc func(context.Context, *model.Podcast, *model.Episode) error

func outputMetadataNeedsRepair(episode *model.Episode) bool {
	if episode == nil {
		return false
	}
	return strings.TrimSpace(episode.Type) == "" || episode.Length <= 0 || episode.Duration.Duration <= 0
}

func repairOutputMetadata(_ context.Context, episode *model.Episode, outputPath string) (bool, error) {
	if episode == nil || strings.TrimSpace(outputPath) == "" {
		return false, nil
	}
	if _, err := os.Stat(outputPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to access local encoded output %s: %w", outputPath, err)
	}
	if !outputMetadataNeedsRepair(episode) {
		return false, nil
	}

	changed := false
	contentType := strings.TrimSpace(episode.Type)
	if contentType == "" {
		detectedType, err := GetFileContentType(outputPath)
		if err != nil {
			return false, err
		}
		episode.Type = detectedType
		contentType = detectedType
		changed = true
	}

	if strings.EqualFold(contentType, "audio/mpeg") {
		di, err := mp3duration.ReadFile(outputPath)
		if err != nil {
			return false, err
		}
		if episode.Length <= 0 {
			episode.Length = di.Length
			changed = true
		}
		if episode.Duration.Duration <= 0 {
			episode.Duration.Duration = di.TimeDuration
			changed = true
		}
		return changed, nil
	}

	duration, size, err := GetSizeAndDurationViaFFprobe(outputPath)
	if err != nil {
		return false, err
	}
	if episode.Length <= 0 {
		episode.Length = size
		changed = true
	}
	if episode.Duration.Duration <= 0 {
		episode.Duration.Duration = duration
		changed = true
	}
	return changed, nil
}

func ensureEpisodePubDate(episode *model.Episode) bool {
	if episode == nil || !episode.PubDate.IsZero() {
		return false
	}
	episode.PubDate.Time = time.Now().UTC()
	return true
}

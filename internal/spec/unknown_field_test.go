package spec

import (
	"errors"
	"testing"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

func TestValidateRejectsUnknownRequiredField(t *testing.T) {
	original := append([]string(nil), RequiredTopLevelFields...)
	RequiredTopLevelFields = append(RequiredTopLevelFields, "definitely.unknown")
	defer func() { RequiredTopLevelFields = original }()

	err := Validate(&model.Podcast{
		Author:      "Host",
		FeedFile:    "podcast.rss",
		Title:       "Show",
		TTL:         60,
		Language:    "en",
		Copyright:   "Copyright",
		WebMaster:   "wm@example.com",
		Description: "Description",
		Subtitle:    "Subtitle",
		OwnerName:   "Owner",
		OwnerEmail:  "owner@example.com",
		Config: model.Config{
			BaseURL:         "https://example.com",
			Image:           "https://example.com/cover.jpg",
			DefaultPodImage: "artwork/cover.jpg",
		},
	})
	if !errors.Is(err, ErrUnknownRequiredField) {
		t.Fatalf("Validate() error = %v, want %v", err, ErrUnknownRequiredField)
	}
}

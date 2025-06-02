package ports

import (
	"context"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

// ForParsing should produce a podcast RSS feed from the model.
type ForParsing interface {
	WriteRSS(ctx context.Context, atom *model.Atom) error
	WriteRSSToStdout(ctx context.Context, atom *model.Atom) error
}

package ports

import (
	"context"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

type ForEncoding interface {
	// Encode encodes episode with UID uid. If uid is -1, all episodes
	// in the Episodes slice of atom will be encoded.
	Encode(ctx context.Context, atom *model.Atom, uid int64, force bool) error
}

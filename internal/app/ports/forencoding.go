package ports

import (
	"context"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

type ForEncoding interface {
	ForAsking
	// Encode encodes episode with UID uid. If uid is -1, all episodes
	// in the Episodes slice of atom should be encoded. The postEncoding
	// function is optional and can be disabled by issuing a nil
	// value. The function is called after each episode has been encoded
	// and could be used to for example upload the encoded episode
	// (episode.Output) to final storage (using e.g
	// ports.ForUploading). If the postEncoding function returns an
	// error, the entire encoding loop is cancelled and the error is
	// returned by Encode.
	Encode(ctx context.Context, atom *model.Atom, uid int64, postEncoding PostEncodeFunc) error
	// The Encode method adds the path to each encoded output
	// (episode.Output) to a string slice in the interface instance,
	// GetEncodedOutputs returns that slice.
	GetEncodedOutputs() []string
}

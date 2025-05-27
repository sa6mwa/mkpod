package ports

import (
	"context"

	"github.com/sa6mwa/mkpod/internal/app/model"
)

type ForConfiguring interface {
	Load(ctx context.Context) (*model.Atom, error)
	Save(ctx context.Context, atom *model.Atom) error
	Set(ctx context.Context, property string, value any) error
	Get(ctx context.Context, property string) (any, error)
}

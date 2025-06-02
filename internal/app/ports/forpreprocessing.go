package ports

import (
	"context"
)

type ForPreprocessing interface {
	Process(ctx context.Context, mediaFilePaths []string) error
	SetPrefix(prefix string) ForPreprocessing
	SetPreset(preset string) ForPreprocessing
	SetTool(toolPath string) ForPreprocessing
}

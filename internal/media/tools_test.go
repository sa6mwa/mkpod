package media

import (
	"errors"
	"testing"
)

func TestEnsureToolAvailableFailsForMissingTool(t *testing.T) {
	err := EnsureToolAvailable("mkpod-definitely-missing-tool")
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("EnsureToolAvailable() error = %v, want ErrToolNotFound", err)
	}
}

package media

import (
	"errors"
	"strings"
	"testing"
)

func TestEnsureToolAvailableFailsForMissingTool(t *testing.T) {
	err := EnsureToolAvailable("mkpod-definitely-missing-tool")
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("EnsureToolAvailable() error = %v, want ErrToolNotFound", err)
	}
	if !strings.Contains(err.Error(), "install it or configure an explicit tool path") {
		t.Fatalf("EnsureToolAvailable() error = %q, want install hint", err)
	}
}

package media

import (
	"errors"
	"fmt"
	"os/exec"
)

var ErrToolNotFound = errors.New("required tool not found")

func EnsureToolAvailable(tool string) error {
	if _, err := exec.LookPath(tool); err != nil {
		return fmt.Errorf("%w: %s (install it or configure an explicit tool path)", ErrToolNotFound, tool)
	}
	return nil
}

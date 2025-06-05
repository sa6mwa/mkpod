// asker implements the ports.ForAsking interface.
package asker

import (
	"context"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
	"golang.org/x/term"
)

type forAsking struct {
	dryrun bool
	force  bool
}

func New(dryrun, force bool) ports.ForAsking {
	return &forAsking{
		dryrun: dryrun,
		force:  force,
	}
}

func (p *forAsking) Ask(ctx context.Context, format string, a ...any) bool {
	l := logger.FromContext(ctx)
	if p.dryrun {
		l.Info(fmt.Sprintf("%s No", fmt.Sprintf(format, a...)))
		return true
	}
	if p.force {
		l.Info(fmt.Sprintf("%s Yes", fmt.Sprintf(format, a...)))
		return true
	}
	return p.yes(ctx, format, a...)
}

func (p *forAsking) yes(ctx context.Context, format string, a ...any) bool {
	l := logger.FromContext(ctx)
	if !p.isTerminal() {
		l.Warn("Stdout is not a terminal, will answer no", "question", fmt.Sprintf(format, a...))
		return false
	}
	choice := ""
	prompt := &survey.Select{
		Message: fmt.Sprintf(format, a...),
		Options: []string{"No", "Yes", "Exit program"},
		Default: "Yes",
	}
	survey.AskOne(prompt, &choice)
	switch choice {
	case "", "No":
		return false
	case "Yes":
		return true
	case "Exit program":
		l.Warn("Exiting")
		os.Exit(0)
	}
	return false
}

func (p *forAsking) isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

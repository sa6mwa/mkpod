package ports

import "context"

type ForAsking interface {
	// For asking questions in a terminal (or always return no or yes
	// based on other inputs such as dry-run or force flags). Should
	// return false if "no" and true if "yes". Should support exiting
	// the program by some mechanism as well (usually simply os.Exit(0)
	// if choosing "exit program" or something). ctx should/could hold a
	// slog.Logger set with logger package using logger.WithLogger or
	// logger.WithDefaultLogger.
	Ask(ctx context.Context, format string, a ...any) bool
}

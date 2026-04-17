// logger is an slog.Logger adapter to store an slog.Logger (using
// logger.WithLogger) into a context.Context and later retrieve it
// (using logger.FromContext). The default logger is
// github.com/charmbracelet/log.
package logger

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

type contextKey struct{}

var loggerKey = &contextKey{}

// WithLogger returns a context with l as slog.Logger based off the
// ctx context. Retrieve the logger using FromContext.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// WithDefaultLogger returns a context with DefaultLogger set as the
// slog.Logger based off the ctx context. Retrieve the logger using
// FromContext.
func WithDefaultLogger(ctx context.Context) context.Context {
	return WithLogger(ctx, DefaultLogger())
}

// FromContext retrieves an slog.Logger saved by WithLogger from
// ctx. If there is not such logger in the context,
// logger.DefaultLogger() is returned ensuring this function will
// always return a valid slog.Logger.
func FromContext(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok {
		return DefaultLogger()
	}
	return l
}

// DefaultLogger returns the default logger for this adapter package
// which utilizes github.com/charmbracelet/log.
func DefaultLogger() *slog.Logger {
	return slog.New(log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		//TimeFormat:      time.RFC3339,
		TimeFormat: time.Kitchen,
	}))
}

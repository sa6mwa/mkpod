// Package logging stores a pslog.Logger in a context.Context and later
// retrieves it. The default logger uses pkt.systems/pslog.
package logger

import (
	"context"
	"os"
	"time"

	"pkt.systems/pslog"
)

type contextKey struct{}

var loggerKey = &contextKey{}

// WithLogger returns a context with l as pslog.Logger based off the
// ctx context. Retrieve the logger using FromContext.
func WithLogger(ctx context.Context, l pslog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// WithDefaultLogger returns a context with DefaultLogger set as the
// pslog.Logger based off the ctx context. Retrieve the logger using
// FromContext.
func WithDefaultLogger(ctx context.Context) context.Context {
	return WithLogger(ctx, DefaultLogger())
}

// FromContext retrieves a pslog.Logger saved by WithLogger from
// ctx. If there is not such logger in the context,
// logger.DefaultLogger() is returned ensuring this function will
// always return a valid pslog.Logger.
func FromContext(ctx context.Context) pslog.Logger {
	l, ok := ctx.Value(loggerKey).(pslog.Logger)
	if !ok {
		return DefaultLogger()
	}
	return l
}

// DefaultLogger returns the default logger for this logging package
// which utilizes pkt.systems/pslog.
func DefaultLogger() pslog.Logger {
	return pslog.LoggerFromEnv(context.Background(),
		pslog.WithEnvWriter(os.Stderr),
		pslog.WithEnvOptions(pslog.Options{
			Mode:       pslog.ModeConsole,
			TimeFormat: time.Kitchen,
		}),
	)
}

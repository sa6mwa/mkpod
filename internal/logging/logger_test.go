package logger

import (
	"context"
	"testing"

	"pkt.systems/pslog"
)

func TestFromContextReturnsDefaultLogger(t *testing.T) {
	if FromContext(context.Background()) == nil {
		t.Fatal("FromContext returned nil logger")
	}
}

func TestWithLoggerStoresPslogLogger(t *testing.T) {
	want := pslog.NoopLogger()
	ctx := WithLogger(context.Background(), want)
	if got := FromContext(ctx); got != want {
		t.Fatalf("FromContext() = %T %p, want %T %p", got, got, want, want)
	}
}

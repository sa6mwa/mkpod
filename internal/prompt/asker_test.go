package prompt

import (
	"context"
	"testing"
)

func TestAskDryRunReturnsNo(t *testing.T) {
	p := New(true, false)
	if p.Ask(context.Background(), "rewrite file?") {
		t.Fatal("dry-run prompt should answer no")
	}
}

func TestAskForceReturnsYes(t *testing.T) {
	p := New(false, true)
	if !p.Ask(context.Background(), "rewrite file?") {
		t.Fatal("force prompt should answer yes")
	}
}

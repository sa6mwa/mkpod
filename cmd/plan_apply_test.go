package cmd

import (
	"context"
	"errors"
	"testing"
)

func TestPlannedContentType(t *testing.T) {
	tests := map[string]string{
		"episode.mp3": "audio/mpeg",
		"episode.m4a": "audio/mp4",
		"episode.m4b": "audio/mp4",
		"episode.mp4": "video/mp4",
		"episode.wav": "",
	}
	for filename, want := range tests {
		if got := plannedContentType(filename); got != want {
			t.Fatalf("plannedContentType(%q) = %q, want %q", filename, got, want)
		}
	}
}

type fakeRemoteChecker struct {
	exists bool
	err    error
}

func (f fakeRemoteChecker) FileExists(context.Context, string, string) (bool, error) {
	return f.exists, f.err
}

func TestFillRemoteObjectPlan(t *testing.T) {
	plan := remoteObjectPlan{Bucket: "bucket", Key: "episode.m4a"}
	fillRemoteObjectPlan(context.Background(), fakeRemoteChecker{exists: true}, &plan)
	if plan.Exists != "true" || plan.Error != "" {
		t.Fatalf("remote plan = %+v, want exists true without error", plan)
	}

	plan = remoteObjectPlan{Bucket: "bucket", Key: "episode.m4a"}
	fillRemoteObjectPlan(context.Background(), fakeRemoteChecker{}, &plan)
	if plan.Exists != "false" || plan.Error != "" {
		t.Fatalf("remote plan = %+v, want exists false without error", plan)
	}

	plan = remoteObjectPlan{Bucket: "bucket", Key: "episode.m4a"}
	fillRemoteObjectPlan(context.Background(), fakeRemoteChecker{err: errors.New("boom")}, &plan)
	if plan.Exists != "unknown" || plan.Error != "boom" {
		t.Fatalf("remote plan = %+v, want unknown with error", plan)
	}
}

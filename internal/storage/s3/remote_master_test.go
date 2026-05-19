package s3store

import "testing"

func TestEvaluateRemoteMasterRemoval(t *testing.T) {
	tests := []struct {
		name         string
		localExists  bool
		localSize    int64
		remoteExists bool
		remoteSize   int64
		wantAllowed  bool
		wantReason   string
		wantMin      int64
	}{
		{name: "local missing", localExists: false, remoteExists: true, remoteSize: 100, wantReason: RemovalLocalMissing},
		{name: "remote missing", localExists: true, localSize: 100, remoteExists: false, wantReason: RemovalRemoteMissing},
		{name: "local too small", localExists: true, localSize: 49, remoteExists: true, remoteSize: 100, wantReason: RemovalLocalTooSmall, wantMin: 50},
		{name: "local meets threshold", localExists: true, localSize: 50, remoteExists: true, remoteSize: 100, wantAllowed: true, wantReason: RemovalAllowed, wantMin: 50},
		{name: "local exceeds threshold", localExists: true, localSize: 1000, remoteExists: true, remoteSize: 100, wantAllowed: true, wantReason: RemovalAllowed, wantMin: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateRemoteMasterRemoval(tt.localExists, tt.localSize, tt.remoteExists, tt.remoteSize)
			if got.Allowed != tt.wantAllowed {
				t.Fatalf("Allowed = %v, want %v", got.Allowed, tt.wantAllowed)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.wantReason)
			}
			if got.MinRequiredSize != tt.wantMin {
				t.Fatalf("MinRequiredSize = %d, want %d", got.MinRequiredSize, tt.wantMin)
			}
		})
	}
}

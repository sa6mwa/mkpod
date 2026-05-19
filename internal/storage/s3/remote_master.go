package s3store

type RemoteMasterRemovalDecision struct {
	Allowed         bool
	Reason          string
	LocalSize       int64
	RemoteSize      int64
	MinRequiredSize int64
}

const (
	RemovalAllowed       = "allowed"
	RemovalLocalMissing  = "local_missing"
	RemovalRemoteMissing = "remote_missing"
	RemovalLocalTooSmall = "local_too_small"
)

func EvaluateRemoteMasterRemoval(localExists bool, localSize int64, remoteExists bool, remoteSize int64) RemoteMasterRemovalDecision {
	decision := RemoteMasterRemovalDecision{
		LocalSize:  localSize,
		RemoteSize: remoteSize,
	}
	if !localExists {
		decision.Reason = RemovalLocalMissing
		return decision
	}
	if !remoteExists {
		decision.Reason = RemovalRemoteMissing
		return decision
	}
	decision.MinRequiredSize = remoteSize / 2
	if localSize < decision.MinRequiredSize {
		decision.Reason = RemovalLocalTooSmall
		return decision
	}
	decision.Allowed = true
	decision.Reason = RemovalAllowed
	return decision
}

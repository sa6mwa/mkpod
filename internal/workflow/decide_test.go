package workflow

import "testing"

func TestDecideEpisodeJustMasterUploadsMissingRemoteMaster(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master: ObjectState{
			Kind:      ObjectMaster,
			Label:     "episode master",
			Bucket:    "input",
			Key:       "masters/episode.flac",
			LocalPath: "/pod/masters/episode.flac",
			Local:     FileState{Exists: true, Size: 100},
			Remote:    RemoteState{Checked: true, Exists: false},
		},
	}, Options{Mode: ModeJustMaster})

	if decision.State != StateMasterSynced {
		t.Fatalf("State = %q, want %q", decision.State, StateMasterSynced)
	}
	operation := requireOperation(t, decision, OperationUploadMaster)
	if !operation.RequiresPrompt {
		t.Fatal("upload master RequiresPrompt = false, want true")
	}
	if operation.Bucket != "input" || operation.Key != "masters/episode.flac" {
		t.Fatalf("upload operation = %+v, want input/masters/episode.flac", operation)
	}
	if hasOperation(decision, OperationEncode) {
		t.Fatalf("just-master decision included encode operation: %+v", decision.Operations)
	}
	if hasOperation(decision, OperationRegenerateRSS) {
		t.Fatalf("just-master decision included RSS operation: %+v", decision.Operations)
	}
}

func TestDecideEpisodeJustMasterSyncsEncodeArtifacts(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		Artifacts: []ObjectState{
			{
				Kind:      ObjectEncodeArtifact,
				Label:     "episode image",
				Bucket:    "input",
				Key:       "artwork/episode.jpg",
				LocalPath: "/pod/artwork/episode.jpg",
				Local:     FileState{Exists: true, Size: 50},
				Remote:    RemoteState{Checked: true, Exists: false},
			},
		},
	}, Options{Mode: ModeJustMaster})

	if decision.State != StateMasterSynced {
		t.Fatalf("State = %q, want %q", decision.State, StateMasterSynced)
	}
	requireOperation(t, decision, OperationUploadArtifact)
	if hasOperation(decision, OperationEncode) {
		t.Fatalf("just-master decision included encode operation: %+v", decision.Operations)
	}
}

func TestDecideEpisodeForceRequiresLocalMaster(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master: ObjectState{
			Label:  "episode master",
			Bucket: "input",
			Key:    "masters/episode.flac",
			Remote: RemoteState{Checked: true, Exists: true, Size: 100},
		},
	}, Options{Mode: ModeJustMaster, Force: true})

	if decision.State != StateBlocked {
		t.Fatalf("State = %q, want %q", decision.State, StateBlocked)
	}
	if len(decision.Operations) != 0 {
		t.Fatalf("Operations = %+v, want none", decision.Operations)
	}
	check := requireCheck(t, decision, "master")
	if check.Passed {
		t.Fatalf("master check passed unexpectedly: %+v", check)
	}
	if check.Reason != "forced master sync requires local master" {
		t.Fatalf("master check reason = %q", check.Reason)
	}
}

func TestDecideEpisodeYesCanDownloadMissingLocalMaster(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master: ObjectState{
			Label:     "episode master",
			Bucket:    "input",
			Key:       "masters/episode.flac",
			LocalPath: "/pod/masters/episode.flac",
			Remote:    RemoteState{Checked: true, Exists: true, Size: 100},
		},
	}, Options{Mode: ModeJustMaster, Yes: true})

	if decision.State == StateBlocked {
		t.Fatalf("State = blocked, want downloadable master decision: %+v", decision)
	}
	operation := requireOperation(t, decision, OperationDownloadMaster)
	if operation.RequiresPrompt {
		t.Fatalf("download operation still requires prompt with --yes: %+v", operation)
	}
}

func TestDecideEpisodeRemoteProductionAudioWithCompleteMetadataDoesNotDownload(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		ProductionAudio: ObjectState{
			Label:  "episode output",
			Bucket: "output",
			Key:    "episode.m4a",
			Remote: RemoteState{Checked: true, Exists: true, Size: 200},
		},
		ProductionKnown: true,
	}, Options{})

	if decision.State != StateComplete {
		t.Fatalf("State = %q, want %q", decision.State, StateComplete)
	}
	if hasOperation(decision, OperationDownloadProduction) {
		t.Fatalf("decision downloaded production despite complete metadata: %+v", decision.Operations)
	}
	if hasOperation(decision, OperationEncode) {
		t.Fatalf("decision encoded despite complete remote production audio: %+v", decision.Operations)
	}
}

func TestDecideEpisodeMissingProductionMetadataDownloadsForRepairBeforeEncode(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		ProductionAudio: ObjectState{
			Label:     "episode output",
			Bucket:    "output",
			Key:       "episode.m4a",
			LocalPath: "/pod/episode.m4a",
			Remote:    RemoteState{Checked: true, Exists: true, Size: 200},
		},
		ProductionKnown: false,
	}, Options{})

	requireOperation(t, decision, OperationDownloadProduction)
	repair := requireOperation(t, decision, OperationRepairMetadata)
	assertProductionOperationDetails(t, repair, "output", "episode.m4a", "/pod/episode.m4a")
	if hasOperation(decision, OperationEncode) {
		t.Fatalf("decision encoded before trying production metadata repair: %+v", decision.Operations)
	}
}

func TestDecideEpisodeMissingProductionMetadataRepairsLocalOutputBeforeUpload(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		ProductionAudio: ObjectState{
			Label:     "episode output",
			Bucket:    "output",
			Key:       "episode.m4a",
			LocalPath: "/pod/episode.m4a",
			Local:     FileState{Exists: true, Size: 200, Checksum: "abc"},
			Remote:    RemoteState{Checked: true, Exists: false},
		},
		ProductionKnown: false,
	}, Options{})

	repair := requireOperation(t, decision, OperationRepairMetadata)
	upload := requireOperation(t, decision, OperationUploadProduction)
	assertProductionOperationDetails(t, repair, "output", "episode.m4a", "/pod/episode.m4a")
	if repair.Reason != "production audio metadata is incomplete" {
		t.Fatalf("repair reason = %q", repair.Reason)
	}
	if upload.Reason != "remote production audio is missing" {
		t.Fatalf("upload reason = %q", upload.Reason)
	}
	if operationIndex(decision, OperationRepairMetadata) > operationIndex(decision, OperationUploadProduction) {
		t.Fatalf("operations = %+v, want repair before upload", decision.Operations)
	}
	if hasOperation(decision, OperationEncode) {
		t.Fatalf("decision encoded instead of repairing local output: %+v", decision.Operations)
	}
}

func TestDecideEpisodeMissingProductionMetadataRepairsMatchingLocalRemoteOutput(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		ProductionAudio: ObjectState{
			Label:     "episode output",
			Bucket:    "output",
			Key:       "episode.m4a",
			LocalPath: "/pod/episode.m4a",
			Local:     FileState{Exists: true, Size: 200, Checksum: "abc"},
			Remote:    RemoteState{Checked: true, Exists: true, Size: 200, ETag: `"abc"`},
		},
		ProductionKnown: false,
	}, Options{})

	repair := requireOperation(t, decision, OperationRepairMetadata)
	assertProductionOperationDetails(t, repair, "output", "episode.m4a", "/pod/episode.m4a")
	if hasOperation(decision, OperationUploadProduction) {
		t.Fatalf("decision uploaded matching production audio: %+v", decision.Operations)
	}
	if hasOperation(decision, OperationEncode) {
		t.Fatalf("decision encoded instead of repairing matching local output: %+v", decision.Operations)
	}
}

func TestDecideEpisodeMissingProductionMetadataRepairsDifferingLocalRemoteOutputBeforeUpload(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		ProductionAudio: ObjectState{
			Label:     "episode output",
			Bucket:    "output",
			Key:       "episode.m4a",
			LocalPath: "/pod/episode.m4a",
			Local:     FileState{Exists: true, Size: 200, Checksum: "abc"},
			Remote:    RemoteState{Checked: true, Exists: true, Size: 200, ETag: `"def"`},
		},
		ProductionKnown: false,
	}, Options{})

	repair := requireOperation(t, decision, OperationRepairMetadata)
	upload := requireOperation(t, decision, OperationUploadProduction)
	assertProductionOperationDetails(t, repair, "output", "episode.m4a", "/pod/episode.m4a")
	assertProductionOperationDetails(t, upload, "output", "episode.m4a", "/pod/episode.m4a")
	if operationIndex(decision, OperationRepairMetadata) > operationIndex(decision, OperationUploadProduction) {
		t.Fatalf("operations = %+v, want repair before upload", decision.Operations)
	}
}

func TestDecideEpisodeFullApplySchedulesEncodeAndSingleRSSRegeneration(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		ProductionAudio: ObjectState{
			Label:     "episode output",
			Bucket:    "output",
			Key:       "episode.m4a",
			LocalPath: "/pod/episode.m4a",
		},
		RSSDirty: true,
	}, Options{})

	requireOperation(t, decision, OperationEncode)
	upload := requireOperation(t, decision, OperationUploadProduction)
	if upload.Bucket != "output" || upload.Key != "episode.m4a" || upload.LocalPath != "/pod/episode.m4a" {
		t.Fatalf("upload operation = %+v, want planned production upload", upload)
	}
	if upload.Reason != "newly encoded production audio will be uploaded" {
		t.Fatalf("upload reason = %q", upload.Reason)
	}
	if !upload.RequiresPrompt {
		t.Fatal("upload RequiresPrompt = false, want true")
	}
	rssOps := countOperations(decision, OperationRegenerateRSS)
	if rssOps != 1 {
		t.Fatalf("RSS operations = %d, want 1: %+v", rssOps, decision.Operations)
	}
}

func TestDecideEpisodeReencodeRegeneratesExistingProductionAudio(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		ProductionAudio: ObjectState{
			Label:     "episode output",
			Bucket:    "output",
			Key:       "episode.m4a",
			LocalPath: "/pod/episode.m4a",
			Local:     FileState{Exists: true, Size: 200},
			Remote:    RemoteState{Checked: true, Exists: false},
		},
		RSSDirty: true,
	}, Options{Reencode: true})

	requireOperation(t, decision, OperationEncode)
	upload := requireOperation(t, decision, OperationUploadProduction)
	if upload.Reason != "newly encoded production audio will be uploaded" {
		t.Fatalf("upload reason = %q", upload.Reason)
	}
}

func TestDecideEpisodeReencodeMarksRemoteOverwriteDestructive(t *testing.T) {
	decision := DecideEpisode(EpisodeInput{
		Metadata: appliedMetadata(),
		Master:   syncedMaster(),
		ProductionAudio: ObjectState{
			Label:     "episode output",
			Bucket:    "output",
			Key:       "episode.m4a",
			LocalPath: "/pod/episode.m4a",
			Local:     FileState{Exists: true, Size: 200},
			Remote:    RemoteState{Checked: true, Exists: true, Size: 200},
		},
	}, Options{Reencode: true})

	upload := requireOperation(t, decision, OperationUploadProduction)
	if !upload.Destructive {
		t.Fatalf("upload operation = %+v, want destructive remote overwrite", upload)
	}
	if upload.Reason != "remote production audio will be overwritten by re-encoded output" {
		t.Fatalf("upload reason = %q", upload.Reason)
	}
}

func TestEquivalentUsesSizeAndChecksumOrETag(t *testing.T) {
	if !equivalent(FileState{Exists: true, Size: 10, Checksum: "ABC"}, RemoteState{Exists: true, Size: 10, Checksum: "abc"}) {
		t.Fatal("checksum-equivalent objects did not match")
	}
	if !equivalent(FileState{Exists: true, Size: 10, Checksum: "abc"}, RemoteState{Exists: true, Size: 10, ETag: `"abc"`}) {
		t.Fatal("local checksum did not match usable remote etag")
	}
	if !equivalent(FileState{Exists: true, Size: 10, ETag: `"abc"`}, RemoteState{Exists: true, Size: 10, ETag: "abc"}) {
		t.Fatal("etag-equivalent objects did not match")
	}
	if equivalent(FileState{Exists: true, Size: 10, Checksum: "abc"}, RemoteState{Exists: true, Size: 10, Checksum: "def"}) {
		t.Fatal("different checksums matched")
	}
	if equivalent(FileState{Exists: true, Size: 10}, RemoteState{Exists: true, Size: 11}) {
		t.Fatal("different sizes matched")
	}
	if equivalent(FileState{Exists: true, Size: 10}, RemoteState{Exists: true, Size: 10}) {
		t.Fatal("size-only objects matched")
	}
	if equivalent(FileState{Exists: true, Size: 10, Checksum: "abc"}, RemoteState{Exists: true, Size: 10, ETag: `"multipart-etag-2"`}) {
		t.Fatal("unusable remote etag matched by size")
	}
}

func appliedMetadata() MetadataState {
	return MetadataState{Planned: true, Applied: true, Matches: true, Complete: true}
}

func syncedMaster() ObjectState {
	return ObjectState{
		Kind:      ObjectMaster,
		Label:     "episode master",
		Bucket:    "input",
		Key:       "masters/episode.flac",
		LocalPath: "/pod/masters/episode.flac",
		Local:     FileState{Exists: true, Size: 100, ETag: "same"},
		Remote:    RemoteState{Checked: true, Exists: true, Size: 100, ETag: `"same"`},
	}
}

func requireOperation(t *testing.T, decision Decision, kind OperationKind) Operation {
	t.Helper()
	for _, operation := range decision.Operations {
		if operation.Kind == kind {
			return operation
		}
	}
	t.Fatalf("missing operation %q in %+v", kind, decision.Operations)
	return Operation{}
}

func hasOperation(decision Decision, kind OperationKind) bool {
	return countOperations(decision, kind) > 0
}

func countOperations(decision Decision, kind OperationKind) int {
	count := 0
	for _, operation := range decision.Operations {
		if operation.Kind == kind {
			count++
		}
	}
	return count
}

func operationIndex(decision Decision, kind OperationKind) int {
	for i, operation := range decision.Operations {
		if operation.Kind == kind {
			return i
		}
	}
	return -1
}

func assertProductionOperationDetails(t *testing.T, operation Operation, bucket, key, localPath string) {
	t.Helper()
	if operation.ObjectKind != ObjectProductionAudio {
		t.Fatalf("operation ObjectKind = %q, want %q", operation.ObjectKind, ObjectProductionAudio)
	}
	if operation.Bucket != bucket || operation.Key != key || operation.LocalPath != localPath {
		t.Fatalf("operation = %+v, want bucket=%q key=%q localPath=%q", operation, bucket, key, localPath)
	}
}

func requireCheck(t *testing.T, decision Decision, kind string) Check {
	t.Helper()
	for _, check := range decision.Checks {
		if check.Kind == kind {
			return check
		}
	}
	t.Fatalf("missing check %q in %+v", kind, decision.Checks)
	return Check{}
}

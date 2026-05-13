package cmd

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	workflow "github.com/sa6mwa/mkpod/internal/workflow"
)

type applyPreflightRemote struct {
	infos map[string]*s3store.FileInfo
	errs  map[string]error
}

func (r *applyPreflightRemote) GetFileInfo(_ context.Context, bucket, key string) (*s3store.FileInfo, error) {
	if r.errs != nil {
		if err, ok := r.errs[bucket+"/"+key]; ok {
			return nil, err
		}
	}
	if r.infos == nil {
		return &s3store.FileInfo{Exists: false}, nil
	}
	if info, ok := r.infos[bucket+"/"+key]; ok {
		return info, nil
	}
	return &s3store.FileInfo{Exists: false}, nil
}

func TestBuildNewEpisodeApplyDecisionJustMasterUploadsLocalMaster(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	masterPath := filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.flac")
	writeFile(t, masterPath, []byte("master"))
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	plan.Episode.Input = "masters/new.flac"

	decision, err := buildNewEpisodeApplyDecision(context.Background(), plan, applyPreflightOptions{JustMaster: true}, &applyPreflightRemote{})
	if err != nil {
		t.Fatalf("buildNewEpisodeApplyDecision() error = %v", err)
	}
	operation := requireWorkflowOperation(t, decision, workflow.OperationUploadMaster)
	if !operation.RequiresPrompt {
		t.Fatal("upload master RequiresPrompt = false, want true")
	}
	if operation.Bucket != "input" || operation.Key != "masters/new.flac" {
		t.Fatalf("upload operation = %+v, want input/masters/new.flac", operation)
	}
	if hasWorkflowOperation(decision, workflow.OperationEncode) {
		t.Fatalf("just-master decision included encode: %+v", decision.Operations)
	}
}

func TestBuildNewEpisodeApplyDecisionPropagatesRemoteInspectionError(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	plan.Episode.Input = "masters/new.wav"
	remote := &applyPreflightRemote{errs: map[string]error{
		"input/masters/new.wav": errors.New("head denied"),
	}}

	_, err := buildNewEpisodeApplyDecision(context.Background(), plan, applyPreflightOptions{}, remote)
	if err == nil {
		t.Fatal("buildNewEpisodeApplyDecision() error = nil, want remote inspection error")
	}
	if !strings.Contains(err.Error(), "inspect remote episode master masters/new.wav in bucket input") || !strings.Contains(err.Error(), "head denied") {
		t.Fatalf("buildNewEpisodeApplyDecision() error = %v", err)
	}
}

func TestBuildNewEpisodeApplyDecisionForceMissingLocalMasterBlocks(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	plan.Episode.Input = "masters/missing.flac"
	remote := &applyPreflightRemote{infos: map[string]*s3store.FileInfo{
		"input/masters/missing.flac": {Exists: true, Size: 100},
	}}

	decision, err := buildNewEpisodeApplyDecision(context.Background(), plan, applyPreflightOptions{JustMaster: true, Force: true}, remote)
	if err != nil {
		t.Fatalf("buildNewEpisodeApplyDecision() error = %v", err)
	}
	if decision.State != workflow.StateBlocked {
		t.Fatalf("State = %q, want blocked", decision.State)
	}
	if hasWorkflowOperation(decision, workflow.OperationDownloadMaster) {
		t.Fatalf("force decision downloaded missing local master: %+v", decision.Operations)
	}
}

func TestBuildNewEpisodeApplyDecisionRemoteProductionWithCompleteMetadataDoesNotDownload(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	masterPath := filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.flac")
	writeFile(t, masterPath, []byte("master"))
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	plan.Episode.Input = "masters/new.flac"
	plan.Episode.Output = "new.m4a"
	plan.Episode.Type = "audio/mp4"
	plan.Episode.Length = 200
	plan.Episode.Duration = model.ItunesDuration{Duration: time.Minute}
	plan.Episode.PubDate = model.ItunesTime{Time: time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)}
	remote := &applyPreflightRemote{infos: map[string]*s3store.FileInfo{
		"input/masters/new.flac": {Exists: true, Size: int64(len("master"))},
		"output/new.m4a":         {Exists: true, Size: 200},
	}}

	decision, err := buildNewEpisodeApplyDecision(context.Background(), plan, applyPreflightOptions{}, remote)
	if err != nil {
		t.Fatalf("buildNewEpisodeApplyDecision() error = %v", err)
	}
	if hasWorkflowOperation(decision, workflow.OperationDownloadProduction) {
		t.Fatalf("decision downloaded production despite complete metadata: %+v", decision.Operations)
	}
	if hasWorkflowOperation(decision, workflow.OperationEncode) {
		t.Fatalf("decision encoded despite remote complete production audio: %+v", decision.Operations)
	}
}

func TestBuildNewEpisodeApplyDecisionMissingProductionPlansUploadAfterEncode(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	masterPath := filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.flac")
	writeFile(t, masterPath, []byte("master"))
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	plan.Episode.Input = "masters/new.flac"
	plan.Episode.Output = "new.m4a"
	remote := &applyPreflightRemote{infos: map[string]*s3store.FileInfo{
		"input/masters/new.flac": {Exists: true, Size: int64(len("master"))},
	}}

	decision, err := buildNewEpisodeApplyDecision(context.Background(), plan, applyPreflightOptions{}, remote)
	if err != nil {
		t.Fatalf("buildNewEpisodeApplyDecision() error = %v", err)
	}
	requireWorkflowOperation(t, decision, workflow.OperationEncode)
	upload := requireWorkflowOperation(t, decision, workflow.OperationUploadProduction)
	if upload.Bucket != "output" || upload.Key != "new.m4a" {
		t.Fatalf("upload operation = %+v, want output/new.m4a", upload)
	}
	if !upload.RequiresPrompt {
		t.Fatal("upload RequiresPrompt = false, want true")
	}
}

func TestBuildNewEpisodeApplyDecisionReencodeExistingLocalProduction(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	masterPath := filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.flac")
	writeFile(t, masterPath, []byte("master"))
	outputPath := filepath.Join(atom.LocalStorageDirExpanded(), "new.m4a")
	writeFile(t, outputPath, []byte("old output"))
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	plan.Episode.Input = "masters/new.flac"
	plan.Episode.Output = "new.m4a"
	remote := &applyPreflightRemote{infos: map[string]*s3store.FileInfo{
		"input/masters/new.flac": {Exists: true, Size: int64(len("master"))},
	}}

	decision, err := buildNewEpisodeApplyDecision(context.Background(), plan, applyPreflightOptions{Reencode: true}, remote)
	if err != nil {
		t.Fatalf("buildNewEpisodeApplyDecision() error = %v", err)
	}
	requireWorkflowOperation(t, decision, workflow.OperationEncode)
	upload := requireWorkflowOperation(t, decision, workflow.OperationUploadProduction)
	if upload.Reason != "newly encoded production audio will be uploaded" {
		t.Fatalf("upload reason = %q", upload.Reason)
	}
}

func TestWriteWorkflowDecisionShowsChecksOperationsAndPrompts(t *testing.T) {
	decision := workflow.Decision{
		State: workflow.StateMasterSynced,
		Checks: []workflow.Check{
			{Label: "metadata", Passed: true, Reason: "already matches"},
			{Label: "production audio", Passed: false, Reason: "local and remote differ"},
		},
		Operations: []workflow.Operation{
			{
				Kind:           workflow.OperationUploadMaster,
				Label:          "episode master",
				Bucket:         "input",
				Key:            "masters/new.flac",
				LocalPath:      "/tmp/pod/masters/new.flac",
				Reason:         "remote is missing",
				RequiresPrompt: true,
			},
		},
	}
	var out bytes.Buffer
	writeWorkflowDecision(&out, decision)
	got := out.String()
	for _, want := range []string{
		"Workflow state: master-synced",
		"- ok: metadata (already matches)",
		"- blocked: production audio (local and remote differ)",
		"- upload-master: episode master s3://input/masters/new.flac <= /tmp/pod/masters/new.flac (remote is missing) [prompt]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("decision output missing %q:\n%s", want, got)
		}
	}
}

func TestWriteWorkflowDecisionShowsNoOperations(t *testing.T) {
	decision := workflow.Decision{State: workflow.StateComplete}
	var out bytes.Buffer
	writeWorkflowDecision(&out, decision)
	if got := out.String(); !strings.Contains(got, "Operations: none") {
		t.Fatalf("decision output = %q, want no operations", got)
	}
}

func requireWorkflowOperation(t *testing.T, decision workflow.Decision, kind workflow.OperationKind) workflow.Operation {
	t.Helper()
	for _, operation := range decision.Operations {
		if operation.Kind == kind {
			return operation
		}
	}
	t.Fatalf("missing operation %q in %+v", kind, decision.Operations)
	return workflow.Operation{}
}

func hasWorkflowOperation(decision workflow.Decision, kind workflow.OperationKind) bool {
	for _, operation := range decision.Operations {
		if operation.Kind == kind {
			return true
		}
	}
	return false
}

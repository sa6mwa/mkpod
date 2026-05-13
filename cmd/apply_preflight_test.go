package cmd

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	workflow "github.com/sa6mwa/mkpod/internal/workflow"
)

type applyPreflightRemote struct {
	infos map[string]*s3store.FileInfo
}

func (r *applyPreflightRemote) GetFileInfo(_ context.Context, bucket, key string) (*s3store.FileInfo, error) {
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

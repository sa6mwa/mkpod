package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/media/encode"
	"github.com/sa6mwa/mkpod/internal/spec"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	workflow "github.com/sa6mwa/mkpod/internal/workflow"
)

type applyPreflightOptions struct {
	JustMaster bool
	Yes        bool
	Force      bool
}

type applyRemoteInspector interface {
	GetFileInfo(context.Context, string, string) (*s3store.FileInfo, error)
}

func buildNewEpisodeApplyDecision(ctx context.Context, plan *newEpisodePlan, options applyPreflightOptions, inspector applyRemoteInspector) (workflow.Decision, error) {
	config := spec.New(plan.SpecFile)
	atom, err := config.Load(ctx)
	if err != nil {
		return workflow.Decision{}, err
	}
	episode := plan.Episode
	if err := spec.ApplyEpisodeDefaultsForEncoding(atom, &episode); err != nil {
		return workflow.Decision{}, err
	}

	mode := workflow.ModeFull
	if options.JustMaster {
		mode = workflow.ModeJustMaster
	}
	input := workflow.EpisodeInput{
		Metadata:        newEpisodeMetadataState(atom, &episode),
		Master:          newWorkflowObject(ctx, atom, inspector, workflow.ObjectMaster, "episode master", atom.Config.Aws.Buckets.Input, episode.Input, true),
		Artifacts:       newEpisodeWorkflowArtifacts(ctx, atom, inspector, &episode),
		ProductionAudio: newEpisodeWorkflowProduction(ctx, atom, inspector, &episode),
		ProductionKnown: len(spec.MissingFieldsForRSS(atom, &episode)) == 0,
		RSSDirty:        !options.JustMaster,
	}
	return workflow.DecideEpisode(input, workflow.Options{Mode: mode, Yes: options.Yes, Force: options.Force}), nil
}

func newEpisodeMetadataState(atom *model.Podcast, episode *model.Episode) workflow.MetadataState {
	state := workflow.MetadataState{Planned: true, Complete: true}
	index := atom.ContainsEpisode(episode.UID)
	if index < 0 {
		return state
	}
	state.Applied = true
	if newEpisodePlanMatchesExisting(episode, &atom.Episodes[index]) {
		state.Matches = true
		return state
	}
	state.Conflicts = true
	return state
}

func newEpisodeWorkflowArtifacts(ctx context.Context, atom *model.Podcast, inspector applyRemoteInspector, episode *model.Episode) []workflow.ObjectState {
	artifacts := make([]workflow.ObjectState, 0, 2)
	if strings.TrimSpace(atom.Encoding.Coverfront) != "" {
		artifacts = append(artifacts, newWorkflowObjectInBuckets(ctx, atom, inspector, workflow.ObjectEncodeArtifact, "cover image", []string{atom.Config.Aws.Buckets.Input, atom.Config.Aws.Buckets.Output}, atom.Encoding.Coverfront, true))
	}
	if image := spec.EffectiveEpisodeImage(atom, episode); strings.TrimSpace(image) != "" {
		artifacts = append(artifacts, newWorkflowObjectInBuckets(ctx, atom, inspector, workflow.ObjectEncodeArtifact, "episode image", []string{atom.Config.Aws.Buckets.Input, atom.Config.Aws.Buckets.Output}, image, true))
	}
	return artifacts
}

func newEpisodeWorkflowProduction(ctx context.Context, atom *model.Podcast, inspector applyRemoteInspector, episode *model.Episode) workflow.ObjectState {
	output := strings.TrimSpace(episode.Output)
	if output == "" {
		output = plannedEpisodeOutput(atom, episode)
	}
	return newWorkflowObject(ctx, atom, inspector, workflow.ObjectProductionAudio, "production audio", atom.Config.Aws.Buckets.Output, output, false)
}

func plannedEpisodeOutput(atom *model.Podcast, episode *model.Episode) string {
	if strings.TrimSpace(episode.Input) == "" {
		return ""
	}
	inputPath := filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(episode.Input))
	if _, err := os.Stat(inputPath); err != nil {
		return ""
	}
	contentType, err := encode.GetFileContentType(inputPath)
	if err != nil {
		return ""
	}
	planned, err := encode.PlanEpisode(atom, episode, contentType)
	if err != nil {
		return ""
	}
	return planned.Output
}

func newWorkflowObject(ctx context.Context, atom *model.Podcast, inspector applyRemoteInspector, kind workflow.ObjectKind, label, bucket, rawKey string, required bool) workflow.ObjectState {
	return newWorkflowObjectInBuckets(ctx, atom, inspector, kind, label, []string{bucket}, rawKey, required)
}

func newWorkflowObjectInBuckets(ctx context.Context, atom *model.Podcast, inspector applyRemoteInspector, kind workflow.ObjectKind, label string, buckets []string, rawKey string, required bool) workflow.ObjectState {
	key := normalizeStorageKey(atom, rawKey)
	object := workflow.ObjectState{
		Kind:      kind,
		Label:     label,
		Key:       key,
		LocalPath: localAssetPath(atom, key),
		Required:  required,
	}
	if key == "" {
		return object
	}
	if info, err := os.Stat(object.LocalPath); err == nil {
		object.Local.Exists = true
		object.Local.Size = info.Size()
	}
	if inspector == nil {
		return object
	}
	for _, bucket := range buckets {
		bucket = strings.TrimSpace(bucket)
		if bucket == "" {
			continue
		}
		remote, err := inspector.GetFileInfo(ctx, bucket, key)
		if err != nil {
			continue
		}
		object.Remote.Checked = true
		if remote != nil && remote.Exists {
			object.Bucket = bucket
			object.Remote.Exists = true
			object.Remote.Size = remote.Size
			object.Remote.ETag = remote.ETag
			object.Remote.ContentType = remote.ContentType
			object.Remote.LastModified = remote.LastModified
			return object
		}
		if object.Bucket == "" {
			object.Bucket = bucket
		}
	}
	return object
}

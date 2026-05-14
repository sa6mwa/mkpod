package cmd

import (
	"context"
	"fmt"
	"io"
	"mime"
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
	Reencode   bool
}

type applyRemoteInspector interface {
	GetFileInfo(context.Context, string, string) (*s3store.FileInfo, error)
}

func buildNewEpisodeApplyDecision(ctx context.Context, plan *newEpisodePlan, options applyPreflightOptions, inspector applyRemoteInspector) (workflow.Decision, error) {
	return buildEpisodeApplyDecision(ctx, plan, options, inspector, false)
}

func buildRenewEpisodeApplyDecision(ctx context.Context, plan *newEpisodePlan, options applyPreflightOptions, inspector applyRemoteInspector) (workflow.Decision, error) {
	return buildEpisodeApplyDecision(ctx, plan, options, inspector, true)
}

func buildEpisodeApplyDecision(ctx context.Context, plan *newEpisodePlan, options applyPreflightOptions, inspector applyRemoteInspector, renew bool) (workflow.Decision, error) {
	config := spec.New(plan.SpecFile)
	atom, err := config.Load(ctx)
	if err != nil {
		return workflow.Decision{}, err
	}
	metadataEpisode := plan.Episode
	decisionEpisode := plan.Episode
	if err := spec.ApplyEpisodeDefaultsForEncoding(atom, &decisionEpisode); err != nil {
		return workflow.Decision{}, err
	}

	mode := workflow.ModeFull
	if options.JustMaster {
		mode = workflow.ModeJustMaster
	}
	input := workflow.EpisodeInput{
		Metadata:        episodeMetadataState(atom, &metadataEpisode, renew),
		ProductionKnown: len(spec.MissingFieldsForRSS(atom, &decisionEpisode)) == 0,
		RSSDirty:        !options.JustMaster,
	}
	input.Master, err = newWorkflowObject(ctx, atom, inspector, workflow.ObjectMaster, "episode master", atom.Config.Aws.Buckets.Input, decisionEpisode.Input, true)
	if err != nil {
		return workflow.Decision{}, err
	}
	input.Artifacts, err = newEpisodeWorkflowArtifacts(ctx, atom, inspector, &decisionEpisode)
	if err != nil {
		return workflow.Decision{}, err
	}
	input.ProductionAudio, err = newEpisodeWorkflowProduction(ctx, atom, inspector, &decisionEpisode, input.Master.Remote.ContentType)
	if err != nil {
		return workflow.Decision{}, err
	}
	return workflow.DecideEpisode(input, workflow.Options{Mode: mode, Yes: options.Yes, Force: options.Force, Reencode: options.Reencode}), nil
}

func newEpisodeMetadataState(atom *model.Podcast, episode *model.Episode) workflow.MetadataState {
	return episodeMetadataState(atom, episode, false)
}

func episodeMetadataState(atom *model.Podcast, episode *model.Episode, renew bool) workflow.MetadataState {
	state := workflow.MetadataState{Planned: true, Complete: true}
	index := atom.ContainsEpisode(episode.UID)
	if index < 0 {
		if renew {
			state.Planned = false
			state.Conflicts = true
		}
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

func newEpisodeWorkflowArtifacts(ctx context.Context, atom *model.Podcast, inspector applyRemoteInspector, episode *model.Episode) ([]workflow.ObjectState, error) {
	artifacts := make([]workflow.ObjectState, 0, 2)
	seen := make(map[string]struct{})
	appendArtifact := func(label, rawKey string) error {
		key := normalizeStorageKey(atom, rawKey)
		if key == "" {
			return nil
		}
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		artifact, err := newWorkflowObjectInBuckets(ctx, atom, inspector, workflow.ObjectEncodeArtifact, label, []string{atom.Config.Aws.Buckets.Input, atom.Config.Aws.Buckets.Output}, key, true)
		if err != nil {
			return err
		}
		artifacts = append(artifacts, artifact)
		return nil
	}
	if err := appendArtifact("cover image", atom.Encoding.Coverfront); err != nil {
		return nil, err
	}
	if err := appendArtifact("episode image", spec.EffectiveEpisodeImage(atom, episode)); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func newEpisodeWorkflowProduction(ctx context.Context, atom *model.Podcast, inspector applyRemoteInspector, episode *model.Episode, remoteInputContentType string) (workflow.ObjectState, error) {
	output := strings.TrimSpace(episode.Output)
	if output == "" {
		var err error
		output, err = plannedEpisodeOutput(atom, episode, remoteInputContentType)
		if err != nil {
			return workflow.ObjectState{}, err
		}
	}
	return newWorkflowObject(ctx, atom, inspector, workflow.ObjectProductionAudio, "production audio", atom.Config.Aws.Buckets.Output, output, false)
}

func plannedEpisodeOutput(atom *model.Podcast, episode *model.Episode, remoteInputContentType string) (string, error) {
	if strings.TrimSpace(episode.Input) == "" {
		return "", nil
	}
	inputContentType := strings.TrimSpace(remoteInputContentType)
	inputPath := filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(episode.Input))
	if _, err := os.Stat(inputPath); err == nil {
		contentType, err := encode.GetFileContentType(inputPath)
		if err != nil {
			return "", fmt.Errorf("detect input content type for %s: %w", inputPath, err)
		}
		inputContentType = contentType
	}
	if inputContentType == "" {
		inputContentType = mime.TypeByExtension(filepath.Ext(episode.Input))
	}
	if inputContentType == "" {
		return "", nil
	}
	planned, err := encode.PlanEpisode(atom, episode, inputContentType)
	if err != nil {
		return "", fmt.Errorf("plan production output for input %s: %w", episode.Input, err)
	}
	return planned.Output, nil
}

func newWorkflowObject(ctx context.Context, atom *model.Podcast, inspector applyRemoteInspector, kind workflow.ObjectKind, label, bucket, rawKey string, required bool) (workflow.ObjectState, error) {
	return newWorkflowObjectInBuckets(ctx, atom, inspector, kind, label, []string{bucket}, rawKey, required)
}

func newWorkflowObjectInBuckets(ctx context.Context, atom *model.Podcast, inspector applyRemoteInspector, kind workflow.ObjectKind, label string, buckets []string, rawKey string, required bool) (workflow.ObjectState, error) {
	key := normalizeStorageKey(atom, rawKey)
	object := workflow.ObjectState{
		Kind:      kind,
		Label:     label,
		Key:       key,
		LocalPath: localAssetPath(atom, key),
		Required:  required,
	}
	if key == "" {
		return object, nil
	}
	if info, err := os.Stat(object.LocalPath); err == nil {
		object.Local.Exists = true
		object.Local.Size = info.Size()
		if checksum, err := fileMD5Hex(object.LocalPath); err == nil {
			object.Local.Checksum = checksum
		} else {
			return workflow.ObjectState{}, fmt.Errorf("checksum local %s %s: %w", label, object.LocalPath, err)
		}
	}
	if inspector == nil {
		return object, nil
	}
	for _, bucket := range buckets {
		bucket = strings.TrimSpace(bucket)
		if bucket == "" {
			continue
		}
		remote, err := inspector.GetFileInfo(ctx, bucket, key)
		if err != nil {
			return workflow.ObjectState{}, fmt.Errorf("inspect remote %s %s in bucket %s: %w", label, key, bucket, err)
		}
		object.Remote.Checked = true
		if remote != nil && remote.Exists {
			object.Bucket = bucket
			object.Remote.Exists = true
			object.Remote.Size = remote.Size
			object.Remote.ETag = remote.ETag
			object.Remote.ContentType = remote.ContentType
			object.Remote.LastModified = remote.LastModified
			return object, nil
		}
		if object.Bucket == "" {
			object.Bucket = bucket
		}
	}
	return object, nil
}

func writeWorkflowDecision(w io.Writer, decision workflow.Decision) {
	fmt.Fprintf(w, "Workflow state: %s\n", decision.State)
	if len(decision.Checks) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Checks:")
		for _, check := range decision.Checks {
			marker := "ok"
			if !check.Passed {
				marker = "blocked"
			}
			fmt.Fprintf(w, "- %s: %s", marker, check.Label)
			if strings.TrimSpace(check.Reason) != "" {
				fmt.Fprintf(w, " (%s)", check.Reason)
			}
			fmt.Fprintln(w)
		}
	}
	if len(decision.Operations) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Operations:")
		for _, operation := range decision.Operations {
			fmt.Fprintf(w, "- %s: %s", operation.Kind, operation.Label)
			if operation.Bucket != "" && operation.Key != "" {
				fmt.Fprintf(w, " s3://%s/%s", operation.Bucket, operation.Key)
			} else if operation.Key != "" {
				fmt.Fprintf(w, " %s", operation.Key)
			}
			if operation.LocalPath != "" {
				fmt.Fprintf(w, " <= %s", operation.LocalPath)
			}
			if strings.TrimSpace(operation.Reason) != "" {
				fmt.Fprintf(w, " (%s)", operation.Reason)
			}
			if operation.RequiresPrompt {
				fmt.Fprint(w, " [prompt]")
			}
			if operation.Destructive {
				fmt.Fprint(w, " [destructive]")
			}
			fmt.Fprintln(w)
		}
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Operations: none")
}

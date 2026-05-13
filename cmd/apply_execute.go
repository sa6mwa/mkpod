package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/prompt"
	"github.com/sa6mwa/mkpod/internal/spec"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	workflow "github.com/sa6mwa/mkpod/internal/workflow"
	"golang.org/x/term"
)

type applyWorkflowStorage interface {
	applyRemoteInspector
	DownloadFile(context.Context, string, string) error
	UploadFile(context.Context, string, string, string, *s3store.UploadOptions) error
}

func applyNewEpisodeJustMaster(ctx context.Context, plan *newEpisodePlan, decision workflow.Decision, options applySavedPlanOptions, storage applyWorkflowStorage) error {
	if err := requireApplyDecisionProceed(decision, options); err != nil {
		return err
	}
	atom, err := spec.New(plan.SpecFile).Load(ctx)
	if err != nil {
		return err
	}
	if storage == nil {
		return errors.New("apply requires remote storage access")
	}
	if err := applyNewEpisodePlan(ctx, plan); err != nil {
		return err
	}
	for _, operation := range decision.Operations {
		switch operation.Kind {
		case workflow.OperationWriteMetadata:
			continue
		case workflow.OperationUploadMaster, workflow.OperationUploadArtifact:
			if err := uploadApplyObject(ctx, atom, storage, operation); err != nil {
				return err
			}
		case workflow.OperationDownloadMaster, workflow.OperationDownloadArtifact:
			if err := storage.DownloadFile(ctx, operation.Bucket, operation.Key); err != nil {
				return fmt.Errorf("download %s %s from bucket %s: %w", operation.Label, operation.Key, operation.Bucket, err)
			}
		default:
			return fmt.Errorf("unexpected just-master operation %q", operation.Kind)
		}
	}
	fmt.Printf("Continue with: mkpod apply %s\n", planApplyPathHint(plan))
	return nil
}

func uploadApplyObject(ctx context.Context, atom *model.Podcast, storage applyWorkflowStorage, operation workflow.Operation) error {
	if operation.LocalPath == "" {
		return fmt.Errorf("upload %s: local path is empty", operation.Label)
	}
	options := &s3store.UploadOptions{StorageClass: atom.Config.Aws.Buckets.GetStorageClass(operation.Bucket)}
	if err := storage.UploadFile(ctx, operation.Bucket, operation.Key, operation.LocalPath, options); err != nil {
		return fmt.Errorf("upload %s %s to bucket %s: %w", operation.Label, operation.Key, operation.Bucket, err)
	}
	return nil
}

func requireApplyDecisionProceed(decision workflow.Decision, options applySavedPlanOptions) error {
	if decision.State == workflow.StateBlocked {
		for _, check := range decision.Checks {
			if !check.Passed {
				return fmt.Errorf("apply preflight blocked: %s: %s", check.Label, check.Reason)
			}
		}
		return errors.New("apply preflight blocked")
	}
	if options.Yes || options.Force || !decisionRequiresPrompt(decision) {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return errors.New("apply requires confirmation; rerun with --yes or --force")
	}
	proceed := false
	if err := survey.AskOne(&survey.Confirm{
		Message: "Proceed with these apply operations?",
		Default: false,
	}, &proceed); err != nil {
		return err
	}
	if !proceed {
		return errors.New("apply cancelled")
	}
	return nil
}

func decisionRequiresPrompt(decision workflow.Decision) bool {
	for _, operation := range decision.Operations {
		if operation.RequiresPrompt {
			return true
		}
	}
	return false
}

func newApplyStorage(ctx context.Context, specFile string, options applySavedPlanOptions) (applyWorkflowStorage, error) {
	atom, err := spec.New(specFile).Load(ctx)
	if err != nil {
		return nil, err
	}
	return s3store.New(atom, prompt.New(false, options.Force || options.Yes)), nil
}

func sameWorkflowDecision(a, b workflow.Decision) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

func planApplyPathHint(plan *newEpisodePlan) string {
	if plan == nil || plan.SpecFile == "" {
		return "new.plan.json"
	}
	return defaultPlanPath("new", plan.SpecFile)
}

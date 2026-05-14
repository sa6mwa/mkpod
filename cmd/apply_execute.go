package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sa6mwa/mkpod/internal/app/model"
	logger "github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/prompt"
	"github.com/sa6mwa/mkpod/internal/rss"
	"github.com/sa6mwa/mkpod/internal/spec"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	workflow "github.com/sa6mwa/mkpod/internal/workflow"
	"golang.org/x/term"
)

type applyWorkflowStorage interface {
	applyRemoteInspector
	DownloadFile(context.Context, string, string) error
	FileExists(context.Context, string, string) (bool, error)
	UploadFile(context.Context, string, string, string, *s3store.UploadOptions) error
}

func applyNewEpisodeJustMaster(ctx context.Context, plan *newEpisodePlan, decision workflow.Decision, options applySavedPlanOptions, storage applyWorkflowStorage, renew bool) error {
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
	if err := applyEpisodePlanMetadata(ctx, plan, renew); err != nil {
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
	fmt.Printf("Continue with: mkpod apply %s\n", planApplyPathHint(plan, options.PlanPath))
	return nil
}

func applyNewEpisodeFull(ctx context.Context, plan *newEpisodePlan, decision workflow.Decision, options applySavedPlanOptions, storage applyWorkflowStorage, renew bool) error {
	if err := requireApplyDecisionProceed(decision, options); err != nil {
		return err
	}
	if err := applyEpisodePlanMetadata(ctx, plan, renew); err != nil {
		return err
	}
	atom, err := spec.New(plan.SpecFile).Load(ctx)
	if err != nil {
		return err
	}
	encodedThisRun := false
	for _, operation := range decision.Operations {
		switch operation.Kind {
		case workflow.OperationWriteMetadata:
			continue
		case workflow.OperationUploadMaster, workflow.OperationUploadArtifact:
			if storage == nil {
				continue
			}
			if err := uploadApplyObject(ctx, atom, storage, operation); err != nil {
				return err
			}
		case workflow.OperationDownloadMaster, workflow.OperationDownloadArtifact, workflow.OperationDownloadProduction:
			if storage == nil {
				continue
			}
			if err := storage.DownloadFile(ctx, operation.Bucket, operation.Key); err != nil {
				return fmt.Errorf("download %s %s from bucket %s: %w", operation.Label, operation.Key, operation.Bucket, err)
			}
		case workflow.OperationUploadProduction:
			if storage == nil {
				continue
			}
			if encodedThisRun {
				atom, err = spec.New(plan.SpecFile).Load(ctx)
				if err != nil {
					return err
				}
				operation = refreshedProductionUploadOperation(atom, plan.Episode.UID, operation)
			}
			if err := uploadApplyObject(ctx, atom, storage, operation); err != nil {
				return err
			}
		case workflow.OperationEncode, workflow.OperationRepairMetadata:
			if err := runEncodeWorkflow(logger.WithDefaultLogger(ctx), []string{strconv.FormatInt(plan.Episode.UID, 10)}, encodeWorkflowOptions{
				SpecFile:              plan.SpecFile,
				All:                   false,
				AskNoQuestions:        true,
				ForceReencode:         operation.Kind == workflow.OperationEncode && options.Reencode,
				NeverReencode:         operation.Kind == workflow.OperationRepairMetadata,
				SkipOutputUpload:      true,
				LocalOnly:             storage == nil,
				PreserveLastBuildDate: true,
			}); err != nil {
				return err
			}
			if operation.Kind == workflow.OperationEncode {
				encodedThisRun = true
			}
		case workflow.OperationRegenerateRSS:
			if err := regenerateApplyRSS(ctx, plan.SpecFile); err != nil {
				return err
			}
		case workflow.OperationNoop:
			continue
		default:
			return fmt.Errorf("unexpected apply operation %q", operation.Kind)
		}
	}
	printPostApplyPublishHint(plan.SpecFile)
	return nil
}

func applyEpisodePlanMetadata(ctx context.Context, plan *newEpisodePlan, renew bool) error {
	if renew {
		return applyRenewEpisodePlan(ctx, plan)
	}
	return applyNewEpisodePlan(ctx, plan)
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

func refreshedProductionUploadOperation(atom *model.Podcast, uid int64, operation workflow.Operation) workflow.Operation {
	if atom == nil || operation.ObjectKind != workflow.ObjectProductionAudio {
		return operation
	}
	for i := range atom.Episodes {
		if atom.Episodes[i].UID == uid && atom.Episodes[i].Output != "" {
			operation.LocalPath = localAssetPath(atom, atom.Episodes[i].Output)
			operation.Key = atom.Episodes[i].Output
			return operation
		}
	}
	return operation
}

func regenerateApplyRSS(ctx context.Context, specFile string) error {
	atom, err := spec.New(specFile).Load(ctx)
	if err != nil {
		return err
	}
	return rss.New().WriteRSS(ctx, atom)
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
	choice := ""
	if err := survey.AskOne(&survey.Select{
		Message: "Proceed with these apply operations?",
		Options: []string{"No", "Yes", "Exit program"},
		Default: "No",
	}, &choice); err != nil {
		return err
	}
	switch choice {
	case "Yes":
		return nil
	case "Exit program":
		return errors.New("apply cancelled")
	default:
		return errors.New("apply cancelled")
	}
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

func planApplyPathHint(plan *newEpisodePlan, planPath string) string {
	if planPath != "" {
		return planPath
	}
	if plan == nil || plan.SpecFile == "" {
		return "new.plan.json"
	}
	return defaultPlanPath("new", plan.SpecFile)
}

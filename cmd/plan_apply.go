package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/blenderaddon"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/media/encode"
	"github.com/sa6mwa/mkpod/internal/media/preprocess"
	"github.com/sa6mwa/mkpod/internal/prompt"
	"github.com/sa6mwa/mkpod/internal/spec"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	workflow "github.com/sa6mwa/mkpod/internal/workflow"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply <plan.json>",
	Short: "Apply a saved mkpod workflow plan",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("provide exactly one saved plan JSON file")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		options, err := applySavedPlanOptionsFromCommand(cmd)
		if err != nil {
			l.Error("Unable to read apply options", "error", err)
			os.Exit(1)
		}
		if err := applySavedPlanWithOptions(context.Background(), args[0], options); err != nil {
			l.Error("Unable to apply saved workflow plan", "error", err)
			os.Exit(1)
		}
	},
}

var inspectCmd = &cobra.Command{
	Use:   "inspect <plan.json>",
	Short: "Inspect a saved mkpod workflow plan",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("provide exactly one saved plan JSON file")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		if err := inspectSavedPlan(args[0]); err != nil {
			l.Error("Unable to inspect saved workflow plan", "error", err)
			os.Exit(1)
		}
	},
}

type savedPlan struct {
	Workflow string          `json:"workflow"`
	Plan     json.RawMessage `json:"plan"`
}

type applySavedPlanOptions struct {
	JustMaster  bool
	Yes         bool
	Force       bool
	Reencode    bool
	CheckRemote bool
	Storage     applyWorkflowStorage
}

type episodeWorkflowPlan struct {
	SpecFile           string
	UID                int64
	Title              string
	RemoveRemoteMaster bool
	Input              string
	InputPath          string
	InputContentType   string
	Output             string
	OutputPath         string
	OutputExists       bool
	WillEncode         bool
	EncodeReason       string
	EncodeMode         string
	PreferredFormat    string
	EpisodeFormat      string
	MetadataWillWrite  bool
	FeedFile           string
	FeedPath           string
	FeedExists         bool
	RSSReady           bool
	RSSMissingFields   []string
	RemotePreview      bool
	RemoteOutput       remoteObjectPlan
	RemoteFeed         remoteObjectPlan
	Operations         []workflowOperation
}

type feedWorkflowPlan struct {
	SpecFile        string
	FeedFile        string
	FeedPath        string
	FeedExists      bool
	OutputBucket    string
	ValidEpisodes   int
	SkippedEpisodes int
	Upload          bool
	RemotePreview   bool
	RemoteFeed      remoteObjectPlan
	Operations      []workflowOperation
}

type remoteObjectPlan struct {
	Bucket string
	Key    string
	Exists string
	Error  string
}

func writePlan(cmd *cobra.Command, workflow string, plan any, defaultPath string) string {
	out, err := cmd.Flags().GetString("out")
	if err != nil {
		logger.DefaultLogger().Error("Unable to read workflow plan output path", "error", err)
		os.Exit(1)
	}
	if strings.TrimSpace(out) == "" {
		out = defaultPath
	}
	planContent, err := json.Marshal(plan)
	if err != nil {
		logger.DefaultLogger().Error("Unable to marshal workflow plan", "error", err)
		os.Exit(1)
	}
	content, err := json.MarshalIndent(savedPlan{Workflow: workflow, Plan: planContent}, "", "  ")
	if err != nil {
		logger.DefaultLogger().Error("Unable to marshal workflow plan", "error", err)
		os.Exit(1)
	}
	content = append(content, '\n')
	if err := os.WriteFile(out, content, 0o644); err != nil {
		logger.DefaultLogger().Error("Unable to write workflow plan", "error", err, "file", out)
		os.Exit(1)
	}
	fmt.Printf("Wrote plan: %s\n", out)
	return out
}

func defaultPlanPath(workflow, specFile string) string {
	dir := "."
	if strings.TrimSpace(specFile) != "" {
		dir = filepath.Dir(specFile)
	}
	return filepath.Join(dir, workflow+".plan.json")
}

func mustGetStringFlag(cmd *cobra.Command, name string) string {
	value, err := cmd.Flags().GetString(name)
	if err != nil {
		logger.DefaultLogger().Error("Unable to read flag", "flag", name, "error", err)
		os.Exit(1)
	}
	return value
}

func loadSavedPlan(path string) (*savedPlan, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var saved savedPlan
	if err := json.Unmarshal(content, &saved); err != nil {
		return nil, err
	}
	if strings.TrimSpace(saved.Workflow) == "" || len(saved.Plan) == 0 {
		return nil, errors.New("invalid saved plan: workflow and plan are required")
	}
	return &saved, nil
}

func applySavedPlan(ctx context.Context, path string) error {
	return applySavedPlanWithOptions(ctx, path, applySavedPlanOptions{})
}

func applySavedPlanWithOptions(ctx context.Context, path string, options applySavedPlanOptions) error {
	saved, err := loadSavedPlan(path)
	if err != nil {
		return err
	}
	switch saved.Workflow {
	case "new", "renew":
		var plan newEpisodePlan
		if err := json.Unmarshal(saved.Plan, &plan); err != nil {
			return err
		}
		renew := saved.Workflow == "renew"
		inspector := applyRemoteInspector(nil)
		storage := options.Storage
		if storage == nil && options.CheckRemote {
			var err error
			storage, err = newApplyStorage(ctx, plan.SpecFile, options)
			if err != nil {
				return err
			}
		}
		if storage != nil {
			inspector = storage
		}
		decision, err := buildSavedEpisodeApplyDecision(ctx, &plan, applyPreflightOptions{
			JustMaster: options.JustMaster,
			Yes:        options.Yes,
			Force:      options.Force,
			Reencode:   options.Reencode,
		}, inspector, renew)
		if err != nil {
			return err
		}
		writeWorkflowDecision(os.Stdout, decision)
		if inspector != nil {
			rechecked, err := buildSavedEpisodeApplyDecision(ctx, &plan, applyPreflightOptions{
				JustMaster: options.JustMaster,
				Yes:        options.Yes,
				Force:      options.Force,
				Reencode:   options.Reencode,
			}, inspector, renew)
			if err != nil {
				return err
			}
			if !sameWorkflowDecision(decision, rechecked) {
				return errors.New("apply preflight changed before execution; rerun mkpod apply")
			}
		}
		if options.JustMaster {
			return applyNewEpisodeJustMaster(ctx, &plan, decision, options, storage, renew)
		}
		return applyNewEpisodeFull(ctx, &plan, decision, options, storage, renew)
	default:
		return fmt.Errorf("saved plan workflow %q is not supported by apply; use mkpod new or mkpod renew to create apply plans", saved.Workflow)
	}
}

func buildSavedEpisodeApplyDecision(ctx context.Context, plan *newEpisodePlan, options applyPreflightOptions, inspector applyRemoteInspector, renew bool) (workflow.Decision, error) {
	if renew {
		return buildRenewEpisodeApplyDecision(ctx, plan, options, inspector)
	}
	return buildNewEpisodeApplyDecision(ctx, plan, options, inspector)
}

func applySavedPlanOptionsFromCommand(cmd *cobra.Command) (applySavedPlanOptions, error) {
	justMaster, err := cmd.Flags().GetBool("just-master")
	if err != nil {
		return applySavedPlanOptions{}, err
	}
	yes, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return applySavedPlanOptions{}, err
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return applySavedPlanOptions{}, err
	}
	reencode, err := cmd.Flags().GetBool("reencode")
	if err != nil {
		return applySavedPlanOptions{}, err
	}
	if justMaster && reencode {
		return applySavedPlanOptions{}, fmt.Errorf("--reencode cannot be combined with --just-master")
	}
	return applySavedPlanOptions{JustMaster: justMaster, Yes: yes, Force: force, Reencode: reencode, CheckRemote: true}, nil
}

func inspectSavedPlan(path string) error {
	saved, err := loadSavedPlan(path)
	if err != nil {
		return err
	}
	switch saved.Workflow {
	case "blender":
		var plan blenderaddon.Plan
		if err := json.Unmarshal(saved.Plan, &plan); err != nil {
			return err
		}
		printBlenderPlan(&plan)
	case "episode":
		var plan episodeWorkflowPlan
		if err := json.Unmarshal(saved.Plan, &plan); err != nil {
			return err
		}
		printEpisodePlan(&plan)
	case "feed":
		var plan feedWorkflowPlan
		if err := json.Unmarshal(saved.Plan, &plan); err != nil {
			return err
		}
		printFeedPlan(&plan)
	case "new", "renew":
		var plan newEpisodePlan
		if err := json.Unmarshal(saved.Plan, &plan); err != nil {
			return err
		}
		printEpisodeMutationPlan(saved.Workflow, &plan)
		decision, err := buildSavedEpisodeApplyDecision(context.Background(), &plan, applyPreflightOptions{}, nil, saved.Workflow == "renew")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout)
		writeWorkflowDecision(os.Stdout, decision)
	case "preprocess":
		var plan preprocess.Plan
		if err := json.Unmarshal(saved.Plan, &plan); err != nil {
			return err
		}
		printPreprocessPlan(&plan)
	default:
		return fmt.Errorf("saved plan workflow %q is not inspectable", saved.Workflow)
	}
	return nil
}

func validateSavedEpisodePlan(ctx context.Context, plan *episodeWorkflowPlan) error {
	if strings.TrimSpace(plan.SpecFile) == "" {
		return errors.New("stale or invalid episode plan: specFile is required")
	}
	expected, err := buildEpisodePlanFromOptions(ctx, plan.SpecFile, plan.RemotePreview, plan.RemoveRemoteMaster, plan.UID)
	if err != nil {
		return err
	}
	if err := requireSamePlan(expected, plan, "episode"); err != nil {
		return err
	}
	return nil
}

func validateSavedFeedPlan(ctx context.Context, plan *feedWorkflowPlan) error {
	if strings.TrimSpace(plan.SpecFile) == "" {
		return errors.New("stale or invalid feed plan: specFile is required")
	}
	expected, err := buildFeedPlanFromOptions(ctx, plan.SpecFile, plan.RemotePreview, plan.Upload)
	if err != nil {
		return err
	}
	if err := requireSamePlan(expected, plan, "feed"); err != nil {
		return err
	}
	return nil
}

func validateSavedBlenderPlan(plan *blenderaddon.Plan) error {
	if strings.TrimSpace(plan.BlenderPath) == "" {
		return errors.New("stale or invalid blender plan: blenderPath is required")
	}
	expected, err := blenderaddon.BuildPlan(blenderaddon.Options{
		Blender: plan.BlenderPath,
		Repo:    plan.Repo,
	})
	if err != nil {
		return err
	}
	expected.BlenderTool = plan.BlenderTool
	if err := requireSamePlan(expected, plan, "blender"); err != nil {
		return err
	}
	return nil
}

func requireSamePlan(expected, actual any, workflow string) error {
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return err
	}
	if string(expectedJSON) != string(actualJSON) {
		return fmt.Errorf("saved %s plan is stale; recreate the saved plan", workflow)
	}
	return nil
}

func validateSavedPreprocessPlan(plan *preprocess.Plan) error {
	if len(plan.Operations) == 0 {
		return errors.New("stale or invalid preprocess plan: no operations")
	}
	expected, err := preprocess.New(&preprocess.Config{
		Prefix: plan.Prefix,
		Preset: plan.Preset,
		Tool:   plan.Operations[0].Tool,
	}).Plan(preprocessPlanInputs(plan))
	if err != nil {
		return err
	}
	if err := requireSamePlan(expected, plan, "preprocess"); err != nil {
		return err
	}
	return nil
}

func preprocessPlanInputs(plan *preprocess.Plan) []string {
	inputs := make([]string, 0, len(plan.Operations))
	for _, operation := range plan.Operations {
		inputs = append(inputs, operation.Input)
	}
	return inputs
}

func buildEpisodePlan(ctx context.Context, cmd *cobra.Command, uidString string) (*episodeWorkflowPlan, error) {
	specFile, err := cmd.Flags().GetString("spec")
	if err != nil {
		return nil, err
	}
	remotePreview, err := cmd.Flags().GetBool("remote")
	if err != nil {
		return nil, err
	}
	removeRemoteMaster, err := cmd.Flags().GetBool("remove-remote-master")
	if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseInt(uidString, 10, 64)
	if err != nil {
		return nil, err
	}
	return buildEpisodePlanFromOptions(ctx, specFile, remotePreview, removeRemoteMaster, uid)
}

func buildEpisodePlanFromOptions(ctx context.Context, specFile string, remotePreview, removeRemoteMaster bool, uid int64) (*episodeWorkflowPlan, error) {
	atom, err := spec.New(specFile).Load(ctx)
	if err != nil {
		return nil, err
	}
	index := atom.ContainsEpisode(uid)
	if index < 0 {
		return nil, fmt.Errorf("episode UID %d does not exist in podcast specification", uid)
	}

	episode := atom.Episodes[index]
	if err := spec.ApplyEpisodeDefaultsForEncoding(atom, &episode); err != nil {
		return nil, err
	}

	inputPath := filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(episode.Input))
	inputContentType, err := encode.GetFileContentType(inputPath)
	if err != nil {
		return nil, err
	}
	encodingPlan, err := encode.PlanEpisode(atom, &episode, inputContentType)
	if err != nil {
		return nil, err
	}

	outputPath := filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(encodingPlan.Output))
	_, statErr := os.Stat(outputPath)
	outputExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("check output file %s: %w", outputPath, statErr)
	}
	willEncode := !outputExists
	encodeReason := "local output is missing"
	if outputExists {
		encodeReason = "local output exists; apply episode would prompt before re-encoding"
	}
	plannedEpisode := episode
	plannedEpisode.Output = encodingPlan.Output
	if plannedEpisode.Type == "" {
		plannedEpisode.Type = plannedContentType(encodingPlan.Output)
	}
	if plannedEpisode.Length == 0 {
		plannedEpisode.Length = 1
	}
	if plannedEpisode.Duration.Duration == 0 {
		plannedEpisode.Duration.Duration = 1
	}
	rssMissing := spec.MissingFieldsForRSS(atom, &plannedEpisode)

	feedPath := atom.FeedFilePath()
	_, feedStatErr := os.Stat(feedPath)
	feedExists := feedStatErr == nil
	if feedStatErr != nil && !os.IsNotExist(feedStatErr) {
		return nil, fmt.Errorf("check feed file %s: %w", feedPath, feedStatErr)
	}

	plan := &episodeWorkflowPlan{
		SpecFile:           specFile,
		UID:                uid,
		Title:              episode.Title,
		RemoveRemoteMaster: removeRemoteMaster,
		Input:              episode.Input,
		InputPath:          inputPath,
		InputContentType:   inputContentType,
		Output:             encodingPlan.Output,
		OutputPath:         outputPath,
		OutputExists:       outputExists,
		WillEncode:         willEncode,
		EncodeReason:       encodeReason,
		EncodeMode:         encodingPlan.Mode,
		PreferredFormat:    encodingPlan.Preferred,
		EpisodeFormat:      encodingPlan.EpisodeFormat,
		MetadataWillWrite:  strings.TrimSpace(episode.Output) != encodingPlan.Output,
		FeedFile:           atom.FeedFile,
		FeedPath:           feedPath,
		FeedExists:         feedExists,
		RSSReady:           len(rssMissing) == 0,
		RSSMissingFields:   rssMissing,
		RemotePreview:      remotePreview,
		RemoteOutput: remoteObjectPlan{
			Bucket: atom.Config.Aws.Buckets.Output,
			Key:    encodingPlan.Output,
			Exists: "skipped",
		},
		RemoteFeed: remoteObjectPlan{
			Bucket: atom.Config.Aws.Buckets.Output,
			Key:    atom.FeedFile,
			Exists: "skipped",
		},
	}
	if remotePreview {
		storageClient := s3store.New(atom, prompt.New(true, false))
		fillRemoteObjectPlan(ctx, storageClient, &plan.RemoteOutput)
		fillRemoteObjectPlan(ctx, storageClient, &plan.RemoteFeed)
		operations, err := planEpisodeOperations(ctx, atom, &plannedEpisode, storageClient, outputExists, willEncode, removeRemoteMaster)
		if err != nil {
			return nil, err
		}
		plan.Operations = operations
	}
	return plan, nil
}

func planEpisodeOperations(ctx context.Context, atom *model.Podcast, episode *model.Episode, storageClient interface {
	assetInfoClient
	outputExistenceClient
	remoteMasterInfoClient
}, outputExists, willEncode, removeRemoteMaster bool) ([]workflowOperation, error) {
	operations := make([]workflowOperation, 0, 5)
	buckets := []string{atom.Config.Aws.Buckets.Input, atom.Config.Aws.Buckets.Output}
	for _, asset := range []struct {
		key   string
		label string
	}{
		{key: episode.Input, label: fmt.Sprintf("episode %d input", episode.UID)},
		{key: atom.Encoding.Coverfront, label: "cover image"},
		{key: spec.EffectiveEpisodeImage(atom, episode), label: fmt.Sprintf("episode %d image", episode.UID)},
	} {
		operation, err := decideLocalAssetSync(ctx, atom, storageClient, buckets, asset.key, asset.label)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	outputOperation, err := decidePlannedOutputUpload(ctx, atom, episode, storageClient, outputExists, willEncode)
	if err != nil {
		return nil, err
	}
	operations = append(operations, outputOperation)
	if removeRemoteMaster {
		operation, err := decideRemoteMasterRemoval(ctx, atom, episode, storageClient)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return operations, nil
}

func buildFeedPlan(ctx context.Context, cmd *cobra.Command, args []string) (*feedWorkflowPlan, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("feed does not take positional arguments")
	}
	specFile, err := cmd.Flags().GetString("spec")
	if err != nil {
		return nil, err
	}
	remotePreview, err := cmd.Flags().GetBool("remote")
	if err != nil {
		return nil, err
	}
	upload, err := cmd.Flags().GetBool("upload")
	if err != nil {
		return nil, err
	}
	return buildFeedPlanFromOptions(ctx, specFile, remotePreview, upload)
}

func buildFeedPlanFromOptions(ctx context.Context, specFile string, remotePreview, upload bool) (*feedWorkflowPlan, error) {
	atom, err := spec.New(specFile).Load(ctx)
	if err != nil {
		return nil, err
	}

	validEpisodes := 0
	skippedEpisodes := 0
	for i := range atom.Episodes {
		if len(spec.MissingFieldsForRSS(atom, &atom.Episodes[i])) == 0 {
			validEpisodes++
		} else {
			skippedEpisodes++
		}
	}

	feedPath := atom.FeedFilePath()
	_, statErr := os.Stat(feedPath)
	feedExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("check feed file %s: %w", feedPath, statErr)
	}

	plan := &feedWorkflowPlan{
		SpecFile:        specFile,
		FeedFile:        atom.FeedFile,
		FeedPath:        feedPath,
		FeedExists:      feedExists,
		OutputBucket:    atom.Config.Aws.Buckets.Output,
		ValidEpisodes:   validEpisodes,
		SkippedEpisodes: skippedEpisodes,
		Upload:          upload,
		RemotePreview:   remotePreview,
		RemoteFeed: remoteObjectPlan{
			Bucket: atom.Config.Aws.Buckets.Output,
			Key:    atom.FeedFile,
			Exists: "skipped",
		},
	}
	if remotePreview {
		storageClient := s3store.New(atom, prompt.New(true, false))
		fillRemoteObjectPlan(ctx, storageClient, &plan.RemoteFeed)
		if upload {
			operations, err := planFeedOperations(ctx, atom, feedPath, storageClient)
			if err != nil {
				return nil, err
			}
			plan.Operations = operations
		}
	} else if upload {
		operations, err := planFeedOperations(ctx, atom, feedPath, nil)
		if err != nil {
			return nil, err
		}
		plan.Operations = operations
	}
	return plan, nil
}

type remoteExistenceChecker interface {
	FileExists(context.Context, string, string) (bool, error)
}

func fillRemoteObjectPlan(ctx context.Context, storageClient remoteExistenceChecker, remotePlan *remoteObjectPlan) {
	exists, err := storageClient.FileExists(ctx, remotePlan.Bucket, remotePlan.Key)
	if err != nil {
		remotePlan.Exists = "unknown"
		remotePlan.Error = err.Error()
		return
	}
	if exists {
		remotePlan.Exists = "true"
		return
	}
	remotePlan.Exists = "false"
}

func plannedContentType(output string) string {
	switch strings.ToLower(filepath.Ext(output)) {
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".m4b":
		return "audio/mp4"
	case ".mp4":
		return "video/mp4"
	default:
		return ""
	}
}

func printBlenderPlan(plan *blenderaddon.Plan) {
	fmt.Println("Workflow: blender")
	fmt.Printf("Blender: %s\n", plan.BlenderPath)
	fmt.Printf("Repository: %s\n", plan.Repo)
	fmt.Printf("Add-on: %s (%s)\n", plan.AddonName, plan.AddonModule)
	fmt.Println("Operations:")
	for _, action := range plan.Actions {
		fmt.Printf("- %s\n", action)
	}
}

func printPreprocessPlan(plan *preprocess.Plan) {
	fmt.Println("Workflow: preprocess")
	fmt.Printf("Preset: %s\n", plan.Preset)
	fmt.Printf("Prefix: %s\n", plan.Prefix)
	fmt.Printf("Filter: %s\n", plan.Filter)
	fmt.Println("Operations:")
	for _, operation := range plan.Operations {
		fmt.Printf("- %s -> %s\n", operation.Input, operation.Output)
		fmt.Printf("  Tool: %s\n", operation.Tool)
		fmt.Printf("  Args: %v\n", operation.Args)
	}
}

func printEpisodePlan(plan *episodeWorkflowPlan) {
	fmt.Println("Workflow: episode")
	fmt.Printf("Episode: %d\n", plan.UID)
	fmt.Printf("Title: %s\n", plan.Title)
	fmt.Printf("Input: %s\n", plan.Input)
	fmt.Printf("Input path: %s\n", plan.InputPath)
	fmt.Printf("Input content type: %s\n", plan.InputContentType)
	fmt.Printf("Encode mode: %s\n", plan.EncodeMode)
	fmt.Printf("Preferred format: %s\n", plan.PreferredFormat)
	fmt.Printf("Episode format: %s\n", plan.EpisodeFormat)
	fmt.Printf("Output: %s\n", plan.Output)
	fmt.Printf("Output path: %s\n", plan.OutputPath)
	fmt.Printf("Output exists: %t\n", plan.OutputExists)
	fmt.Printf("Will encode: %t\n", plan.WillEncode)
	fmt.Printf("Encode reason: %s\n", plan.EncodeReason)
	fmt.Printf("Podspec metadata update: %t\n", plan.MetadataWillWrite)
	fmt.Printf("Feed: %s\n", plan.FeedFile)
	fmt.Printf("Feed path: %s\n", plan.FeedPath)
	fmt.Printf("Feed exists: %t\n", plan.FeedExists)
	fmt.Printf("RSS ready after apply: %t\n", plan.RSSReady)
	if len(plan.RSSMissingFields) > 0 {
		fmt.Printf("RSS missing fields after apply: %s\n", strings.Join(plan.RSSMissingFields, ", "))
	}
	fmt.Printf("Remote preview: %t\n", plan.RemotePreview)
	printRemoteObjectPlan("Remote output", plan.RemoteOutput)
	printRemoteObjectPlan("Remote feed", plan.RemoteFeed)
	printWorkflowOperations(plan.Operations)
}

func printFeedPlan(plan *feedWorkflowPlan) {
	fmt.Println("Workflow: feed")
	fmt.Printf("Feed: %s\n", plan.FeedFile)
	fmt.Printf("Feed path: %s\n", plan.FeedPath)
	fmt.Printf("Feed exists: %t\n", plan.FeedExists)
	fmt.Printf("Output bucket: %s\n", plan.OutputBucket)
	fmt.Printf("Valid episodes: %d\n", plan.ValidEpisodes)
	fmt.Printf("Skipped episodes: %d\n", plan.SkippedEpisodes)
	fmt.Printf("Upload: %t\n", plan.Upload)
	fmt.Printf("Remote preview: %t\n", plan.RemotePreview)
	printRemoteObjectPlan("Remote feed", plan.RemoteFeed)
	printWorkflowOperations(plan.Operations)
}

func printWorkflowOperations(operations []workflowOperation) {
	if len(operations) == 0 {
		return
	}
	fmt.Println("Operations:")
	for _, operation := range operations {
		fmt.Printf("- %s: %s\n", operation.Kind, operation.Reason)
		if operation.Bucket != "" && operation.Key != "" {
			fmt.Printf("  Remote: s3://%s/%s\n", operation.Bucket, operation.Key)
		}
		if operation.LocalPath != "" {
			fmt.Printf("  Local: %s\n", operation.LocalPath)
		}
		if operation.RequiresPrompt {
			fmt.Println("  Prompt: required")
		}
		if operation.SafetyStatus != "" {
			fmt.Printf("  Safety: %s\n", operation.SafetyStatus)
		}
	}
}

func printRemoteObjectPlan(label string, plan remoteObjectPlan) {
	fmt.Printf("%s: s3://%s/%s\n", label, plan.Bucket, plan.Key)
	fmt.Printf("%s exists: %s\n", label, plan.Exists)
	if plan.Error != "" {
		fmt.Printf("%s error: %s\n", label, plan.Error)
	}
}

func init() {
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(inspectCmd)

	applyCmd.Flags().Bool("just-master", false, "Apply only podspec metadata, master media sync, and encode-time artifact sync")
	applyCmd.Flags().Bool("reencode", false, "Regenerate local production audio from the master before syncing output")
	applyCmd.Flags().BoolP("yes", "y", false, "Answer yes to non-destructive upfront apply decisions")
	applyCmd.Flags().BoolP("force", "f", false, "Force overwrite decisions; local source files must already exist")

}

func addPlanOutputFlag(cmd *cobra.Command) {
	cmd.Flags().String("out", "", "Write the generated workflow plan to a JSON file")
}

func addPreprocessWorkflowFlags(cmd *cobra.Command) {
	cmd.Flags().String("prefix", defaultPreProcessingPrefix, "Prefix to prepend to each generated output filename")
	cmd.Flags().StringP("preset", "p", defaultPreset, "Preprocessing preset to apply. Available: "+availablePreprocessPresets)
	cmd.Flags().String("ffmpeg", "", "ffmpeg executable path or name; defaults to ffmpeg on PATH")
}

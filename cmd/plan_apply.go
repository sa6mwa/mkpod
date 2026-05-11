package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sa6mwa/mkpod/internal/blenderaddon"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/media/encode"
	"github.com/sa6mwa/mkpod/internal/media/preprocess"
	"github.com/sa6mwa/mkpod/internal/prompt"
	"github.com/sa6mwa/mkpod/internal/spec"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan <workflow>",
	Short: "Preview mkpod workflow operations",
}

var applyCmd = &cobra.Command{
	Use:   "apply <workflow>",
	Short: "Execute mkpod workflow operations",
}

var planBlenderCmd = &cobra.Command{
	Use:   "blender",
	Short: "Preview Blender marker exporter installation",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		blender, err := cmd.Flags().GetString("blender")
		if err != nil {
			l.Error("Internal error", "error", err)
			os.Exit(1)
		}
		repo, err := cmd.Flags().GetString("repo")
		if err != nil {
			l.Error("Internal error", "error", err)
			os.Exit(1)
		}

		plan, err := blenderaddon.BuildPlan(blenderaddon.Options{Blender: blender, Repo: repo})
		if err != nil {
			l.Error("Unable to plan Blender marker exporter installation", "error", err)
			os.Exit(1)
		}

		printBlenderPlan(plan)
	},
}

var applyBlenderCmd = &cobra.Command{
	Use:   "blender",
	Short: "Install the Blender marker exporter add-on",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		blender, err := cmd.Flags().GetString("blender")
		if err != nil {
			l.Error("Internal error", "error", err)
			os.Exit(1)
		}
		repo, err := cmd.Flags().GetString("repo")
		if err != nil {
			l.Error("Internal error", "error", err)
			os.Exit(1)
		}

		plan, err := blenderaddon.Apply(context.Background(), blenderaddon.Options{Blender: blender, Repo: repo}, nil)
		if err != nil {
			l.Error("Unable to install Blender marker exporter", "error", err)
			os.Exit(1)
		}

		printBlenderPlan(plan)
		fmt.Println("Installed Blender marker exporter.")
	},
}

var planPreprocessCmd = &cobra.Command{
	Use:     "preprocess [flags] audiofiles...",
	Aliases: []string{"pre"},
	Short:   "Preview audio preprocessing operations",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		plan, err := buildPreprocessPlan(cmd, args)
		if err != nil {
			l.Error("Unable to plan preprocessing", "error", err)
			os.Exit(1)
		}
		printPreprocessPlan(plan)
	},
}

var applyPreprocessCmd = &cobra.Command{
	Use:     "preprocess [flags] audiofiles...",
	Aliases: []string{"pre"},
	Short:   "Execute audio preprocessing operations",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		plan, err := buildPreprocessPlan(cmd, args)
		if err != nil {
			l.Error("Unable to plan preprocessing", "error", err)
			os.Exit(1)
		}
		if err := preprocess.ExecutePlan(context.Background(), plan); err != nil {
			l.Error("Unable to apply preprocessing", "error", err)
			os.Exit(1)
		}
		printPreprocessPlan(plan)
		fmt.Println("Applied preprocessing.")
	},
}

var planEpisodeCmd = &cobra.Command{
	Use:   "episode <uid>",
	Short: "Preview episode encoding workflow",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("provide exactly one episode UID")
		}
		_, err := strconv.ParseInt(args[0], 10, 64)
		return err
	},
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		plan, err := buildEpisodePlan(context.Background(), cmd, args[0])
		if err != nil {
			l.Error("Unable to plan episode workflow", "error", err)
			os.Exit(1)
		}
		printEpisodePlan(plan)
	},
}

var applyEpisodeCmd = &cobra.Command{
	Use:   "episode <uid>",
	Short: "Execute episode encoding workflow",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("provide exactly one episode UID")
		}
		_, err := strconv.ParseInt(args[0], 10, 64)
		return err
	},
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		options, err := encodeWorkflowOptionsFromFlags(cmd)
		if err != nil {
			l.Error("Internal error", "error", err)
			os.Exit(1)
		}
		options.All = false
		if err := runEncodeWorkflow(logger.WithDefaultLogger(context.Background()), args, options); err != nil {
			l.Error("Unable to apply episode workflow", "error", err)
			os.Exit(1)
		}
	},
}

type episodeWorkflowPlan struct {
	UID               int64
	Title             string
	Input             string
	InputPath         string
	InputContentType  string
	Output            string
	OutputPath        string
	OutputExists      bool
	WillEncode        bool
	EncodeReason      string
	EncodeMode        string
	PreferredFormat   string
	EpisodeFormat     string
	MetadataWillWrite bool
	FeedFile          string
	FeedPath          string
	FeedExists        bool
	RSSReady          bool
	RSSMissingFields  []string
	RemotePreview     bool
	RemoteOutput      remoteObjectPlan
	RemoteFeed        remoteObjectPlan
}

type remoteObjectPlan struct {
	Bucket string
	Key    string
	Exists string
	Error  string
}

func buildPreprocessPlan(cmd *cobra.Command, args []string) (*preprocess.Plan, error) {
	prefix, err := cmd.Flags().GetString("prefix")
	if err != nil {
		return nil, err
	}
	preset, err := cmd.Flags().GetString("preset")
	if err != nil {
		return nil, err
	}
	tool, err := cmd.Flags().GetString("ffmpeg")
	if err != nil {
		return nil, err
	}

	processor := preprocess.New(&preprocess.Config{
		Prefix: prefix,
		Preset: preset,
		Tool:   tool,
	})
	return processor.Plan(args)
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
	uid, err := strconv.ParseInt(uidString, 10, 64)
	if err != nil {
		return nil, err
	}

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
		UID:               uid,
		Title:             episode.Title,
		Input:             episode.Input,
		InputPath:         inputPath,
		InputContentType:  inputContentType,
		Output:            encodingPlan.Output,
		OutputPath:        outputPath,
		OutputExists:      outputExists,
		WillEncode:        willEncode,
		EncodeReason:      encodeReason,
		EncodeMode:        encodingPlan.Mode,
		PreferredFormat:   encodingPlan.Preferred,
		EpisodeFormat:     encodingPlan.EpisodeFormat,
		MetadataWillWrite: strings.TrimSpace(episode.Output) != encodingPlan.Output,
		FeedFile:          atom.FeedFile,
		FeedPath:          feedPath,
		FeedExists:        feedExists,
		RSSReady:          len(rssMissing) == 0,
		RSSMissingFields:  rssMissing,
		RemotePreview:     remotePreview,
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
}

func printRemoteObjectPlan(label string, plan remoteObjectPlan) {
	fmt.Printf("%s: s3://%s/%s\n", label, plan.Bucket, plan.Key)
	fmt.Printf("%s exists: %s\n", label, plan.Exists)
	if plan.Error != "" {
		fmt.Printf("%s error: %s\n", label, plan.Error)
	}
}

func init() {
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)

	planCmd.AddCommand(planBlenderCmd)
	applyCmd.AddCommand(applyBlenderCmd)
	planCmd.AddCommand(planPreprocessCmd)
	applyCmd.AddCommand(applyPreprocessCmd)
	planCmd.AddCommand(planEpisodeCmd)
	applyCmd.AddCommand(applyEpisodeCmd)

	planBlenderCmd.Flags().String("blender", "", "Blender executable path or name; defaults to blender on PATH")
	planBlenderCmd.Flags().String("repo", "", "Blender extension repository identifier; defaults to user_default")
	applyBlenderCmd.Flags().String("blender", "", "Blender executable path or name; defaults to blender on PATH")
	applyBlenderCmd.Flags().String("repo", "", "Blender extension repository identifier; defaults to user_default")

	addPreprocessWorkflowFlags(planPreprocessCmd)
	addPreprocessWorkflowFlags(applyPreprocessCmd)

	planEpisodeCmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file")
	planEpisodeCmd.Flags().Bool("remote", false, "Perform read-only S3 checks for planned remote objects")
	applyEpisodeCmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file")
	applyEpisodeCmd.Flags().BoolP("force", "f", false, "Do not prompt when applying the episode workflow")
	applyEpisodeCmd.Flags().BoolP("remove-remote-master", "R", false, "Remove remote input master audio or video file after safety checks")
}

func addPreprocessWorkflowFlags(cmd *cobra.Command) {
	cmd.Flags().String("prefix", defaultPreProcessingPrefix, "Prefix to prepend to each generated output filename")
	cmd.Flags().StringP("preset", "p", defaultPreset, "Preprocessing preset to apply. Available: "+availablePreprocessPresets)
	cmd.Flags().String("ffmpeg", "", "ffmpeg executable path or name; defaults to ffmpeg on PATH")
}

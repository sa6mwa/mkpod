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
	"github.com/sa6mwa/mkpod/internal/spec"
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
	EncodeMode        string
	PreferredFormat   string
	EpisodeFormat     string
	MetadataWillWrite bool
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

	return &episodeWorkflowPlan{
		UID:               uid,
		Title:             episode.Title,
		Input:             episode.Input,
		InputPath:         inputPath,
		InputContentType:  inputContentType,
		Output:            encodingPlan.Output,
		OutputPath:        outputPath,
		OutputExists:      outputExists,
		EncodeMode:        encodingPlan.Mode,
		PreferredFormat:   encodingPlan.Preferred,
		EpisodeFormat:     encodingPlan.EpisodeFormat,
		MetadataWillWrite: strings.TrimSpace(episode.Output) != encodingPlan.Output,
	}, nil
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
	fmt.Printf("Podspec metadata update: %t\n", plan.MetadataWillWrite)
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
	applyEpisodeCmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file")
	applyEpisodeCmd.Flags().BoolP("force", "f", false, "Do not prompt when applying the episode workflow")
	applyEpisodeCmd.Flags().BoolP("remove-remote-master", "R", false, "Remove remote input master audio or video file after safety checks")
}

func addPreprocessWorkflowFlags(cmd *cobra.Command) {
	cmd.Flags().String("prefix", defaultPreProcessingPrefix, "Prefix to prepend to each generated output filename")
	cmd.Flags().StringP("preset", "p", defaultPreset, "Preprocessing preset to apply. Available: "+availablePreprocessPresets)
	cmd.Flags().String("ffmpeg", "", "ffmpeg executable path or name; defaults to ffmpeg on PATH")
}

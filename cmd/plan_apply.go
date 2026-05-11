package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sa6mwa/mkpod/internal/blenderaddon"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/media/preprocess"
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

func init() {
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)

	planCmd.AddCommand(planBlenderCmd)
	applyCmd.AddCommand(applyBlenderCmd)
	planCmd.AddCommand(planPreprocessCmd)
	applyCmd.AddCommand(applyPreprocessCmd)

	planBlenderCmd.Flags().String("blender", "", "Blender executable path or name; defaults to blender on PATH")
	planBlenderCmd.Flags().String("repo", "", "Blender extension repository identifier; defaults to user_default")
	applyBlenderCmd.Flags().String("blender", "", "Blender executable path or name; defaults to blender on PATH")
	applyBlenderCmd.Flags().String("repo", "", "Blender extension repository identifier; defaults to user_default")

	addPreprocessWorkflowFlags(planPreprocessCmd)
	addPreprocessWorkflowFlags(applyPreprocessCmd)
}

func addPreprocessWorkflowFlags(cmd *cobra.Command) {
	cmd.Flags().String("prefix", defaultPreProcessingPrefix, "Prefix to prepend to each generated output filename")
	cmd.Flags().StringP("preset", "p", defaultPreset, "Preprocessing preset to apply. Available: "+availablePreprocessPresets)
	cmd.Flags().String("ffmpeg", "", "ffmpeg executable path or name; defaults to ffmpeg on PATH")
}

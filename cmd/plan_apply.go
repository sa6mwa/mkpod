package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sa6mwa/mkpod/internal/blenderaddon"
	"github.com/sa6mwa/mkpod/internal/logging"
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

func init() {
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)

	planCmd.AddCommand(planBlenderCmd)
	applyCmd.AddCommand(applyBlenderCmd)

	planBlenderCmd.Flags().String("blender", "", "Blender executable path or name; defaults to blender on PATH")
	planBlenderCmd.Flags().String("repo", "", "Blender extension repository identifier; defaults to user_default")
	applyBlenderCmd.Flags().String("blender", "", "Blender executable path or name; defaults to blender on PATH")
	applyBlenderCmd.Flags().String("repo", "", "Blender extension repository identifier; defaults to user_default")
}

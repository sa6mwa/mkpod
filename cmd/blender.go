package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sa6mwa/mkpod/internal/blenderaddon"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/spf13/cobra"
)

var blenderCmd = &cobra.Command{
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
		result, err := blenderaddon.Apply(context.Background(), blenderaddon.Options{Blender: blender, Repo: repo}, nil)
		if err != nil {
			l.Error("Unable to install Blender marker exporter", "error", err)
			os.Exit(1)
		}
		fmt.Printf("Installed Blender marker exporter: %s\n", result.AddonName)
	},
}

func init() {
	rootCmd.AddCommand(blenderCmd)
	blenderCmd.Flags().String("blender", "", "Blender executable path or name; defaults to blender on PATH")
	blenderCmd.Flags().String("repo", "", "Blender extension repository identifier; defaults to user_default")
}

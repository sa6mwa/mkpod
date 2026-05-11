/*
Copyright © 2025 SA6MWA Michel
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mkpod",
	Short: "Prepare, encode, and publish a self-hosted podcast to AWS S3",
	Long: `mkpod is a CLI for a spec-driven podcast workflow: initialize a
workspace, encode episode media with host-provided tools, render RSS, and
publish the results to AWS S3.

The emerging workflow is:
  1. mkpod plan <workflow>
  2. mkpod apply <workflow>

Legacy encode/parse/preprocess commands remain available during the
plan/apply migration, but are hidden from the primary help surface.

mkpod preprocess remains available as an optional thin utility for raw
microphone cleanup before editing; it is not required for the main publish
pipeline.`,
	Example: `  mkpod init ./podcast
  mkpod plan preprocess masters/raw.wav
  mkpod apply preprocess masters/raw.wav
  mkpod plan episode --spec ./podcast/podspec.yaml 16
  mkpod apply episode --spec ./podcast/podspec.yaml 16
  mkpod plan feed --spec ./podcast/podspec.yaml
  mkpod apply feed --spec ./podcast/podspec.yaml --upload`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
}

/*
Copyright © 2025 SA6MWA Michel
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"pkt.systems/version"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "mkpod",
	Short:   "Prepare, encode, and publish a self-hosted podcast to AWS S3",
	Version: version.CurrentWithDirty(),
	Long: `mkpod is a CLI for a spec-driven podcast workflow: initialize a
workspace, encode episode media with host-provided tools, render RSS, and
publish the results to AWS S3.

The main workflow is:
  1. mkpod new
  2. mkpod inspect new.plan.json
  3. mkpod apply new.plan.json
  4. mkpod publish

Existing episodes can be repaired or re-encoded by creating a renew plan:
  mkpod renew <uid|all>
  mkpod inspect renew-<uid>.plan.json
  mkpod apply renew-<uid>.plan.json

Legacy encode/parse commands remain available during the cutover, but are
hidden from the primary help surface. mkpod preprocess remains available as an
optional thin utility for raw microphone cleanup before editing.`,
	Example: `  mkpod init ./podcast
  mkpod new
  mkpod inspect new.plan.json
  mkpod apply new.plan.json
  mkpod renew 34
  mkpod apply renew-34.plan.json
  mkpod publish
  mkpod publish --spec ./podcast/podspec.yaml`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print mkpod version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), version.CurrentWithDirty())
	},
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
	version.SetDefaultModule("github.com/sa6mwa/mkpod")
	rootCmd.Version = version.CurrentWithDirty()
	rootCmd.AddCommand(versionCmd)
}

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
	Short: "Generate and encode podcasts and publish to a cloud object store",
	Long: `mkpod is designed to automate the process of preprocessing, encoding,
generating metadata in Apple podcast RSS format (with chapter
information), and publishing a podcast to an object store. AWS S3 is
currently the only supported storage backend.`,
	Example: `  mkpod init ./podcast
  mkpod preprocess masters/raw.wav
  mkpod encode --spec ./podcast/podspec.yaml --all
  mkpod parse --spec ./podcast/podspec.yaml --upload`,

	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
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
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mkpod.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

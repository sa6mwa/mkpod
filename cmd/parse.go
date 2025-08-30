/*
Copyright © 2025 SA6MWA Michel

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	"os"

	"github.com/sa6mwa/mkpod/internal/infra/adapters/configurator"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
	"github.com/spf13/cobra"
)

// parseCmd represents the parse command
var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		imsg := "Internal error"
		specFile, err := cmd.Flags().GetString("spec")
		if err != nil {
			l.Error(imsg, "error", err)
			os.Exit(1)
		}
		askNoQuestions, err := cmd.Flags().GetBool("force")
		if err != nil {
			l.Error(imsg, "error", err)
			os.Exit(1)
		}
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			l.Error(imsg, "error", err)
		}

		if len(args) > 0 {
			l.Error("Syntax error", "error", "this command does not take any arguments")
			os.Exit(1)
		}

		ctx := context.Background()

		config := configurator.New(specFile)

		atom, err := config.Load(ctx)
		if err != nil {
			l.Error("Failed to load configuration", "error", err, "specfile", specFile)
			os.Exit(1)
		}

	},
}

func init() {
	rootCmd.AddCommand(parseCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// parseCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// parseCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	parseCmd.Flags().StringP("spec", "s", configurator.DefaultSpecfile, "Main configuration file for generating the RSS atom")
	parseCmd.Flags().BoolP("upload", "u", false, "Upload podcast.rss to \"output\" Amazon AWS S3 bucket defined in spec file")
	parseCmd.Flags().BoolP("force", "f", false, "Force, do not ask if to proceed with an action, just do it")
	parseCmd.Flags().BoolP("dry-run", "n", false, "Behaves like the force option without modifying or producing anything. Will output RSS to stdout instead of file")
}

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

	"time"

	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/asker"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/configurator"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/parser"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/uploader"
	"github.com/spf13/cobra"
)

// parseCmd represents the parse command
var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse podspec.yaml into podcast RSS feed",
	Long: `Parse the podcast specification file (podspec.yaml) and generate
the RSS feed (podcast.rss). This command reads the configuration,
validates the podcast metadata, and generates the RSS XML file.
Optionally, it can upload the RSS file to the configured S3 bucket.`,
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
			os.Exit(1)
		}
		upload, err := cmd.Flags().GetBool("upload")
		if err != nil {
			l.Error(imsg, "error", err)
			os.Exit(1)
		}

		if len(args) > 0 {
			l.Error("Syntax error", "error", "this command does not take any arguments")
			os.Exit(1)
		}

		ctx := context.Background()
		ctx = logger.WithDefaultLogger(ctx)

		// Load configuration
		config := configurator.New(specFile)
		atom, err := config.Load(ctx)
		if err != nil {
			l.Error("Failed to load configuration", "error", err, "specfile", specFile)
			os.Exit(1)
		}

		if upload {
			l.Info("About to generate RSS and upload to S3", "atom", atom.Atom, "bucket", atom.Config.Aws.Buckets.Output)
		} else {
			l.Info("About to generate RSS", "atom", atom.Atom)
		}

		// Create adapters
		askerAdapter := asker.New(dryRun, askNoQuestions)
		parserAdapter := parser.New()

		// Ask if user wants to refresh lastBuildDate
		if askerAdapter.Ask(ctx, "Refresh lastBuildDate (will update %s and optionally %s)?", atom.Atom, specFile) {
			atom.LastBuildDate.Time = time.Now().UTC()
			
			// Save updated configuration
			if askerAdapter.Ask(ctx, "Fields in the atom have changed, re-write %s?", specFile) {
				if err := config.Save(ctx, atom); err != nil {
					l.Error("Unable to save configuration", "error", err, "specfile", specFile)
					os.Exit(1)
				}
			}
		}

		// Generate RSS
		if dryRun {
			if err := parserAdapter.WriteRSSToStdout(ctx, atom); err != nil {
				l.Error("Failed to write RSS to stdout", "error", err)
				os.Exit(1)
			}
		} else {
			if err := parserAdapter.WriteRSS(ctx, atom); err != nil {
				l.Error("Failed to write RSS file", "error", err, "file", atom.Atom)
				os.Exit(1)
			}
			l.Info("Successfully generated RSS", "file", atom.Atom)
		}

		// Upload if requested
		if upload && !dryRun {
			uploaderAdapter := uploader.New(atom)
			
			// Show diff first
			if err := uploaderAdapter.Diff(ctx, atom.Config.Aws.Buckets.Output, atom.Atom, atom.Atom); err != nil {
				l.Error("Failed to show diff", "error", err)
				// Don't exit on diff error, continue with upload
			}

			if askerAdapter.Ask(ctx, "Upload new %s?", atom.Atom) {
				request := &ports.ForUploadingRequest{
					Store:       atom.Config.Aws.Buckets.Output,
					To:          atom.Atom,
					From:        atom.Atom,
					ContentType: "text/xml",
				}
				if err := uploaderAdapter.Upload(ctx, request, nil); err != nil {
					l.Error("Failed to upload RSS", "error", err)
					os.Exit(1)
				}
			}
		} else if upload && dryRun {
			l.Info("Dry run: would upload RSS", "file", atom.Atom, "bucket", atom.Config.Aws.Buckets.Output)
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

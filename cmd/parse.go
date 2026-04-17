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
	"strings"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/prompt"
	"github.com/sa6mwa/mkpod/internal/rss"
	"github.com/sa6mwa/mkpod/internal/spec"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	"github.com/spf13/cobra"
)

// parseCmd represents the parse command
var parseCmd = &cobra.Command{
	Use:     "parse",
	Aliases: []string{"p"},
	Short:   "Parse podspec.yaml into podcast RSS feed",
	Long: `Parse the podcast specification file (podspec.yaml) and generate
the RSS feed (podcast.rss). This command reads the configuration,
validates the podcast metadata, and generates the RSS XML file. Episodes
that are still missing required RSS fields are skipped with warnings so
the feed can still be rendered from valid episodes.
Optionally, it can upload the RSS file to the configured S3 bucket.`,
	Example: `  mkpod parse
  mkpod parse --spec ./podcast/podspec.yaml
  mkpod parse --spec ./podcast/podspec.yaml --dry-run
  mkpod parse --spec ./podcast/podspec.yaml --upload`,
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
			l.Error("Syntax error", "error", "parse does not take positional arguments")
			os.Exit(1)
		}

		ctx := context.Background()
		ctx = logger.WithDefaultLogger(ctx)

		// Load configuration
		config := spec.New(specFile)
		atom, err := config.Load(ctx)
		if err != nil {
			l.Error("Failed to load podcast specification", "error", err, "specfile", specFile)
			os.Exit(1)
		}

		if upload {
			l.Info("About to generate RSS and upload to S3", "feed", atom.FeedFile, "bucket", atom.Config.Aws.Buckets.Output)
		} else {
			l.Info("About to generate RSS", "feed", atom.FeedFile)
		}

		feedPath := atom.FeedFilePath()

		// Create services
		prompter := prompt.New(dryRun, askNoQuestions)
		renderer := rss.New()

		// Ask if user wants to refresh lastBuildDate
		if prompter.Ask(ctx, "Refresh lastBuildDate (will update %s and optionally %s)?", feedPath, specFile) {
			atom.LastBuildDate.Time = time.Now().UTC()

			// Save updated configuration
			if prompter.Ask(ctx, "Podcast metadata changed, rewrite %s?", specFile) {
				if err := config.Save(ctx, atom); err != nil {
					l.Error("Unable to save configuration", "error", err, "specfile", specFile)
					os.Exit(1)
				}
			}
		}

		// Generate RSS
		if dryRun {
			if err := renderer.WriteRSSToStdout(ctx, atom); err != nil {
				l.Error("Failed to write RSS to stdout", "error", err)
				os.Exit(1)
			}
		} else {
			if err := renderer.WriteRSS(ctx, atom); err != nil {
				l.Error("Failed to write RSS file", "error", err, "file", feedPath)
				os.Exit(1)
			}
			l.Info("Successfully generated RSS", "file", feedPath)
		}

		// Upload if requested
		if upload && !dryRun {
			storageClient := s3store.New(atom, prompter)

			// Ensure every referenced image can be published from local state or remote fallback.
			if err := checkAndUploadPodcastImage(ctx, atom, prompter, storageClient); err != nil {
				l.Error("Failed to sync required podcast images", "error", err)
				os.Exit(1)
			}

			// Show diff first
			if err := storageClient.DiffTextObject(ctx, atom.Config.Aws.Buckets.Output, atom.FeedFile, feedPath); err != nil {
				l.Error("Failed to show diff", "error", err)
				// Don't exit on diff error, continue with upload
			}

			if prompter.Ask(ctx, "Upload new %s?", atom.FeedFile) {
				options := &s3store.UploadOptions{ContentType: "text/xml"}
				if err := storageClient.UploadFile(ctx, atom.Config.Aws.Buckets.Output, atom.FeedFile, feedPath, options); err != nil {
					l.Error("Failed to upload RSS", "error", err)
					os.Exit(1)
				}
			}
		} else if upload && dryRun {
			l.Info("Dry run: would upload RSS", "file", feedPath, "bucket", atom.Config.Aws.Buckets.Output)
			// In dry run, also show what images would be checked/uploaded
			if err := checkAndUploadPodcastImage(ctx, atom, prompter, nil); err != nil {
				l.Warn("Failed to check podcast image (dry run)", "error", err)
			}
		}
	},
}

// checkAndUploadPodcastImage keeps publishable images in sync with local-first semantics.
func checkAndUploadPodcastImage(ctx context.Context, atom *model.Podcast, askerAdapter interface {
	Ask(context.Context, string, ...any) bool
}, uploaderAdapter interface {
	GetFileInfo(context.Context, string, string) (*s3store.FileInfo, error)
	DownloadFile(context.Context, string, string) error
	UploadFile(context.Context, string, string, string, *s3store.UploadOptions) error
}) error {
	return syncReferencedImagesForPublish(ctx, atom, askerAdapter, uploaderAdapter)
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

	parseCmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file (podspec.yaml)")
	parseCmd.Flags().BoolP("upload", "u", false, "Upload podcast.rss to \"output\" Amazon AWS S3 bucket defined in spec file")
	parseCmd.Flags().BoolP("force", "f", false, "Do not prompt before rewriting metadata, uploading RSS, or uploading missing images")
	parseCmd.Flags().BoolP("dry-run", "n", false, "Behaves like the force option without modifying or producing anything. Will output RSS to stdout instead of file")
}

func joinBaseURLPath(baseURL, relPath string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	relPath = strings.TrimLeft(relPath, "/")
	if baseURL == "" {
		return relPath
	}
	if relPath == "" {
		return baseURL
	}
	return baseURL + "/" + relPath
}

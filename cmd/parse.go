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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/asker"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
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
			l.Error("Failed to load podcast spec", "error", err, "specfile", specFile)
			os.Exit(1)
		}

		if upload {
			l.Info("About to generate RSS and upload to S3", "feed", atom.FeedFile, "bucket", atom.Config.Aws.Buckets.Output)
		} else {
			l.Info("About to generate RSS", "feed", atom.FeedFile)
		}

		feedPath := atom.FeedFilePath()

		// Create adapters
		askerAdapter := asker.New(dryRun, askNoQuestions)
		parserAdapter := rss.New()

		// Ask if user wants to refresh lastBuildDate
		if askerAdapter.Ask(ctx, "Refresh lastBuildDate (will update %s and optionally %s)?", feedPath, specFile) {
			atom.LastBuildDate.Time = time.Now().UTC()

			// Save updated configuration
			if askerAdapter.Ask(ctx, "Podcast metadata changed, rewrite %s?", specFile) {
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
				l.Error("Failed to write RSS file", "error", err, "file", feedPath)
				os.Exit(1)
			}
			l.Info("Successfully generated RSS", "file", feedPath)
		}

		// Upload if requested
		if upload && !dryRun {
			storageClient := s3store.New(atom, askerAdapter)

			// Check and upload podcast image if needed
			if err := checkAndUploadPodcastImage(ctx, atom, askerAdapter, storageClient); err != nil {
				l.Warn("Failed to check/upload podcast image", "error", err)
				// Don't exit on podcast image error - this is not critical
			}

			// Show diff first
			if err := storageClient.DiffTextObject(ctx, atom.Config.Aws.Buckets.Output, atom.FeedFile, feedPath); err != nil {
				l.Error("Failed to show diff", "error", err)
				// Don't exit on diff error, continue with upload
			}

			if askerAdapter.Ask(ctx, "Upload new %s?", atom.FeedFile) {
				request := &s3store.UploadRequest{
					Store:       atom.Config.Aws.Buckets.Output,
					Key:         atom.FeedFile,
					Filename:    feedPath,
					ContentType: "text/xml",
				}
				if err := storageClient.UploadFile(ctx, request); err != nil {
					l.Error("Failed to upload RSS", "error", err)
					os.Exit(1)
				}
			}
		} else if upload && dryRun {
			l.Info("Dry run: would upload RSS", "file", feedPath, "bucket", atom.Config.Aws.Buckets.Output)
			// In dry run, also show what images would be checked/uploaded
			if err := checkAndUploadPodcastImage(ctx, atom, askerAdapter, nil); err != nil {
				l.Warn("Failed to check podcast image (dry run)", "error", err)
			}
		}
	},
}

// checkAndUploadPodcastImage checks if podcast images referenced in URLs exist in S3 and uploads missing ones
func checkAndUploadPodcastImage(ctx context.Context, atom *model.Podcast, askerAdapter interface {
	Ask(context.Context, string, ...any) bool
}, uploaderAdapter interface {
	FileExists(context.Context, *s3store.ObjectRequest) (bool, error)
	UploadFile(context.Context, *s3store.UploadRequest) error
}) error {
	l := logger.FromContext(ctx)

	// Extract bucket domain from output bucket URL
	bucketDomain := fmt.Sprintf("https://%s.s3.%s.amazonaws.com", atom.Config.Aws.Buckets.Output, atom.Config.Aws.Region)

	// Function to check and upload an image
	checkAndUpload := func(imageURL, localImagePath, imageType string) error {
		if imageURL == "" {
			return nil // No image URL specified
		}

		// Check if the image URL points to our S3 bucket
		if !strings.HasPrefix(imageURL, bucketDomain) {
			l.Debug("Image URL does not reference our S3 bucket, skipping", "url", imageURL, "type", imageType)
			return nil
		}

		// Extract the S3 key from the URL
		s3Key := strings.TrimPrefix(imageURL, bucketDomain+"/")
		if strings.TrimSpace(localImagePath) == "" {
			localImagePath = s3Key
		}

		// Build the full path using localStorageDir from config
		fullLocalPath := localImagePath
		if atom.Config.LocalStorageDir != "" {
			fullLocalPath = atom.Config.LocalStorageDir + "/" + localImagePath
		}

		// Check if local image file exists
		if _, err := os.Stat(fullLocalPath); os.IsNotExist(err) {
			l.Warn("Local image file does not exist", "path", fullLocalPath, "type", imageType)
			return nil // Can't upload if local file doesn't exist
		} else if err != nil {
			return fmt.Errorf("failed to check local image file %s: %w", fullLocalPath, err)
		}

		if uploaderAdapter == nil {
			// Dry run mode
			l.Info("Would check/upload image", "s3Key", s3Key, "localPath", localImagePath, "type", imageType)
			return nil
		}

		exists, err := uploaderAdapter.FileExists(ctx, &s3store.ObjectRequest{
			Store: atom.Config.Aws.Buckets.Output,
			Key:   s3Key,
		})
		if err != nil {
			return fmt.Errorf("failed to check remote image %s: %w", s3Key, err)
		}
		if exists {
			l.Info("Image already exists in S3, skipping upload", "s3Key", s3Key, "type", imageType)
			return nil
		}

		if askerAdapter.Ask(ctx, "Upload %s image %s to S3?", imageType, localImagePath) {
			// Determine content type based on file extension
			contentType := "image/jpeg"
			if strings.HasSuffix(strings.ToLower(localImagePath), ".png") {
				contentType = "image/png"
			}

			request := &s3store.UploadRequest{
				Store:       atom.Config.Aws.Buckets.Output,
				Key:         s3Key,
				Filename:    fullLocalPath,
				ContentType: contentType,
			}
			return uploaderAdapter.UploadFile(ctx, request)
		}

		return nil
	}

	// Check main podcast image
	// Extract the local path from the image URL by removing the baseURL prefix
	mainImageLocalPath := ""
	if atom.Config.Image != "" && strings.HasPrefix(atom.Config.Image, atom.Config.BaseURL+"/") {
		mainImageLocalPath = strings.TrimPrefix(atom.Config.Image, atom.Config.BaseURL+"/")
	}
	if err := checkAndUpload(atom.Config.Image, mainImageLocalPath, "podcast"); err != nil {
		return fmt.Errorf("failed to check/upload main podcast image: %w", err)
	}

	// Check encoding cover image
	coverImagePath := atom.Encoding.Coverfront
	if coverImagePath != "" {
		// Construct the URL from the base URL and cover image path
		coverImageURL := atom.Config.BaseURL + "/" + coverImagePath
		if err := checkAndUpload(coverImageURL, coverImagePath, "cover"); err != nil {
			return fmt.Errorf("failed to check/upload cover image: %w", err)
		}
	}

	return nil
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

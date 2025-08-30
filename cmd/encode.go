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
	"strconv"
	"time"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/asker"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/awshandler"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/configurator"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/encoder"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/uploader"
	"github.com/spf13/cobra"
)

// encodeCmd represents the encode command
var encodeCmd = &cobra.Command{
	Use:   "encode [episode-uids...] | --all",
	Short: "Encode and upload single or all episodes in podspec.yaml",
	Long: `Encode and upload single or all output files defined in the podcast
specification. This command will encode audio/video master files into
the specified output formats (MP3, M4A, M4B, MP4) and optionally upload
them to the configured S3 bucket.

Usage:
  encode 1 2 3     # Encode episodes with UIDs 1, 2, and 3
  encode --all     # Encode any episode with empty output, missing duration or length
  encode -af       # Encode all episodes (force re-encode without prompting)

The --all flag will only encode episodes that need it (missing output file,
duration, or length). Use --all --force to re-encode all episodes regardless.`,
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		imsg := "Internal error"
		
		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			l.Error(imsg, "error", err)
			os.Exit(1)
		}
		
		if len(args) == 0 && !all {
			l.Error("Syntax error", "error", "You need to select one or several episode UIDs to encode as argument(s) to this command or use the all-option --all")
			os.Exit(1)
		}

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
		removeRemoteMaster, err := cmd.Flags().GetBool("remove-remote-master")
		if err != nil {
			l.Error(imsg, "error", err)
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

		// Create adapters
		askerAdapter := asker.New(false, askNoQuestions)
		encoderAdapter := encoder.New(askerAdapter, "INTELLIGENT_TIERING")
		uploaderAdapter := uploader.New(atom)
		var awshandlerAdapter ports.ForAdministeringRemoteFiles
		if removeRemoteMaster {
			awshandlerAdapter = awshandler.New(atom, askerAdapter)
		}
		
		// Create combined post-processing function
		postEncodeFunc := func(atom *model.Atom, episode *model.Episode) error {
			// First, handle remote master removal if requested
			if removeRemoteMaster && episode.Input != "" {
				request := &ports.ForAdministeringRemoteFilesRequest{
					Store: atom.Config.Aws.Buckets.Input,
					Key:   episode.Input,
				}
				if err := awshandlerAdapter.DeleteRemoteFile(ctx, request); err != nil {
					l.Warn("Failed to remove remote master file", "error", err, "file", episode.Input)
					// Don't fail the entire process for this - just log and continue
				}
			}
			
			// Then, upload the encoded episode
			if episode.Output != "" {
				request := &ports.ForUploadingRequest{
					Store: atom.Config.Aws.Buckets.Output,
					To:    episode.Output,
					From:  episode.Output,
				}
				return uploaderAdapter.Upload(ctx, request, nil)
			}
			return nil
		}

		processedCount := 0
		
		if all {
			// Encode all episodes
			if err := encoderAdapter.Encode(ctx, atom, -1, postEncodeFunc); err != nil {
				l.Error("Failed to encode episodes", "error", err)
				os.Exit(1)
			}
			processedCount = len(encoderAdapter.GetEncodedOutputs())
		} else {
			// Encode specific episodes by UID
			for _, uidStr := range args {
				uid, err := strconv.ParseInt(uidStr, 10, 64)
				if err != nil {
					l.Error("Invalid episode UID", "uid", uidStr, "error", err)
					continue
				}
				
				if err := encoderAdapter.Encode(ctx, atom, uid, postEncodeFunc); err != nil {
					l.Error("Failed to encode episode", "uid", uid, "error", err)
					os.Exit(1)
				}
			}
			processedCount = len(encoderAdapter.GetEncodedOutputs())
		}

		if processedCount == 0 {
			l.Info("No episode was processed")
		} else {
			plural := ""
			if processedCount > 1 {
				plural = "s"
			}
			l.Info("Processing complete", "processed", processedCount, "episodes", plural)
		}

		// Save updated configuration if needed
		if askerAdapter.Ask(ctx, "Fields in the atom have changed, re-write %s?", specFile) {
			atom.LastBuildDate.Time = time.Now().UTC()
			if err := config.Save(ctx, atom); err != nil {
				l.Error("Unable to save configuration", "error", err, "specfile", specFile)
				os.Exit(1)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(encodeCmd)

	// Add flags matching the old mkpod encode command
	encodeCmd.Flags().StringP("spec", "s", configurator.DefaultSpecfile, "Main configuration file for generating the RSS atom")
	encodeCmd.Flags().BoolP("all", "a", false, "Encode any episode with an empty output filename, missing duration or missing length")
	encodeCmd.Flags().BoolP("force", "f", false, "Do not ask whether to re-encode, just do it. Combined with the \"all\" flag, all episodes will be re-encoded")
	encodeCmd.Flags().BoolP("remove-remote-master", "R", false, "Remove remote input master audio or video file before uploading local master input file. Unless the force option is given, there is a yes/no prompt before proceeding")
}

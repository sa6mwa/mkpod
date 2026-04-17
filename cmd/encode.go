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
	"path"
	"strconv"
	"time"

	"golang.org/x/term"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/media/encode"
	"github.com/sa6mwa/mkpod/internal/prompt"
	"github.com/sa6mwa/mkpod/internal/spec"
	s3store "github.com/sa6mwa/mkpod/internal/storage/s3"
	"github.com/spf13/cobra"
)

// encodeCmd represents the encode command
var encodeCmd = &cobra.Command{
	Aliases: []string{"e"},
	Use:     "encode [episode-uids...] | --all",
	Short:   "Encode and upload single or all episodes in podspec.yaml",
	Long: `Encode and upload episode media defined in the podcast specification.
This command encodes local master files into the configured output
formats (MP3, M4A, M4B, MP4) and can upload the resulting files to S3.

Usage:
  encode 1 2 3     # Encode episodes with UIDs 1, 2, and 3
  encode --all     # Encode episodes whose local output file is missing
  encode -af       # Re-encode every episode without prompting

The --all flag skips episodes whose local output file already exists.
Use --all --force to re-encode all episodes regardless.`,
	Example: `  mkpod encode 1
  mkpod encode 1 2 3
  mkpod encode --spec ./podcast/podspec.yaml --all
  mkpod encode --spec ./podcast/podspec.yaml --all --force
  mkpod encode --spec ./podcast/podspec.yaml --all --remove-remote-master`,
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		imsg := "Internal error"

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			l.Error(imsg, "error", err)
			os.Exit(1)
		}

		if len(args) == 0 && !all {
			l.Error("Syntax error", "error", "select one or more episode UIDs or use --all")
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
		config := spec.New(specFile)
		atom, err := config.Load(ctx)
		if err != nil {
			l.Error("Failed to load podcast specification", "error", err, "specfile", specFile)
			os.Exit(1)
		}

		// Create services
		prompter := prompt.New(false, askNoQuestions)
		encoderService := encode.New(prompter)
		storageClient := s3store.New(atom, prompter)

		postEncodeFunc := func(atom *model.Podcast, episode *model.Episode, wasEncoded bool) error {
			if removeRemoteMaster && episode.Input != "" {
				localMasterPath := path.Join(atom.LocalStorageDirExpanded(), episode.Input)
				bucket := atom.Config.Aws.Buckets.Input
				key := episode.Input

				localStat, err := os.Stat(localMasterPath)
				if err != nil && !os.IsNotExist(err) {
					l.Warn("Failed to check local master file", "error", err, "file", localMasterPath)
				} else {
					remoteInfo, err := storageClient.GetFileInfo(ctx, bucket, key)
					if err != nil {
						l.Warn("Failed to get remote master file info", "error", err, "file", episode.Input)
					} else {
						decision := s3store.EvaluateRemoteMasterRemoval(err == nil, fileSize(localStat), remoteInfo.Exists, remoteInfo.Size)
						switch decision.Reason {
						case s3store.RemovalLocalMissing:
							l.Warn("Skipping remote master removal: local master file does not exist", "localFile", localMasterPath, "remoteFile", episode.Input)
						case s3store.RemovalRemoteMissing:
							l.Info("Remote master file does not exist, nothing to remove", "file", episode.Input)
						case s3store.RemovalLocalTooSmall:
							l.Warn("Skipping remote master removal: local file is too small compared to remote", "localFile", localMasterPath, "localSize", decision.LocalSize, "remoteSize", decision.RemoteSize, "minRequired", decision.MinRequiredSize)
						case s3store.RemovalAllowed:
							l.Info("Safety checks passed for remote master removal", "localFile", localMasterPath, "localSize", decision.LocalSize, "remoteSize", decision.RemoteSize)
							if err := storageClient.DeleteRemoteFile(ctx, bucket, key); err != nil {
								l.Warn("Failed to remove remote master file", "error", err, "file", episode.Input)
							}
						}
					}
				}
			}

			// Check if local output file exists but is missing from output bucket
			if episode.Output != "" {
				shouldUpload, err := checkForMissingOutputFile(ctx, atom, episode, prompter, storageClient, wasEncoded)
				if err != nil {
					return fmt.Errorf("failed to check for missing output file: %w", err)
				}

				if shouldUpload {
					// Use full local path for upload
					localPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)
					return storageClient.UploadFile(ctx, atom.Config.Aws.Buckets.Output, episode.Output, localPath, nil)
				}
			}
			return nil
		}

		processedCount := 0
		metadataChanged := false

		prepareEpisode := func(ctx context.Context, atom *model.Podcast, episode *model.Episode) error {
			return prepareEpisodeAssetsForEncode(ctx, atom, episode, storageClient)
		}

		if all {
			result, err := encoderService.Encode(ctx, atom, encode.EncodeOptions{
				All:            true,
				ForceReencode:  askNoQuestions,
				PrepareEpisode: prepareEpisode,
			}, postEncodeFunc)
			if err != nil {
				l.Error("Failed to encode episodes", "error", err)
				os.Exit(1)
			}
			processedCount = len(result.EncodedOutputs)
			metadataChanged = metadataChanged || result.MetadataChanged
		} else {
			// Encode specific episodes by UID
			for _, uidStr := range args {
				uid, err := strconv.ParseInt(uidStr, 10, 64)
				if err != nil {
					l.Error("Invalid episode UID", "uid", uidStr, "error", err)
					continue
				}
				result, err := encoderService.Encode(ctx, atom, encode.EncodeOptions{
					EpisodeUID:     &uid,
					PrepareEpisode: prepareEpisode,
				}, postEncodeFunc)
				if err != nil {
					l.Error("Failed to encode episode", "uid", uid, "error", err)
					os.Exit(1)
				}
				processedCount += len(result.EncodedOutputs)
				metadataChanged = metadataChanged || result.MetadataChanged
			}
		}

		if processedCount == 0 {
			l.Info("No episode was processed")
		} else {
			l.Info("Processing complete", "processed", processedCount)
		}

		shouldSave := metadataChanged
		if shouldSave && !askNoQuestions && term.IsTerminal(int(os.Stdout.Fd())) {
			shouldSave = prompter.Ask(ctx, "Podcast metadata changed, rewrite %s?", specFile)
		}
		if shouldSave {
			atom.LastBuildDate.Time = time.Now().UTC()
			if err := config.Save(ctx, atom); err != nil {
				l.Error("Unable to save configuration", "error", err, "specfile", specFile)
				os.Exit(1)
			}
		}
	},
}

// checkForMissingOutputFile checks if a local output file exists but is missing from the output bucket
func checkForMissingOutputFile(ctx context.Context, atom *model.Podcast, episode *model.Episode, askerAdapter interface {
	Ask(context.Context, string, ...any) bool
}, storageClient interface {
	FileExists(context.Context, string, string) (bool, error)
}, wasEncoded bool) (bool, error) {
	l := logger.FromContext(ctx)

	// Build full local path
	localPath := path.Join(atom.LocalStorageDirExpanded(), episode.Output)

	// Check if local file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return false, nil // No local file, nothing to upload
	} else if err != nil {
		return false, fmt.Errorf("failed to access local encoded file %s: %w", localPath, err)
	}

	// Check if remote file exists
	if storageClient != nil {
		bucket := atom.Config.Aws.Buckets.Output
		key := episode.Output
		exists, err := storageClient.FileExists(ctx, bucket, key)
		if err != nil {
			return false, fmt.Errorf("failed to check remote output %s in bucket %s: %w", key, bucket, err)
		}

		if !exists {
			l.Info("Local output file exists but is missing from output bucket", "file", episode.Output, "localPath", localPath)
			return askerAdapter.Ask(ctx, "Upload local file %s to output bucket?", episode.Output), nil
		} else if wasEncoded {
			// File exists remotely but was just re-encoded locally, ask if we should overwrite
			l.Info("Output file was re-encoded and remote file exists", "file", episode.Output)
			return askerAdapter.Ask(ctx, "Overwrite remote file %s with newly encoded version?", episode.Output), nil
		}
	}

	return false, nil
}

func init() {
	rootCmd.AddCommand(encodeCmd)

	// Add flags matching the old mkpod encode command
	encodeCmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file (podspec.yaml)")
	encodeCmd.Flags().BoolP("all", "a", false, "Encode episodes whose local output file is missing")
	encodeCmd.Flags().BoolP("force", "f", false, "Do not prompt. Combined with --all, re-encode every episode even if a local output already exists")
	encodeCmd.Flags().BoolP("remove-remote-master", "R", false, "Remove remote input master audio or video file before uploading local master input file. Unless the force option is given, there is a yes/no prompt before proceeding")
}

func fileSize(fi os.FileInfo) int64 {
	if fi == nil {
		return 0
	}
	return fi.Size()
}

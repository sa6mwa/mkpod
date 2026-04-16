/*
Copyright © 2025 SA6MWA Michel
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/spec"
	"github.com/spf13/cobra"
)

var (
	ErrTargetDirRequired = errors.New("target directory argument is required")
	ErrTargetNotEmpty    = errors.New("target directory exists and is not empty")
)

var initCmd = &cobra.Command{
	Use:   "init <directory>",
	Short: "Initialize a new mkpod workspace",
	Long: `Initialize a new mkpod workspace by creating a directory structure
and a starter podspec.yaml template in the target directory.`,
	Example: `  mkpod init .
  mkpod init ~/podcast
  mkpod init /srv/podcasts/my-show`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrTargetDirRequired
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir, err := initWorkspace(args[0])
		if err != nil {
			return err
		}

		cmd.Printf("Initialized mkpod workspace in %s\n", targetDir)
		cmd.Println("Created:")
		cmd.Printf("  %s\n", filepath.Join(targetDir, spec.DefaultSpecfile))
		cmd.Printf("  %s\n", filepath.Join(targetDir, "artwork"))
		cmd.Printf("  %s\n", filepath.Join(targetDir, "masters"))
		cmd.Println("")
		cmd.Println("Next steps:")
		cmd.Println("  1. Edit podspec.yaml with your podcast metadata and S3 bucket names.")
		cmd.Println("  2. Add artwork under artwork/ and source audio under masters/.")
		cmd.Println("  3. Encoded outputs and podcast.rss will be written under localStorageDir (this workspace by default).")
		cmd.Println("  4. Run `mkpod parse --spec <dir>/podspec.yaml` or `mkpod encode --spec <dir>/podspec.yaml --all`.")
		return nil
	},
}

func initWorkspace(target string) (string, error) {
	targetDir, err := resolveWorkspacePath(target)
	if err != nil {
		return "", err
	}

	if err := ensureEmptyDir(targetDir); err != nil {
		return "", err
	}

	for _, dir := range []string{
		targetDir,
		filepath.Join(targetDir, "artwork"),
		filepath.Join(targetDir, "masters"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create %s: %w", dir, err)
		}
	}

	initialSpec := newInitialSpec(targetDir)
	store := spec.New(filepath.Join(targetDir, spec.DefaultSpecfile))
	if err := store.Save(context.Background(), initialSpec); err != nil {
		return "", err
	}

	return targetDir, nil
}

func resolveWorkspacePath(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", ErrTargetDirRequired
	}
	target = expandTilde(target)
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}
	return abs, nil
}

func expandTilde(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func ensureEmptyDir(targetDir string) error {
	info, err := os.Stat(targetDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", targetDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", targetDir)
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", targetDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%w: %s", ErrTargetNotEmpty, targetDir)
	}
	return nil
}

func newInitialSpec(targetDir string) *model.Atom {
	podcastName := filepath.Base(targetDir)
	if podcastName == "." || podcastName == string(filepath.Separator) || strings.TrimSpace(podcastName) == "" {
		podcastName = "my-podcast"
	}

	baseURL := "https://example-podcast-bucket.s3.us-east-1.amazonaws.com"

	spec := &model.Atom{
		Config: model.Config{
			BaseURL:         baseURL,
			Image:           baseURL + "/artwork/podcast-cover.jpg",
			DefaultPodImage: "artwork/podcast-cover.jpg",
			Aws: model.AwsConfig{
				Profile: "default",
				Region:  "us-east-1",
				Buckets: model.Buckets{
					Input:  "example-podcast-assets",
					Output: "example-podcast-bucket",
				},
			},
			LocalStorageDir: targetDir,
		},
		Atom:        "podcast.rss",
		Title:       podcastName,
		Link:        "https://example.com/" + podcastName,
		TTL:         60,
		Language:    "en",
		Copyright:   "Copyright Example",
		WebMaster:   "you@example.com",
		Description: "Replace this description with your podcast summary.",
		Subtitle:    "Replace this subtitle.",
		OwnerName:   "Your Name",
		OwnerEmail:  "you@example.com",
		Author:      "Your Name",
		Explicit:    model.ItunesExplicit{S: "no"},
		Keywords:    "podcast",
		Categories: []model.Category{
			{Name: "Technology", Subcategories: []string{}},
		},
		Episodes: []model.Episode{},
	}

	spec.Encoding.PreferredFormat = "mp3"
	spec.Encoding.Bitrate = 128
	spec.Encoding.Lamepath = "lame"
	spec.Encoding.FFmpegPath = "ffmpeg"
	spec.Encoding.CRF = 28
	spec.Encoding.ABR = "128k"
	spec.Encoding.Coverfront = "artwork/podcast-cover.jpg"
	spec.Encoding.Genre = "Podcast"
	spec.Encoding.Language = "eng"

	return spec
}

func init() {
	rootCmd.AddCommand(initCmd)
}

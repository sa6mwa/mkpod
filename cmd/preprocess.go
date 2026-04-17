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

	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/media/preprocess"
	"github.com/spf13/cobra"
)

const defaultPreProcessingPrefix string = "preprocessed-"
const defaultPreset string = "sm7b"
const availablePreprocessPresets = "sm7b, qzj, aggressive, heavy, qzj-podmic, qzj-podmic2, lowcut"

// preprocessCmd represents the preprocess command
var preprocessCmd = &cobra.Command{
	Use:     "preprocess [flags] audiofiles...",
	Aliases: []string{"pre"},
	Short:   "Apply optional ffmpeg-based cleanup filters to raw audio files.",
	Long: `mkpod preprocess is a thin utility wrapper around a small set of
ffmpeg filter presets. It is intended for optional cleanup of raw voice
recordings before editing, not as a required part of the spec-driven
encode/parse workflow.

The default preset is sm7b. Available presets are: ` + availablePreprocessPresets + `.`,
	Example: `  mkpod preprocess masters/raw.wav
  mkpod preprocess --preset sm7b masters/intro.wav masters/interview.wav
  mkpod preprocess --preset lowcut masters/room-tone.wav
  mkpod pre --prefix cleaned- masters/episode.wav`,
	Run: func(cmd *cobra.Command, args []string) {
		imsg := "Internal error"
		l := logger.DefaultLogger()
		prefix, err := cmd.Flags().GetString("prefix")
		if err != nil {
			l.Error(imsg, "error", err)
			os.Exit(1)
		}
		preset, err := cmd.Flags().GetString("preset")
		if err != nil {
			l.Error(imsg, "error", err)
			os.Exit(1)
		}

		if len(args) == 0 {
			l.Error("Syntax error", "error", "provide one or more audio files to preprocess")
			os.Exit(1)
		}

		processor := preprocess.New(&preprocess.Config{
			Prefix: prefix,
			Preset: preset,
		})

		ctx := context.Background()

		if err := processor.Process(ctx, args); err != nil {
			l.Error("Failed to preprocess audio", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(preprocessCmd)

	preprocessCmd.Flags().String("prefix", defaultPreProcessingPrefix, "Prefix to prepend to each generated output filename")
	preprocessCmd.Flags().StringP("preset", "p", defaultPreset, "Preprocessing preset to apply. Available: "+availablePreprocessPresets)
}

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

	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/preprocessor"
	"github.com/spf13/cobra"
)

const defaultPreProcessingPrefix string = "preprocessed-"
const defaultPreset string = "sm7b"

// preprocessCmd represents the preprocess command
var preprocessCmd = &cobra.Command{
	Use:     "preprocess [flags] audiofiles...",
	Aliases: []string{"pre"},
	Short:   "Pre-process an audio file (e.g a raw microphone track).",
	Long: `mkpod preprocess is intended to be used before editing an episode to
adjust EQ, compression, limiter and similar. The default preprocessing
preset is sm7b (for audio recorded with the Shure SM7B). Available
presets are: sm7b, qzj, aggressive, heavy, qzj-podmic, qzj-podmic2,
none. Limiter settings (except preset "none") will allow you to have
background audio/music -10 dB. Minus 10.01 dB in fraction is
0.3158639048423471 or 0.31586 which should produce a mix without
clipping.`,
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
			l.Error(imsg, "error", "no file(s) to preprocess given as arguments")
			os.Exit(1)
		}

		preprocess := preprocessor.New(&preprocessor.Config{
			Prefix: prefix,
			Preset: preset,
		})

		ctx := context.Background()

		if err := preprocess.Process(ctx, args); err != nil {
			l.Error("Error pre-processing", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(preprocessCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// preprocessCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// preprocessCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	preprocessCmd.Flags().String("prefix", defaultPreProcessingPrefix, "Prefix to add to the output filename")
	preprocessCmd.Flags().StringP("preset", "p", defaultPreset, "Preset for EQ, compression, limiter and similar, available: sm7b, qzj, aggressive, heavy, qzj-podmic, qzj-podmic2, none.")
}

package preprocessor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
	"github.com/sa6mwa/mkpod/internal/media"
)

var (
	ErrNoFilesToProcess error = errors.New("empty slice, no media files to process")
)

const defaultPrefix = "preprocessed-"
const defaultPreset = "sm7b"
const defaultTool = "ffmpeg"

func New(config *Config) *Processor {
	if config == nil {
		config = &Config{
			Preset: defaultPreset,
			Prefix: defaultPrefix,
			Tool:   defaultTool,
		}
	} else {
		if config.Prefix == "" {
			config.Prefix = defaultPrefix
		}
		if config.Preset == "" {
			config.Preset = defaultPreset
		}
		if config.Tool == "" {
			config.Tool = defaultTool
		}
	}
	return &Processor{config: *config}
}

// Preprocessor configuration.
type Config struct {
	// Preset can be sm7b (default if empty), qzj, aggressive, heavy,
	// qzj-podmic, qzj-podmic2.
	Preset string
	// Prefix is prepended to the input file as the output file. Default
	// to preprocessed- if empty.
	Prefix string
	// Tool command, can be absolute path. Defaults to ffmpeg.
	Tool string
}

type Processor struct {
	config Config
}

func (p *Processor) Process(ctx context.Context, mediaFilePaths []string) error {
	l := logger.FromContext(ctx)
	if len(mediaFilePaths) == 0 {
		return ErrNoFilesToProcess
	}
	if err := media.EnsureToolAvailable(p.config.Tool); err != nil {
		return err
	}

	filter, err := filterForPreset(p.config.Preset)
	if err != nil {
		return err
	}

	var fcount int
	var lastInput, lastOutput string
	for _, input := range mediaFilePaths {
		output := p.config.Prefix + input
		args := []string{"-y", "-i", input, "-vn", "-ac", "2", "-filter_complex", filter, output}
		l.Info("Preprocessing", "file", input, "output", output, "tool", p.config.Tool, "args", args)
		cmd := exec.CommandContext(ctx, p.config.Tool, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unable to pre-process %q using external tool (%s): %w", input, p.config.Tool, err)
		}
		lastInput = input
		lastOutput = output
		fcount++
	}
	if fcount == 1 {
		l.Info("Processed one media file", "input", lastInput, "output", lastOutput)
	} else {
		l.Info(fmt.Sprintf("Processed %d files", fcount))
	}
	return nil
}

// SetPrefix is a setter for the instance's prefix value. Can be used to over
func (p *Processor) SetPrefix(prefix string) *Processor {
	p.config.Prefix = prefix
	return p
}

// SetPreset is a setter for the instance's preset value.
func (p *Processor) SetPreset(preset string) *Processor {
	p.config.Preset = preset
	return p
}

// SetFFmpeg is a setter for the instance's path to FFmpeg.
func (p *Processor) SetTool(toolPath string) *Processor {
	p.config.Tool = toolPath
	return p
}

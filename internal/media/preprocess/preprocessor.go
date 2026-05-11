package preprocess

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sa6mwa/mkpod/internal/logging"
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

type Plan struct {
	Preset     string
	Prefix     string
	Filter     string
	Operations []Operation
}

type Operation struct {
	Input  string
	Output string
	Tool   string
	Args   []string
}

func (p *Processor) Plan(mediaFilePaths []string) (*Plan, error) {
	if len(mediaFilePaths) == 0 {
		return nil, ErrNoFilesToProcess
	}
	if err := media.EnsureToolAvailable(p.config.Tool); err != nil {
		return nil, err
	}

	filter, err := filterForPreset(p.config.Preset)
	if err != nil {
		return nil, err
	}

	plan := &Plan{
		Preset:     p.config.Preset,
		Prefix:     p.config.Prefix,
		Filter:     filter,
		Operations: make([]Operation, 0, len(mediaFilePaths)),
	}
	for _, input := range mediaFilePaths {
		output := outputPath(input, p.config.Prefix)
		plan.Operations = append(plan.Operations, Operation{
			Input:  input,
			Output: output,
			Tool:   p.config.Tool,
			Args:   buildArgs(input, output, filter),
		})
	}
	return plan, nil
}

func (p *Processor) Process(ctx context.Context, mediaFilePaths []string) error {
	plan, err := p.Plan(mediaFilePaths)
	if err != nil {
		return err
	}
	return ExecutePlan(ctx, plan)
}

func ExecutePlan(ctx context.Context, plan *Plan) error {
	l := logger.FromContext(ctx)
	var fcount int
	var lastInput, lastOutput string
	for _, operation := range plan.Operations {
		l.Info("Preprocessing", "file", operation.Input, "output", operation.Output, "tool", operation.Tool, "args", operation.Args)
		cmd := exec.CommandContext(ctx, operation.Tool, operation.Args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unable to pre-process %q using external tool (%s): %w", operation.Input, operation.Tool, err)
		}
		lastInput = operation.Input
		lastOutput = operation.Output
		fcount++
	}
	if fcount == 1 {
		l.Info("Processed one media file", "input", lastInput, "output", lastOutput)
	} else {
		l.Info(fmt.Sprintf("Processed %d files", fcount))
	}
	return nil
}

func buildArgs(input, output, filter string) []string {
	return []string{"-y", "-i", input, "-vn", "-ac", "2", "-filter_complex", filter, output}
}

func outputPath(input, prefix string) string {
	dir := filepath.Dir(input)
	output := prefix + filepath.Base(input)
	if dir == "." {
		return output
	}
	return filepath.Join(dir, output)
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

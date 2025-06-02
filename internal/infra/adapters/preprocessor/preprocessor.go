// The preprocessor adapter implements the ports.ForPreprocessing
// interface.
package preprocessor

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"os"
	"os/exec"

	"github.com/alessio/shellescape"
	"github.com/sa6mwa/mkpod/internal/app/ports"
	"github.com/sa6mwa/mkpod/internal/infra/adapters/logger"
)

var (
	ErrNoFilesToProcess error = errors.New("empty slice, no media files to process")
)

const defaultPrefix = "preprocessed-"
const defaultPreset = "sm7b"
const shell = "/bin/sh"
const shellCommandOption = "-c"
const defaultTool = "ffmpeg"

// preprocessor.New returns a local-to-local media file preprocessor
// adapter for the ports.ForPreprocessing port. If Config is nil,
// default configuration will be used (preset=sm7b,
// prefix=preprocessed-).
func New(config *Config) ports.ForPreprocessing {
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
	return &forPreprocessing{
		config: *config,
		funcMap: template.FuncMap{
			"escape": func(s string) string {
				return shellescape.Quote(s)
			},
		},
	}
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

type Variables struct {
	Config
	Input string
}

// forPreprocessing implements the ports.ForPreprocessing port (interface).
type forPreprocessing struct {
	config  Config
	funcMap template.FuncMap
}

func (p *forPreprocessing) Process(ctx context.Context, mediaFilePaths []string) error {
	l := logger.FromContext(ctx)
	if len(mediaFilePaths) == 0 {
		return ErrNoFilesToProcess
	}

	tmpl, err := template.New("PreProcessing").Funcs(p.funcMap).Parse(preProcessingTemplate)
	if err != nil {
		return err
	}

	var variables Variables
	variables.Prefix = p.config.Prefix
	variables.Preset = p.config.Preset
	variables.Tool = p.config.Tool

	var fcount int
	var lastInput, lastOutput string
	for _, input := range mediaFilePaths {
		variables.Input = input
		buf := &bytes.Buffer{}
		if err := tmpl.Execute(buf, variables); err != nil {
			return err
		}
		l.Info("Preprocessing", "file", input, "output", variables.Prefix+input, "command", buf.String())
		cmd := exec.CommandContext(ctx, shell, shellCommandOption, buf.String())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unable to pre-process %q using external tool (%s): %w", input, variables.Tool, err)
		}
		lastInput = input
		lastOutput = variables.Prefix + input
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
func (p *forPreprocessing) SetPrefix(prefix string) ports.ForPreprocessing {
	p.config.Prefix = prefix
	return p
}

// SetPreset is a setter for the instance's preset value.
func (p *forPreprocessing) SetPreset(preset string) ports.ForPreprocessing {
	p.config.Preset = preset
	return p
}

// SetFFmpeg is a setter for the instance's path to FFmpeg.
func (p *forPreprocessing) SetTool(toolPath string) ports.ForPreprocessing {
	p.config.Tool = toolPath
	return p
}

package blenderaddon

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultBlenderTool = "blender"
	defaultRepo        = "user_default"
	extensionFilename  = "mkpod_export_markers.zip"
	addonModule        = "mkpod_export_markers"
	addonName          = "Export Markers as YAML Chapters"
)

//go:embed export_markers.py
var markerExporter []byte

type Options struct {
	Blender string
	Repo    string
}

type Plan struct {
	BlenderTool string   `json:"blenderTool"`
	BlenderPath string   `json:"blenderPath"`
	Repo        string   `json:"repo"`
	AddonName   string   `json:"addonName"`
	AddonModule string   `json:"addonModule"`
	Actions     []string `json:"actions"`
}

type Runner interface {
	Run(context.Context, string, ...string) error
}

type CommandRunner struct{}

func (CommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func BuildPlan(options Options) (*Plan, error) {
	tool := strings.TrimSpace(options.Blender)
	if tool == "" {
		tool = defaultBlenderTool
	}
	repo := strings.TrimSpace(options.Repo)
	if repo == "" {
		repo = defaultRepo
	}

	blenderPath, err := findBlender(tool)
	if err != nil {
		return nil, fmt.Errorf("required tool not found: %s (install Blender or pass --blender)", tool)
	}

	return &Plan{
		BlenderTool: tool,
		BlenderPath: blenderPath,
		Repo:        repo,
		AddonName:   addonName,
		AddonModule: addonModule,
		Actions: []string{
			"build embedded Blender extension package",
			"install extension package with Blender's extension CLI",
			"enable the extension after installation",
		},
	}, nil
}

func findBlender(tool string) (string, error) {
	if blenderPath, err := exec.LookPath(tool); err == nil {
		return blenderPath, nil
	}
	if filepath.IsAbs(tool) || strings.ContainsRune(tool, filepath.Separator) || tool != defaultBlenderTool {
		return "", exec.ErrNotFound
	}
	for _, candidate := range commonBlenderPaths(runtime.GOOS) {
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func commonBlenderPaths(goos string) []string {
	switch goos {
	case "darwin":
		return []string{
			"/Applications/Blender.app/Contents/MacOS/Blender",
		}
	case "windows":
		return []string{
			`C:\Program Files\Blender Foundation\Blender\blender.exe`,
			`C:\Program Files\Blender Foundation\Blender 4.2\blender.exe`,
			`C:\Program Files\Blender Foundation\Blender 4.3\blender.exe`,
			`C:\Program Files\Blender Foundation\Blender 4.4\blender.exe`,
		}
	default:
		return []string{
			"/usr/bin/blender",
			"/usr/local/bin/blender",
			"/snap/bin/blender",
			"/var/lib/flatpak/exports/bin/org.blender.Blender",
		}
	}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func Apply(ctx context.Context, options Options, runner Runner) (*Plan, error) {
	if runner == nil {
		runner = CommandRunner{}
	}

	plan, err := BuildPlan(options)
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "mkpod-blender-addon-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary Blender extension directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	extensionPackage, err := extensionZip()
	if err != nil {
		return nil, err
	}

	extensionPath := filepath.Join(tempDir, extensionFilename)
	if err := os.WriteFile(extensionPath, extensionPackage, 0o644); err != nil {
		return nil, fmt.Errorf("write embedded Blender extension package: %w", err)
	}

	args := []string{"--command", "extension", "install-file", "-r", plan.Repo, "-e", extensionPath}
	if err := runner.Run(ctx, plan.BlenderPath, args...); err != nil {
		return nil, fmt.Errorf("install Blender add-on %q using %s: %w", addonName, plan.BlenderPath, err)
	}

	return plan, nil
}

func extensionZip() ([]byte, error) {
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	if err := writeZipFile(archive, "blender_manifest.toml", []byte(extensionManifest)); err != nil {
		archive.Close()
		return nil, err
	}
	if err := writeZipFile(archive, "__init__.py", markerExporter); err != nil {
		archive.Close()
		return nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, fmt.Errorf("build Blender extension package: %w", err)
	}
	return buf.Bytes(), nil
}

func writeZipFile(archive *zip.Writer, name string, content []byte) error {
	file, err := archive.Create(name)
	if err != nil {
		return fmt.Errorf("add %s to Blender extension package: %w", name, err)
	}
	if _, err := io.Copy(file, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write %s to Blender extension package: %w", name, err)
	}
	return nil
}

const extensionManifest = `schema_version = "1.0.0"
id = "mkpod_export_markers"
version = "1.0.0"
name = "mkpod Export Markers"
tagline = "Export timeline markers as YAML podcast chapters"
maintainer = "SA6MWA"
type = "add-on"
blender_version_min = "4.2.0"
license = ["SPDX:MIT"]
`

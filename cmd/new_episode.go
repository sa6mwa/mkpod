package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/sa6mwa/id3v24"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/logging"
	"github.com/sa6mwa/mkpod/internal/spec"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Plan a new podcast episode",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		plan, err := buildNewEpisodePlan(context.Background(), cmd, args)
		if err != nil {
			l.Error("Unable to plan new episode", "error", err)
			os.Exit(1)
		}
		printNewEpisodePlan(plan)
		writePlan(cmd, "new", plan, defaultPlanPath("new", mustGetStringFlag(cmd, "spec")))
	},
}

var planNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Preview adding a new podcast episode",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.DefaultLogger()
		plan, err := buildNewEpisodePlan(context.Background(), cmd, args)
		if err != nil {
			l.Error("Unable to plan new episode", "error", err)
			os.Exit(1)
		}
		printNewEpisodePlan(plan)
		writePlan(cmd, "new", plan, defaultPlanPath("new", mustGetStringFlag(cmd, "spec")))
	},
}

type newEpisodePlan struct {
	SpecFile string        `json:"specFile"`
	Episode  model.Episode `json:"episode"`
}

type newEpisodeInputs struct {
	SpecFile                  string
	NonInteractive            bool
	Description               string
	DescriptionFromClipboard  bool
	UID                       string
	Author                    string
	Title                     string
	Link                      string
	Subtitle                  string
	Image                     string
	Input                     string
	Format                    string
	EncodingLanguage          string
	InheritedEncodingLanguage string
	Chapters                  string
}

type blenderChaptersFile struct {
	Chapters []id3v24.Chapter `yaml:"chapters"`
}

func buildNewEpisodePlan(ctx context.Context, cmd *cobra.Command, args []string) (*newEpisodePlan, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("new does not take positional arguments")
	}
	inputs, err := newEpisodeInputsFromFlags(cmd)
	if err != nil {
		return nil, err
	}
	atom, err := spec.New(inputs.SpecFile).Load(ctx)
	if err != nil {
		return nil, err
	}
	defaults := defaultNewEpisodeInputs(atom, inputs.SpecFile)
	mergeNewEpisodeInputs(&defaults, inputs)
	if inputs.DescriptionFromClipboard {
		description, err := readClipboardDescription()
		if err != nil {
			return nil, err
		}
		defaults.Description = description
	}
	interactive := !defaults.NonInteractive && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	if interactive {
		if err := runNewEpisodeForm(atom, &defaults); err != nil {
			return nil, err
		}
	} else {
		defaults.NonInteractive = true
	}
	if defaults.NonInteractive {
		if err := validateNonInteractiveNewEpisodeFlags(inputs); err != nil {
			return nil, err
		}
	}
	return newEpisodePlanFromInputs(atom, defaults)
}

func newEpisodeInputsFromFlags(cmd *cobra.Command) (newEpisodeInputs, error) {
	specFile, err := cmd.Flags().GetString("spec")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	nonInteractive, err := cmd.Flags().GetBool("non-interactive")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	descriptionFromClipboard, err := cmd.Flags().GetBool("description-from-clipboard")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	get := func(name string) (string, error) {
		return cmd.Flags().GetString(name)
	}
	uid, err := get("uid")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	author, err := get("author")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	title, err := get("title")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	link, err := get("link")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	subtitle, err := get("subtitle")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	description, err := get("description")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	image, err := get("image")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	input, err := get("input")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	format, err := get("format")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	encodingLanguage, err := get("encoding-language")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	chapters, err := get("chapters")
	if err != nil {
		return newEpisodeInputs{}, err
	}
	return newEpisodeInputs{
		SpecFile:                 specFile,
		NonInteractive:           nonInteractive,
		Description:              description,
		DescriptionFromClipboard: descriptionFromClipboard,
		UID:                      uid,
		Author:                   author,
		Title:                    title,
		Link:                     link,
		Subtitle:                 subtitle,
		Image:                    image,
		Input:                    input,
		Format:                   format,
		EncodingLanguage:         encodingLanguage,
		Chapters:                 chapters,
	}, nil
}

func defaultNewEpisodeInputs(atom *model.Podcast, specFile string) newEpisodeInputs {
	defaults := newEpisodeInputs{SpecFile: specFile}
	if len(atom.Episodes) == 0 {
		defaults.UID = "1"
		defaults.Author = atom.Author
		defaults.Image = atom.Config.DefaultPodImage
		defaults.InheritedEncodingLanguage = atom.Encoding.Language
		return defaults
	}
	previous := atom.Episodes[0]
	for _, episode := range atom.Episodes[1:] {
		if episode.UID > previous.UID {
			previous = episode
		}
	}
	defaults.UID = strconv.FormatInt(previous.UID+1, 10)
	defaults.Author = spec.EffectiveEpisodeAuthor(atom, &previous)
	defaults.Image = spec.EffectiveEpisodeImage(atom, &previous)
	defaults.Format = previous.Format
	defaults.InheritedEncodingLanguage = atom.Encoding.Language
	defaults.EncodingLanguage = previous.EncodingLanguage
	if strings.TrimSpace(previous.Input) != "" {
		dir := filepath.ToSlash(filepath.Dir(previous.Input))
		if dir != "." {
			defaults.Input = strings.TrimRight(dir, "/") + "/"
		}
	}
	defaults.Link = previous.Link
	return defaults
}

func mergeNewEpisodeInputs(defaults *newEpisodeInputs, overrides newEpisodeInputs) {
	defaults.NonInteractive = overrides.NonInteractive
	defaults.DescriptionFromClipboard = overrides.DescriptionFromClipboard
	if strings.TrimSpace(overrides.SpecFile) != "" {
		defaults.SpecFile = overrides.SpecFile
	}
	if strings.TrimSpace(overrides.UID) != "" {
		defaults.UID = overrides.UID
	}
	if strings.TrimSpace(overrides.Author) != "" {
		defaults.Author = overrides.Author
	}
	if strings.TrimSpace(overrides.Title) != "" {
		defaults.Title = overrides.Title
	}
	if strings.TrimSpace(overrides.Link) != "" {
		defaults.Link = overrides.Link
	}
	if strings.TrimSpace(overrides.Subtitle) != "" {
		defaults.Subtitle = overrides.Subtitle
	}
	if strings.TrimSpace(overrides.Description) != "" {
		defaults.Description = overrides.Description
	}
	if strings.TrimSpace(overrides.Image) != "" {
		defaults.Image = overrides.Image
	}
	if strings.TrimSpace(overrides.Input) != "" {
		defaults.Input = overrides.Input
	}
	if strings.TrimSpace(overrides.Format) != "" {
		defaults.Format = overrides.Format
	}
	if strings.TrimSpace(overrides.EncodingLanguage) != "" {
		defaults.EncodingLanguage = overrides.EncodingLanguage
	}
	if strings.TrimSpace(overrides.Chapters) != "" {
		defaults.Chapters = overrides.Chapters
	}
}

func runNewEpisodeForm(atom *model.Podcast, inputs *newEpisodeInputs) error {
	chapters := inputs.Chapters
	if strings.TrimSpace(chapters) == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			chapters = home
		}
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("UID").Value(&inputs.UID),
			huh.NewInput().Title("Title").Value(&inputs.Title),
			huh.NewInput().Title("Link").Value(&inputs.Link),
			huh.NewInput().Title("Subtitle").Value(&inputs.Subtitle),
			huh.NewInput().Title("Author").Value(&inputs.Author),
			localStorageFilePicker(atom, "Image", "Stored relative to localStorageDir", inputs.Image, &inputs.Image),
			localStorageFilePicker(atom, "Input", "Edited master, stored relative to localStorageDir", inputs.Input, &inputs.Input),
			huh.NewInput().Title("Format").Description("Optional: mp3, m4a, m4b, audio, video").Value(&inputs.Format).Inline(true),
			huh.NewInput().Title("Encoding language").Description(encodingLanguageDescription(inputs)).Placeholder(inputs.InheritedEncodingLanguage).Value(&inputs.EncodingLanguage).Inline(true),
			huh.NewFilePicker().Title("Chapters file").CurrentDirectory(chaptersPickerDirectory(chapters)).Value(&inputs.Chapters).FileAllowed(true).DirAllowed(false),
		),
		huh.NewGroup(
			huh.NewText().Title("Description").Value(&inputs.Description).Lines(18),
		),
	).WithTheme(huh.ThemeCharm()).WithProgramOptions(
		tea.WithOutput(os.Stderr),
		tea.WithReportFocus(),
		tea.WithAltScreen(),
	)
	return form.Run()
}

func localStorageFilePicker(atom *model.Podcast, title, description, current string, value *string) *huh.FilePicker {
	return huh.NewFilePicker().
		Title(title).
		Description(description).
		CurrentDirectory(localStoragePickerDirectory(atom, current)).
		Value(value).
		FileAllowed(true).
		DirAllowed(false)
}

func localStoragePickerDirectory(atom *model.Podcast, current string) string {
	root := "."
	if atom != nil && strings.TrimSpace(atom.LocalStorageDirExpanded()) != "" {
		root = atom.LocalStorageDirExpanded()
	}
	current = strings.TrimSpace(current)
	if current == "" {
		return root
	}
	if filepath.IsAbs(current) {
		rel, err := localStorageRelativePath(atom, current, "path", false)
		if err != nil {
			return root
		}
		current = rel
	}
	dir := filepath.Dir(filepath.FromSlash(current))
	if dir == "." {
		return root
	}
	return filepath.Join(root, dir)
}

func encodingLanguageDescription(inputs *newEpisodeInputs) string {
	if strings.TrimSpace(inputs.EncodingLanguage) != "" {
		return "Explicit on this episode; clear to inherit podcast encoding language"
	}
	if strings.TrimSpace(inputs.InheritedEncodingLanguage) != "" {
		return "Inherited from encoding.language unless explicitly set here"
	}
	return "Optional per-episode override"
}

func chaptersPickerDirectory(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
		return "."
	}
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return path
	}
	dir := filepath.Dir(path)
	if dir == "." {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	return dir
}

func localStorageRelativePath(atom *model.Podcast, value, field string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", fmt.Errorf("%s is required", field)
		}
		return "", nil
	}
	root := "."
	if atom != nil && strings.TrimSpace(atom.LocalStorageDirExpanded()) != "" {
		root = atom.LocalStorageDirExpanded()
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve localStorageDir for %s: %w", field, err)
	}

	var rel string
	if filepath.IsAbs(value) {
		valueAbs, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("resolve %s path: %w", field, err)
		}
		rel, err = filepath.Rel(rootAbs, valueAbs)
		if err != nil {
			return "", fmt.Errorf("resolve %s relative to localStorageDir: %w", field, err)
		}
	} else {
		rel = filepath.FromSlash(value)
	}

	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." || filepath.IsAbs(clean) {
		return "", fmt.Errorf("%s must be a path inside localStorageDir and must not use ..", field)
	}
	return filepath.ToSlash(clean), nil
}

func newEpisodePlanFromInputs(atom *model.Podcast, inputs newEpisodeInputs) (*newEpisodePlan, error) {
	uid, err := strconv.ParseInt(strings.TrimSpace(inputs.UID), 10, 64)
	if err != nil || uid <= 0 {
		return nil, fmt.Errorf("uid must be a positive integer")
	}
	episode := model.Episode{
		UID:              uid,
		Title:            strings.TrimSpace(inputs.Title),
		Link:             strings.TrimSpace(inputs.Link),
		Subtitle:         strings.TrimSpace(inputs.Subtitle),
		Description:      strings.TrimSpace(inputs.Description),
		Author:           strings.TrimSpace(inputs.Author),
		Image:            strings.TrimSpace(inputs.Image),
		Input:            strings.TrimSpace(inputs.Input),
		Format:           strings.TrimSpace(strings.ToLower(inputs.Format)),
		EncodingLanguage: strings.TrimSpace(inputs.EncodingLanguage),
	}
	if strings.TrimSpace(episode.Author) == "" {
		episode.Author = atom.Author
	}
	if strings.TrimSpace(episode.Image) == "" {
		episode.Image = atom.Config.DefaultPodImage
	}
	if episode.Image, err = localStorageRelativePath(atom, episode.Image, "image", false); err != nil {
		return nil, err
	}
	if episode.Input, err = localStorageRelativePath(atom, episode.Input, "input", true); err != nil {
		return nil, err
	}
	if chaptersPath := strings.TrimSpace(inputs.Chapters); chaptersPath != "" {
		chapters, err := loadChaptersFile(chaptersPath)
		if err != nil {
			return nil, err
		}
		episode.Chapters = chapters
	}
	if err := validateNewEpisode(atom, &episode); err != nil {
		return nil, err
	}
	return &newEpisodePlan{SpecFile: inputs.SpecFile, Episode: episode}, nil
}

func applyNewEpisodePlan(ctx context.Context, plan *newEpisodePlan) error {
	if strings.TrimSpace(plan.SpecFile) == "" {
		return errors.New("stale or invalid new episode plan: specFile is required")
	}
	config := spec.New(plan.SpecFile)
	atom, err := config.Load(ctx)
	if err != nil {
		return err
	}
	episode := plan.Episode
	if err := validateNewEpisode(atom, &episode); err != nil {
		return err
	}
	atom.Episodes = append([]model.Episode{episode}, atom.Episodes...)
	return savePodcastSpec(plan.SpecFile, atom)
}

func savePodcastSpec(path string, atom *model.Podcast) error {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	if err := encoder.Encode(atom); err != nil {
		encoder.Close()
		return fmt.Errorf("unable to marshall yaml: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("unable to marshall yaml: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func validateNewEpisode(atom *model.Podcast, episode *model.Episode) error {
	var missing []string
	if episode.UID <= 0 {
		missing = append(missing, "uid")
	}
	if strings.TrimSpace(episode.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(episode.Link) == "" {
		missing = append(missing, "link")
	}
	if strings.TrimSpace(episode.Subtitle) == "" {
		missing = append(missing, "subtitle")
	}
	if strings.TrimSpace(episode.Description) == "" {
		missing = append(missing, "description")
	}
	if strings.TrimSpace(episode.Input) == "" {
		missing = append(missing, "input")
	}
	if strings.TrimSpace(spec.EffectiveEpisodeAuthor(atom, episode)) == "" {
		missing = append(missing, "author")
	}
	if err := spec.ApplyEpisodeDefaultsForEncoding(atom, episode); err != nil {
		missing = append(missing, strings.TrimPrefix(err.Error(), "episode "))
	}
	if atom.ContainsEpisode(episode.UID) >= 0 {
		return fmt.Errorf("episode UID %d already exists", episode.UID)
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required new episode fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateNonInteractiveNewEpisodeFlags(inputs newEpisodeInputs) error {
	var missing []string
	if strings.TrimSpace(inputs.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(inputs.Link) == "" {
		missing = append(missing, "link")
	}
	if strings.TrimSpace(inputs.Subtitle) == "" {
		missing = append(missing, "subtitle")
	}
	if strings.TrimSpace(inputs.Description) == "" && !inputs.DescriptionFromClipboard {
		missing = append(missing, "description")
	}
	if strings.TrimSpace(inputs.Input) == "" {
		missing = append(missing, "input")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required --non-interactive flags: --%s", strings.Join(missing, ", --"))
	}
	return nil
}

func loadChaptersFile(path string) ([]id3v24.Chapter, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read chapters file %s: %w", path, err)
	}
	var decoded blenderChaptersFile
	if err := yaml.Unmarshal(content, &decoded); err != nil {
		return nil, fmt.Errorf("decode chapters file %s: %w", path, err)
	}
	if len(decoded.Chapters) == 0 {
		return nil, fmt.Errorf("chapters file %s must contain non-empty chapters list", path)
	}
	for i, chapter := range decoded.Chapters {
		if strings.TrimSpace(chapter.Title) == "" {
			return nil, fmt.Errorf("chapters file %s chapter %d missing title", path, i+1)
		}
		if strings.TrimSpace(chapter.Start) == "" {
			return nil, fmt.Errorf("chapters file %s chapter %d missing start", path, i+1)
		}
	}
	return decoded.Chapters, nil
}

func readClipboardDescription() (string, error) {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-o")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("read clipboard with xclip: %w: %s", err, msg)
		}
		return "", fmt.Errorf("read clipboard with xclip: %w", err)
	}
	description := strings.TrimSpace(string(out))
	if description == "" {
		return "", errors.New("clipboard description is empty")
	}
	return description, nil
}

func validateInt64String(value string) error {
	uid, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || uid <= 0 {
		return fmt.Errorf("must be a positive integer")
	}
	return nil
}

func printNewEpisodePlan(plan *newEpisodePlan) {
	fmt.Println("Workflow: new")
	fmt.Printf("Spec: %s\n", plan.SpecFile)
	fmt.Printf("Episode: %d\n", plan.Episode.UID)
	fmt.Printf("Title: %s\n", plan.Episode.Title)
	fmt.Printf("Input: %s\n", plan.Episode.Input)
	fmt.Printf("Link: %s\n", plan.Episode.Link)
	if len(plan.Episode.Chapters) > 0 {
		fmt.Printf("Chapters: %d\n", len(plan.Episode.Chapters))
	}
}

func init() {
	rootCmd.AddCommand(newCmd)
	planCmd.AddCommand(planNewCmd)
	addNewEpisodeFlags(newCmd)
	addNewEpisodeFlags(planNewCmd)
	addPlanOutputFlag(newCmd)
	addPlanOutputFlag(planNewCmd)
}

func addNewEpisodeFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("spec", "s", spec.DefaultSpecfile, "Podcast specification file")
	cmd.Flags().BoolP("non-interactive", "n", false, "Do not open the interactive form; require all non-inferred fields as flags")
	cmd.Flags().String("uid", "", "Episode UID; defaults to previous maximum UID plus one")
	cmd.Flags().String("author", "", "Episode author; defaults to previous episode or podcast author")
	cmd.Flags().String("title", "", "Episode title")
	cmd.Flags().String("link", "", "Episode link")
	cmd.Flags().String("subtitle", "", "Episode subtitle")
	cmd.Flags().String("description", "", "Episode description")
	cmd.Flags().Bool("description-from-clipboard", false, "Read episode description from xclip clipboard; overrides --description")
	cmd.Flags().String("image", "", "Episode image; defaults to previous episode or podcast default image")
	cmd.Flags().String("input", "", "Episode input path relative to localStorageDir")
	cmd.Flags().String("format", "", "Episode format override, for example mp3, m4a, m4b, audio, or video")
	cmd.Flags().String("encoding-language", "", "Episode encoding language override")
	cmd.Flags().String("chapters", "", "Blender-exported YAML chapters file")
}

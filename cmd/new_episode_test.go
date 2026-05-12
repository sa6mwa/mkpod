package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/spec"
)

func TestDefaultNewEpisodeInputsUsePreviousEpisodeTemplate(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	atom.Episodes[0].Author = "Previous Author"
	atom.Episodes[0].Image = "artwork/previous.jpg"
	atom.Episodes[0].Input = "audiopod/masters/previous.flac"
	atom.Episodes[0].Format = "m4a"
	atom.Episodes[0].EncodingLanguage = "SWE"
	atom.Encoding.Language = "eng"

	defaults := defaultNewEpisodeInputs(atom, specFile)
	if defaults.UID != "2" {
		t.Fatalf("UID = %q, want 2", defaults.UID)
	}
	if defaults.Author != "Previous Author" {
		t.Fatalf("Author = %q, want previous author", defaults.Author)
	}
	if defaults.Image != "artwork/previous.jpg" {
		t.Fatalf("Image = %q, want previous image", defaults.Image)
	}
	if defaults.Input != "audiopod/masters/" {
		t.Fatalf("Input = %q, want input directory", defaults.Input)
	}
	if defaults.Format != "m4a" || defaults.EncodingLanguage != "SWE" {
		t.Fatalf("format/language = %q/%q, want m4a/SWE", defaults.Format, defaults.EncodingLanguage)
	}
	if defaults.InheritedEncodingLanguage != "eng" {
		t.Fatalf("InheritedEncodingLanguage = %q, want eng", defaults.InheritedEncodingLanguage)
	}
}

func TestDefaultNewEpisodeInputsDoesNotCopyInheritedEncodingLanguage(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	atom.Encoding.Language = "swe"

	defaults := defaultNewEpisodeInputs(atom, specFile)
	if defaults.EncodingLanguage != "" {
		t.Fatalf("EncodingLanguage = %q, want empty explicit override", defaults.EncodingLanguage)
	}
	if defaults.InheritedEncodingLanguage != "swe" {
		t.Fatalf("InheritedEncodingLanguage = %q, want swe", defaults.InheritedEncodingLanguage)
	}
}

func TestNewEpisodePlanFromInputsParsesChapters(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	chaptersPath := filepath.Join(t.TempDir(), "chapters.yaml")
	writeFile(t, chaptersPath, []byte("chapters:\n- title: Intro\n  start: \"00:00:00.000\"\n"))

	plan, err := newEpisodePlanFromInputs(atom, newEpisodeInputs{
		SpecFile:       specFile,
		UID:            "2",
		Title:          "New Episode",
		Link:           "https://example.com/new",
		Subtitle:       "New subtitle",
		Description:    "New description",
		Input:          "masters/new.wav",
		Author:         "Host",
		Image:          "artwork/cover.jpg",
		Chapters:       chaptersPath,
		NonInteractive: true,
	})
	if err != nil {
		t.Fatalf("newEpisodePlanFromInputs() error = %v", err)
	}
	if len(plan.Episode.Chapters) != 1 || plan.Episode.Chapters[0].Title != "Intro" {
		t.Fatalf("Chapters = %+v, want parsed chapter", plan.Episode.Chapters)
	}
}

func TestNewEpisodePlanFromInputsNormalizesLocalStoragePaths(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	input := filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.wav")
	image := filepath.Join(atom.LocalStorageDirExpanded(), "artwork", "cover.jpg")

	plan, err := newEpisodePlanFromInputs(atom, newEpisodeInputs{
		SpecFile:       specFile,
		UID:            "2",
		Title:          "New Episode",
		Link:           "https://example.com/new",
		Subtitle:       "New subtitle",
		Description:    "New description",
		Input:          input,
		Author:         "Host",
		Image:          image,
		NonInteractive: true,
	})
	if err != nil {
		t.Fatalf("newEpisodePlanFromInputs() error = %v", err)
	}
	if plan.Episode.Input != "masters/new.wav" {
		t.Fatalf("Input = %q, want relative input", plan.Episode.Input)
	}
	if plan.Episode.Image != "artwork/cover.jpg" {
		t.Fatalf("Image = %q, want relative image", plan.Episode.Image)
	}
}

func TestNewEpisodePlanFromInputsRejectsParentRelativePaths(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}

	_, err = newEpisodePlanFromInputs(atom, newEpisodeInputs{
		SpecFile:       specFile,
		UID:            "2",
		Title:          "New Episode",
		Link:           "https://example.com/new",
		Subtitle:       "New subtitle",
		Description:    "New description",
		Input:          "../outside.wav",
		Author:         "Host",
		Image:          "artwork/cover.jpg",
		NonInteractive: true,
	})
	if err == nil {
		t.Fatal("newEpisodePlanFromInputs() error = nil, want parent-relative path rejection")
	}
}

func TestNewEpisodeTUIResizeGrowsDescription(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(100, 50)
	view := tui.View()
	if !strings.Contains(view, "mkpod new episode") {
		t.Fatalf("View() missing title: %q", view)
	}
	if tui.desc.Height() < 16 {
		t.Fatalf("description height = %d, want terminal-adapted height", tui.desc.Height())
	}
}

func TestNewEpisodeTUIViewFitsTerminalHeight(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(80, 32)

	viewHeight := strings.Count(tui.View(), "\n") + 1
	if viewHeight > 32 {
		t.Fatalf("View() height = %d, want <= 32", viewHeight)
	}
}

func TestNewEpisodeTUIFieldOrderAndNoArrowFocus(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(100, 40)
	view := tui.View()
	if strings.Contains(view, ">Title") || strings.Contains(view, ">Input") {
		t.Fatalf("View() contains arrow focus marker: %q", view)
	}
	title := strings.Index(view, "Title")
	subtitle := strings.Index(view, "Subtitle")
	link := strings.Index(view, "Link")
	if !(title >= 0 && subtitle > title && link > subtitle) {
		t.Fatalf("field order title=%d subtitle=%d link=%d in view %q", title, subtitle, link, view)
	}
}

func TestNewEpisodeTUITabFollowsVisualRowOrder(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.focusField(newEpisodeFieldUID)

	model, _ := tui.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := model.(newEpisodeTUIModel)
	if updated.focus != newEpisodeFieldAuthor {
		t.Fatalf("focus after uid tab = %d, want author", updated.focus)
	}
	model, _ = updated.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated = model.(newEpisodeTUIModel)
	if updated.focus != newEpisodeFieldTitle {
		t.Fatalf("focus after author tab = %d, want title", updated.focus)
	}
}

func TestNewEpisodeTUITabDoesNotOpenFilePicker(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.focusField(newEpisodeFieldImage)

	model, _ := tui.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := model.(newEpisodeTUIModel)
	if updated.picking {
		t.Fatal("tab opened file picker, want focus navigation only")
	}
	if updated.focus != newEpisodeFieldInput {
		t.Fatalf("focus = %d, want input field", updated.focus)
	}
}

func TestNewEpisodeTUIEnterOpensFilePickerScreen(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(80, 32)
	tui.focusField(newEpisodeFieldImage)

	model, _ := tui.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(newEpisodeTUIModel)
	if !updated.picking {
		t.Fatal("enter did not open file picker")
	}
	view := updated.View()
	if viewHeight := strings.Count(view, "\n") + 1; viewHeight != 32 {
		t.Fatalf("picker view height = %d, want 32", viewHeight)
	}
	if updated.picker == nil {
		t.Fatal("picker is nil")
	}
	if !strings.Contains(view, "No files found.") && !strings.Contains(view, "Image") {
		t.Fatalf("picker view missing huh file picker content: %q", view)
	}
}

func TestNewEpisodeTUIPickerOverlaysVisualRows(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(80, 32)
	tui.focusField(newEpisodeFieldInput)

	model, _ := tui.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(newEpisodeTUIModel)
	view := updated.View()
	picker := strings.Index(view, "from localStorageDir")
	description := strings.Index(view, "Description")
	if picker < 0 {
		t.Fatalf("picker overlay missing localStorageDir description: %q", view)
	}
	if description >= 0 && picker > description {
		t.Fatalf("picker rendered below description, want visual overlay before it: picker=%d description=%d view=%q", picker, description, view)
	}
	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[18], "Input") {
		t.Fatalf("picker did not align to input row: line 18 = %q view=%q", lines[18], view)
	}
}

func TestNewEpisodeTUIPickerShowsMultipleFilesAndClosesOnSelection(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	writeFile(t, filepath.Join(atom.LocalStorageDirExpanded(), "artwork", "qzj-1000x1000-english.jpg"), []byte("jpeg"))
	writeFile(t, filepath.Join(atom.LocalStorageDirExpanded(), "artwork", "qzj-3000x3000-english.jpg"), []byte("jpeg"))
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(80, 32)
	tui.focusField(newEpisodeFieldImage)

	model, _ := tui.Update(tea.KeyMsg{Type: tea.KeyEnter})
	picking := model.(newEpisodeTUIModel)
	view := picking.View()
	for _, name := range []string{"cover.jpg", "qzj-1000x1000-english.jpg", "qzj-3000x3000-english.jpg"} {
		if !strings.Contains(view, name) {
			t.Fatalf("picker view missing %q: %q", name, view)
		}
	}

	model, _ = picking.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected := model.(newEpisodeTUIModel)
	if selected.picking {
		t.Fatalf("picker stayed open after selection: %q", selected.View())
	}
	if got := selected.fields[newEpisodeFieldImage].Value(); got != "artwork/cover.jpg" {
		t.Fatalf("image field after selection = %q, want artwork/cover.jpg", got)
	}
}

func TestNewEpisodeTUIPickerStartsInsideLocalStorage(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.fields[newEpisodeFieldImage].SetValue("artwork/cover.jpg")

	got := tui.pickerStartDirectory(newEpisodeFieldImage)
	want := filepath.Join(atom.LocalStorageDirExpanded(), "artwork")
	if got != want {
		t.Fatalf("pickerStartDirectory() = %q, want %q", got, want)
	}
}

func TestNewEpisodeTUIPickerRejectsParentRelativeStart(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.fields[newEpisodeFieldInput].SetValue("../outside.wav")

	got := tui.pickerStartDirectory(newEpisodeFieldInput)
	if got != atom.LocalStorageDirExpanded() {
		t.Fatalf("pickerStartDirectory() = %q, want localStorageDir", got)
	}
}

func TestNewEpisodeTUIChaptersPickerStartsAtInputDirectory(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.fields[newEpisodeFieldInput].SetValue("masters/episode.wav")
	tui.fields[newEpisodeFieldChapters].SetValue("")

	got := tui.pickerStartDirectory(newEpisodeFieldChapters)
	want := filepath.Join(atom.LocalStorageDirExpanded(), "masters")
	if got != want {
		t.Fatalf("chapters pickerStartDirectory() = %q, want input dir %q", got, want)
	}
}

func TestNewEpisodeTUIChaptersPickerUsesExplicitChaptersDirectory(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	chaptersDir := filepath.Join(t.TempDir(), "chapters")
	mkdirAll(t, chaptersDir)
	writeFile(t, filepath.Join(chaptersDir, "episode.md"), []byte("- 00:00 intro\n"))
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.fields[newEpisodeFieldInput].SetValue("masters/episode.wav")
	tui.fields[newEpisodeFieldChapters].SetValue(filepath.Join(chaptersDir, "episode.md"))

	got := tui.pickerStartDirectory(newEpisodeFieldChapters)
	if got != chaptersDir {
		t.Fatalf("chapters pickerStartDirectory() = %q, want explicit chapters dir %q", got, chaptersDir)
	}
}

func TestNewEpisodeTUICollectInputs(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.fields[newEpisodeFieldTitle].SetValue("Collected Title")
	tui.fields[newEpisodeFieldInput].SetValue("masters/collected.flac")
	tui.desc.SetValue("Collected description")

	inputs := tui.collectInputs()
	if inputs.Title != "Collected Title" {
		t.Fatalf("Title = %q, want collected title", inputs.Title)
	}
	if inputs.Input != "masters/collected.flac" {
		t.Fatalf("Input = %q, want collected input", inputs.Input)
	}
	if inputs.Description != "Collected description" {
		t.Fatalf("Description = %q, want collected description", inputs.Description)
	}
}

func TestValidateNonInteractiveNewEpisodeFlagsRequiresContentFields(t *testing.T) {
	err := validateNonInteractiveNewEpisodeFlags(newEpisodeInputs{Title: "Title"})
	if err == nil {
		t.Fatal("validateNonInteractiveNewEpisodeFlags() error = nil, want missing flags")
	}
	for _, field := range []string{"link", "subtitle", "description", "input"} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("error %q missing field %q", err.Error(), field)
		}
	}
}

func TestApplyNewEpisodePlanAppendsToSpec(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}

	if err := applyNewEpisodePlan(context.Background(), plan); err != nil {
		t.Fatalf("applyNewEpisodePlan() error = %v", err)
	}
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("reload spec fixture: %v", err)
	}
	if len(atom.Episodes) != 2 {
		t.Fatalf("episodes = %d, want 2", len(atom.Episodes))
	}
	if atom.Episodes[0].UID != 2 || atom.Episodes[0].Title != "New Episode" {
		t.Fatalf("first episode = %+v, want newly prepended episode", atom.Episodes[0])
	}
}

func TestApplySavedNewEpisodePlan(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	planPath := filepath.Join(t.TempDir(), "new.plan.json")
	writeSavedPlanFixture(t, planPath, "new", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	})

	if err := applySavedPlan(context.Background(), planPath); err != nil {
		t.Fatalf("applySavedPlan() error = %v", err)
	}
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("reload spec fixture: %v", err)
	}
	if atom.Episodes[0].UID != 2 {
		t.Fatalf("first UID = %d, want 2", atom.Episodes[0].UID)
	}
}

func episodeFixtureForNewPlan(uid int64) model.Episode {
	return model.Episode{
		UID:         uid,
		Title:       "New Episode",
		Link:        "https://example.com/new",
		Subtitle:    "New subtitle",
		Description: "New description",
		Author:      "Host",
		Image:       "artwork/cover.jpg",
		Input:       "masters/new.wav",
	}
}

func specStoreLoadForTest(t *testing.T, specFile string) (*model.Podcast, error) {
	t.Helper()
	return spec.New(specFile).Load(context.Background())
}

func TestLoadChaptersFileRejectsWrongShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chapters.yaml")
	if err := os.WriteFile(path, []byte("notChapters: []\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	if _, err := loadChaptersFile(path); err == nil {
		t.Fatal("loadChaptersFile() error = nil, want wrong-shape error")
	}
}

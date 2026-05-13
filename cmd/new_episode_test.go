package cmd

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"github.com/sa6mwa/id3v24"
	"github.com/sa6mwa/mkpod/internal/app/model"
	"github.com/sa6mwa/mkpod/internal/spec"
	"github.com/spf13/cobra"
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
	if defaults.InheritedAuthor != "Host" {
		t.Fatalf("InheritedAuthor = %q, want podcast author", defaults.InheritedAuthor)
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
	if !strings.Contains(view, "Test Podcast") || !strings.Contains(view, "Prepare a plan for a new episode") {
		t.Fatalf("View() missing title: %q", view)
	}
	if tui.desc.Height() < 16 {
		t.Fatalf("description height = %d, want terminal-adapted height", tui.desc.Height())
	}
}

func TestNewEpisodeTUITitleBarSpansBodyWidth(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(80, 32)

	titleLine := tui.formLines()[0]
	got, want := ansi.StringWidth(titleLine), 80
	if got != want {
		t.Fatalf("title line width = %d, want %d", got, want)
	}
	screenLine := strings.Split(tui.View(), "\n")[0]
	if got, want := ansi.StringWidth(screenLine), 80; got != want {
		t.Fatalf("screen title line width = %d, want %d", got, want)
	}
}

func TestNewEpisodeTUIWindowSizeEventClearsImmediately(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))

	model, cmd := tui.Update(tea.WindowSizeMsg{Width: 132, Height: 36})
	if cmd == nil {
		t.Fatal("WindowSizeMsg returned nil command, want immediate clear-screen repaint command")
	}
	updated := model.(newEpisodeTUIModel)
	if updated.width != 132 || updated.height != 36 {
		t.Fatalf("size = %dx%d, want 132x36", updated.width, updated.height)
	}
	msg := cmd()
	if got, want := reflect.TypeOf(msg).String(), "tea.clearScreenMsg"; got != want {
		t.Fatalf("WindowSizeMsg command = %s, want %s", got, want)
	}
}

func TestNewEpisodeTUIFitsNarrowTerminalWidth(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(34, 44)

	if got, want := tui.bodyWidth(), 32; got != want {
		t.Fatalf("bodyWidth() = %d, want %d", got, want)
	}
	view := tui.View()
	for i, line := range strings.Split(view, "\n") {
		if got, want := ansi.StringWidth(line), 34; got > want {
			t.Fatalf("line %d width = %d, want <= %d: %q\nview:\n%s", i, got, want, line, view)
		}
	}
	for _, want := range []string{"Author", "Host", "Encoding Language"} {
		if !strings.Contains(view, want) {
			t.Fatalf("narrow view missing %q:\n%s", want, view)
		}
	}
}

func TestNewEpisodeTUIRespondsToDetachedPTYResize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNewEpisodeTUIResizePTYHelper$")
	cmd.Env = append(os.Environ(), "MKPOD_TUI_RESIZE_HELPER=1")
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 60})
	if err != nil {
		t.Fatalf("start detached pty helper: %v", err)
	}
	defer tty.Close()

	resized := make(chan error, 1)
	go func() {
		time.Sleep(150 * time.Millisecond)
		resized <- pty.Setsize(tty, &pty.Winsize{Rows: 24, Cols: 90})
	}()

	output, readErr := io.ReadAll(tty)
	if err := <-resized; err != nil {
		t.Fatalf("resize detached pty: %v", err)
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		t.Fatalf("detached pty helper timed out; output:\n%s", string(output))
	}
	if readErr != nil && !strings.Contains(readErr.Error(), "input/output") {
		t.Fatalf("read detached pty output: %v", readErr)
	}
	if waitErr != nil {
		t.Fatalf("detached pty helper failed: %v\noutput:\n%s", waitErr, string(output))
	}
	out := string(output)
	if !strings.Contains(out, "FINAL_WIDTH=90") {
		t.Fatalf("detached pty resize did not reach TUI model; output:\n%s", out)
	}
	if !strings.Contains(out, "TITLE_WIDTH=90") {
		t.Fatalf("title bar did not adapt to resized pty width; output:\n%s", out)
	}
	if !strings.Contains(out, "VIEW_MIN_WIDTH=90") {
		t.Fatalf("final resized frame did not repaint every row to the new pty width; output:\n%s", out)
	}
}

type newEpisodeResizeProbe struct {
	inner newEpisodeTUIModel
}

func (m newEpisodeResizeProbe) Init() tea.Cmd {
	return m.inner.Init()
}

func (m newEpisodeResizeProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.inner.Update(msg)
	if updated, ok := model.(newEpisodeTUIModel); ok {
		m.inner = updated
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok && size.Width >= 90 {
		return m, tea.Quit
	}
	return m, cmd
}

func (m newEpisodeResizeProbe) View() string {
	return m.inner.View()
}

func TestNewEpisodeTUIResizePTYHelper(t *testing.T) {
	if os.Getenv("MKPOD_TUI_RESIZE_HELPER") != "1" {
		t.Skip("helper only runs as a detached PTY child")
	}
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	program := tea.NewProgram(
		newEpisodeResizeProbe{inner: newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))},
		tea.WithAltScreen(),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	finalModel, err := program.Run()
	if err != nil {
		t.Fatalf("run resize helper tui: %v", err)
	}
	result, ok := finalModel.(newEpisodeResizeProbe)
	if !ok {
		t.Fatalf("resize helper returned unexpected model %T", finalModel)
	}
	viewMinWidth := result.inner.width
	for _, line := range strings.Split(result.inner.View(), "\n") {
		if width := ansi.StringWidth(line); width < viewMinWidth {
			viewMinWidth = width
		}
	}
	fmt.Fprintf(os.Stdout, "\nFINAL_WIDTH=%d\nTITLE_WIDTH=%d\nVIEW_MIN_WIDTH=%d\n", result.inner.width, ansi.StringWidth(result.inner.formLines()[0]), viewMinWidth)
}

func TestNewEpisodeTUIDescriptionBoxSpansContentWidth(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(80, 32)

	lines := strings.Split(tui.View(), "\n")
	descriptionLine := -1
	for i, line := range lines {
		if strings.Contains(line, "Description") {
			descriptionLine = i
			break
		}
	}
	if descriptionLine < 0 || descriptionLine+3 >= len(lines) {
		t.Fatalf("description block not found in view: %q", tui.View())
	}
	for _, idx := range []int{descriptionLine + 1, descriptionLine + 2, descriptionLine + 3} {
		line := lines[idx]
		if got, want := ansi.StringWidth(line), 80; got != want {
			t.Fatalf("description line %d width = %d, want %d: %q", idx, got, want, line)
		}
		if !strings.HasPrefix(line, " ") {
			t.Fatalf("description line %d missing left screen margin: %q", idx, line)
		}
		if strings.HasPrefix(line, "  ") {
			t.Fatalf("description line %d has more than one left margin: %q", idx, line)
		}
		trimmedRight := strings.TrimRight(line, " ")
		if ansi.StringWidth(trimmedRight) < 79 {
			t.Fatalf("description line %d does not reach right screen margin: visual width %d line %q", idx, ansi.StringWidth(trimmedRight), line)
		}
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

func TestNewEpisodeTUIScrollsFocusedFieldIntoSmallTerminal(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	tui.resize(80, 14)
	tui.focusField(newEpisodeFieldDescription)
	view := tui.View()
	if !strings.Contains(view, "Description") {
		t.Fatalf("View() missing focused description after scroll: %q", view)
	}
	if strings.Contains(view, "UID") && !strings.Contains(view, "Description") {
		t.Fatalf("View() did not scroll down to focused field: %q", view)
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

func TestNewEpisodeTUIShowsMarkdownAndFilePickerHints(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	tui := newNewEpisodeTUIModel(atom, defaultNewEpisodeInputs(atom, specFile))
	view := tui.View()
	for _, want := range []string{
		"episode description markdown",
		"Image enter to choose",
		"Input enter to choose",
		"Chapters File enter to choose",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q: %q", want, view)
		}
	}
}

func TestNewEpisodeTUIUsesInheritedAuthorPlaceholder(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	atom.Episodes[0].Author = ""
	defaults := defaultNewEpisodeInputs(atom, specFile)
	tui := newNewEpisodeTUIModel(atom, defaults)
	if got := tui.fields[newEpisodeFieldAuthor].Value(); got != "" {
		t.Fatalf("author value = %q, want empty explicit override", got)
	}
	if got := tui.fields[newEpisodeFieldAuthor].Placeholder; got != "Host" {
		t.Fatalf("author placeholder = %q, want inherited podcast author", got)
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
	if !strings.Contains(view, "arrows/j/k move") || !strings.Contains(view, "esc close") {
		t.Fatalf("picker view missing picker help: %q", view)
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

func TestNewEpisodeTUIShowsEmbeddedChaptersInEditMode(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	inputs := defaultNewEpisodeInputs(atom, specFile)
	inputs.ExistingChapters = []id3v24.Chapter{
		{Title: "Intro", Start: "00:00:00.000"},
		{Title: "Main", Start: "00:01:00.000"},
	}

	tui := newNewEpisodeTUIModel(atom, inputs)
	if got := tui.fields[newEpisodeFieldChapters].Value(); got != "" {
		t.Fatalf("chapters field value = %q, want empty replacement path", got)
	}
	want := "2 embedded chapters in saved plan; enter to replace"
	if got := tui.fields[newEpisodeFieldChapters].Placeholder; got != want {
		t.Fatalf("chapters placeholder = %q, want %q", got, want)
	}
	if !strings.Contains(tui.View(), want) {
		t.Fatalf("View() does not surface embedded chapters placeholder %q:\n%s", want, tui.View())
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

func TestApplyNewEpisodePlanResumesWhenExistingEpisodeMatchesPlan(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	if err := applyNewEpisodePlan(context.Background(), plan); err != nil {
		t.Fatalf("first applyNewEpisodePlan() error = %v", err)
	}

	if err := applyNewEpisodePlan(context.Background(), plan); err != nil {
		t.Fatalf("second applyNewEpisodePlan() error = %v, want resumable no-op", err)
	}

	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("reload spec fixture: %v", err)
	}
	if len(atom.Episodes) != 2 {
		t.Fatalf("episodes = %d, want no duplicate append", len(atom.Episodes))
	}
}

func TestApplyNewEpisodePlanRejectsExistingEpisodeMismatch(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	plan := &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	}
	if err := applyNewEpisodePlan(context.Background(), plan); err != nil {
		t.Fatalf("first applyNewEpisodePlan() error = %v", err)
	}

	changed := *plan
	changed.Episode.Title = "Different Episode"
	err := applyNewEpisodePlan(context.Background(), &changed)
	if err == nil {
		t.Fatal("applyNewEpisodePlan() error = nil, want existing metadata conflict")
	}
	if !strings.Contains(err.Error(), "already exists with different metadata") {
		t.Fatalf("applyNewEpisodePlan() error = %q, want metadata conflict", err)
	}
}

func TestApplySavedNewEpisodePlan(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	writeSilentWaveForNewEpisodeTest(t, filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.wav"))
	planPath := filepath.Join(t.TempDir(), "new.plan.json")
	writeSavedPlanFixture(t, planPath, "new", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	})

	if err := applySavedPlanWithOptions(context.Background(), planPath, applySavedPlanOptions{Yes: true}); err != nil {
		t.Fatalf("applySavedPlan() error = %v", err)
	}
	atom, err = specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("reload spec fixture: %v", err)
	}
	if atom.Episodes[0].UID != 2 {
		t.Fatalf("first UID = %d, want 2", atom.Episodes[0].UID)
	}
	if atom.Episodes[0].Output == "" {
		t.Fatal("new episode output is empty, want local encode metadata")
	}
	if _, err := os.Stat(filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(atom.Episodes[0].Output))); err != nil {
		t.Fatalf("encoded output missing: %v", err)
	}
	if _, err := os.Stat(atom.FeedFilePath()); err != nil {
		t.Fatalf("generated RSS missing: %v", err)
	}
	if !atom.LastBuildDate.IsZero() {
		t.Fatalf("lastBuildDate = %s, want unchanged by apply", atom.LastBuildDate.Time)
	}
}

func TestApplySavedNewEpisodePlanResumesExistingMetadataAndEncodes(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	atom, err := specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("load spec fixture: %v", err)
	}
	writeSilentWaveForNewEpisodeTest(t, filepath.Join(atom.LocalStorageDirExpanded(), "masters", "new.wav"))
	episode := episodeFixtureForNewPlan(2)
	atom.Episodes = append([]model.Episode{episode}, atom.Episodes...)
	if err := savePodcastSpec(specFile, atom); err != nil {
		t.Fatalf("save partially applied spec: %v", err)
	}
	planPath := filepath.Join(t.TempDir(), "new.plan.json")
	writeSavedPlanFixture(t, planPath, "new", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episode,
	})

	if err := applySavedPlanWithOptions(context.Background(), planPath, applySavedPlanOptions{Yes: true}); err != nil {
		t.Fatalf("applySavedPlan() error = %v", err)
	}
	atom, err = specStoreLoadForTest(t, specFile)
	if err != nil {
		t.Fatalf("reload spec fixture: %v", err)
	}
	if len(atom.Episodes) != 2 {
		t.Fatalf("episodes = %d, want no duplicate append", len(atom.Episodes))
	}
	if atom.Episodes[0].UID != 2 {
		t.Fatalf("first UID = %d, want 2", atom.Episodes[0].UID)
	}
	if atom.Episodes[0].Output == "" {
		t.Fatal("new episode output is empty, want resumed local encode metadata")
	}
	if _, err := os.Stat(filepath.Join(atom.LocalStorageDirExpanded(), filepath.FromSlash(atom.Episodes[0].Output))); err != nil {
		t.Fatalf("encoded output missing: %v", err)
	}
}

func TestEditSavedNewEpisodePlanWritesBackToSamePath(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	planPath := filepath.Join(t.TempDir(), "new.post.json")
	episode := episodeFixtureForNewPlan(2)
	episode.Chapters = []id3v24.Chapter{{Title: "Intro", Start: "00:00:00.000"}}
	writeSavedPlanFixture(t, planPath, "new", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episode,
	})

	cmd := newTestNewEpisodeCommand(t)
	mustSetFlag(t, cmd, "non-interactive", "true")
	mustSetFlag(t, cmd, "title", "Edited Episode")

	plan, out, err := buildAndWriteNewEpisodePlan(context.Background(), cmd, nil, planPath)
	if err != nil {
		t.Fatalf("buildAndWriteNewEpisodePlan() error = %v", err)
	}
	if out != planPath {
		t.Fatalf("output path = %q, want edit path %q", out, planPath)
	}
	if plan.Episode.Title != "Edited Episode" {
		t.Fatalf("plan title = %q, want edited title", plan.Episode.Title)
	}

	saved, err := loadSavedNewEpisodePlan(planPath)
	if err != nil {
		t.Fatalf("loadSavedNewEpisodePlan() error = %v", err)
	}
	if saved.Episode.Title != "Edited Episode" {
		t.Fatalf("saved title = %q, want edited title", saved.Episode.Title)
	}
	if len(saved.Episode.Chapters) != 1 || saved.Episode.Chapters[0].Title != "Intro" {
		t.Fatalf("saved chapters = %+v, want preserved chapter", saved.Episode.Chapters)
	}
}

func TestEditSavedNewEpisodePlanCanWriteToOutPath(t *testing.T) {
	specFile := writeWorkflowSpecFixture(t)
	planPath := filepath.Join(t.TempDir(), "new.post.json")
	outPath := filepath.Join(t.TempDir(), "edited.post.json")
	writeSavedPlanFixture(t, planPath, "new", &newEpisodePlan{
		SpecFile: specFile,
		Episode:  episodeFixtureForNewPlan(2),
	})

	cmd := newTestNewEpisodeCommand(t)
	mustSetFlag(t, cmd, "non-interactive", "true")
	mustSetFlag(t, cmd, "out", outPath)
	mustSetFlag(t, cmd, "subtitle", "Edited subtitle")

	_, out, err := buildAndWriteNewEpisodePlan(context.Background(), cmd, nil, planPath)
	if err != nil {
		t.Fatalf("buildAndWriteNewEpisodePlan() error = %v", err)
	}
	if out != outPath {
		t.Fatalf("output path = %q, want --out path %q", out, outPath)
	}
	original, err := loadSavedNewEpisodePlan(planPath)
	if err != nil {
		t.Fatalf("load original plan: %v", err)
	}
	if original.Episode.Subtitle == "Edited subtitle" {
		t.Fatal("original plan was modified despite --out")
	}
	edited, err := loadSavedNewEpisodePlan(outPath)
	if err != nil {
		t.Fatalf("load edited plan: %v", err)
	}
	if edited.Episode.Subtitle != "Edited subtitle" {
		t.Fatalf("edited subtitle = %q, want override", edited.Episode.Subtitle)
	}
}

func newTestNewEpisodeCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "new"}
	addNewEpisodeFlags(cmd)
	addPlanOutputFlag(cmd)
	cmd.Flags().StringP("edit", "e", "", "Edit a saved new episode plan JSON file")
	return cmd
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set flag %s: %v", name, err)
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

func writeSilentWaveForNewEpisodeTest(t *testing.T, filename string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(filename), err)
	}
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Create(%q): %v", filename, err)
	}
	defer file.Close()

	const sampleRate uint32 = 8000
	const seconds uint32 = 1
	const channels uint16 = 1
	const bitsPerSample uint16 = 16
	bytesPerSample := uint32(bitsPerSample / 8)
	dataSize := sampleRate * seconds * uint32(channels) * bytesPerSample
	byteRate := sampleRate * uint32(channels) * bytesPerSample
	blockAlign := channels * bitsPerSample / 8

	write := func(value any) {
		if err := binary.Write(file, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%q): %v", filename, err)
		}
	}
	if _, err := file.Write([]byte("RIFF")); err != nil {
		t.Fatalf("Write RIFF: %v", err)
	}
	write(uint32(36) + dataSize)
	if _, err := file.Write([]byte("WAVEfmt ")); err != nil {
		t.Fatalf("Write WAVEfmt: %v", err)
	}
	write(uint32(16))
	write(uint16(1))
	write(channels)
	write(sampleRate)
	write(byteRate)
	write(blockAlign)
	write(bitsPerSample)
	if _, err := file.Write([]byte("data")); err != nil {
		t.Fatalf("Write data: %v", err)
	}
	write(dataSize)
	if _, err := file.Write(make([]byte, dataSize)); err != nil {
		t.Fatalf("Write data bytes: %v", err)
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

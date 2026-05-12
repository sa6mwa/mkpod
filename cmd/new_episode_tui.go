package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sa6mwa/mkpod/internal/app/model"
)

const (
	newEpisodeFieldUID = iota
	newEpisodeFieldTitle
	newEpisodeFieldLink
	newEpisodeFieldSubtitle
	newEpisodeFieldAuthor
	newEpisodeFieldImage
	newEpisodeFieldInput
	newEpisodeFieldFormat
	newEpisodeFieldEncodingLanguage
	newEpisodeFieldChapters
	newEpisodeFieldDescription
	newEpisodeFieldCount
)

var newEpisodeFieldLabels = []string{
	"UID",
	"Title",
	"Link",
	"Subtitle",
	"Author",
	"Image",
	"Input",
	"Format",
	"Encoding Language",
	"Chapters File",
	"Description",
}

type newEpisodeTUIModel struct {
	atom      *model.Podcast
	inputs    newEpisodeInputs
	fields    []textinput.Model
	desc      textarea.Model
	focus     int
	width     int
	height    int
	message   string
	cancelled bool
	submitted bool
	picking   bool
	picker    filepicker.Model
	pickField int

	titleStyle       lipgloss.Style
	subtitleStyle    lipgloss.Style
	labelStyle       lipgloss.Style
	focusedStyle     lipgloss.Style
	blurredStyle     lipgloss.Style
	helpStyle        lipgloss.Style
	errorStyle       lipgloss.Style
	descriptionStyle lipgloss.Style
	pickerStyle      lipgloss.Style
}

func runNewEpisodeForm(atom *model.Podcast, inputs *newEpisodeInputs) error {
	m := newNewEpisodeTUIModel(atom, *inputs)
	finalModel, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(os.Stderr), tea.WithReportFocus()).Run()
	if err != nil {
		return err
	}
	result, ok := finalModel.(newEpisodeTUIModel)
	if !ok {
		return fmt.Errorf("new episode form returned unexpected model %T", finalModel)
	}
	if result.cancelled {
		return fmt.Errorf("new episode form cancelled")
	}
	*inputs = result.collectInputs()
	return nil
}

func newNewEpisodeTUIModel(atom *model.Podcast, inputs newEpisodeInputs) newEpisodeTUIModel {
	fields := make([]textinput.Model, newEpisodeFieldDescription)
	values := []string{
		inputs.UID,
		inputs.Title,
		inputs.Link,
		inputs.Subtitle,
		inputs.Author,
		inputs.Image,
		inputs.Input,
		inputs.Format,
		inputs.EncodingLanguage,
		inputs.Chapters,
	}
	placeholders := []string{
		"next numeric id",
		"episode title",
		"https://example.com/episode",
		"short episode subtitle",
		"episode author",
		"artwork/cover.jpg",
		"audiopod/masters/episode.flac",
		"m4a",
		inputs.InheritedEncodingLanguage,
		"optional chapters.yaml",
	}
	for i := range fields {
		field := textinput.New()
		field.Prompt = ""
		field.SetValue(values[i])
		field.Placeholder = placeholders[i]
		field.CharLimit = 0
		fields[i] = field
	}

	desc := textarea.New()
	desc.Prompt = ""
	desc.ShowLineNumbers = false
	desc.Placeholder = "episode description"
	desc.SetValue(inputs.Description)

	m := newEpisodeTUIModel{
		atom:             atom,
		inputs:           inputs,
		fields:           fields,
		desc:             desc,
		focus:            newEpisodeFieldTitle,
		width:            100,
		height:           32,
		titleStyle:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
		subtitleStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		labelStyle:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")),
		focusedStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		blurredStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		helpStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		errorStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		descriptionStyle: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1),
		pickerStyle:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1),
	}
	m.focusField(m.focus)
	m.resize(100, 32)
	return m
}

func (m newEpisodeTUIModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m newEpisodeTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
	case tea.KeyMsg:
		m.message = ""
		if m.picking {
			return m.updateFilePicker(msg)
		}
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "ctrl+s":
			m.submitted = true
			return m, tea.Quit
		case "ctrl+j", "down":
			m.focusNext()
		case "ctrl+k", "up":
			m.focusPrev()
		case "enter":
			if m.isFileField(m.focus) {
				return m, m.startFilePicker(m.focus)
			}
			if m.focus == newEpisodeFieldDescription {
				var cmd tea.Cmd
				m.desc, cmd = m.desc.Update(msg)
				return m, cmd
			}
			m.focusNext()
		case "shift+tab":
			m.focusPrev()
		case "tab":
			if m.isFileField(m.focus) {
				return m, m.startFilePicker(m.focus)
			} else {
				m.focusNext()
			}
		default:
			var cmd tea.Cmd
			if m.focus == newEpisodeFieldDescription {
				m.desc, cmd = m.desc.Update(msg)
			} else if m.focus >= 0 && m.focus < len(m.fields) {
				m.fields[m.focus], cmd = m.fields[m.focus].Update(msg)
			}
			cmds = append(cmds, cmd)
		}
	default:
		var cmd tea.Cmd
		if m.focus == newEpisodeFieldDescription {
			m.desc, cmd = m.desc.Update(msg)
		} else if m.focus >= 0 && m.focus < len(m.fields) {
			m.fields[m.focus], cmd = m.fields[m.focus].Update(msg)
		}
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m newEpisodeTUIModel) View() string {
	if m.width <= 0 {
		return ""
	}
	bodyWidth := clampInt(m.width-4, 60, 120)
	outer := lipgloss.NewStyle().Width(m.width).Padding(1, 2)

	var lines []string
	lines = append(lines,
		m.titleStyle.Render("mkpod new episode"),
		m.subtitleStyle.Render("Prepare a saved plan. Nothing is written to podspec.yaml until apply."),
		"",
		m.renderTopFields(bodyWidth),
		"",
		m.renderDescription(bodyWidth),
	)
	if m.message != "" {
		lines = append(lines, m.errorStyle.Render(m.message))
	}
	if m.picking {
		lines = append(lines, "", m.renderPicker(bodyWidth))
	}
	lines = append(lines, m.helpStyle.Render(m.helpText()))
	return outer.Render(strings.Join(lines, "\n"))
}

func (m *newEpisodeTUIModel) resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	bodyWidth := clampInt(m.width-4, 60, 120)
	inputWidth := maxInt(16, (bodyWidth/2)-22)
	for i := range m.fields {
		m.fields[i].Width = inputWidth
	}
	fullWidth := maxInt(20, bodyWidth-4)
	m.fields[newEpisodeFieldTitle].Width = fullWidth
	m.fields[newEpisodeFieldLink].Width = fullWidth
	m.fields[newEpisodeFieldSubtitle].Width = fullWidth
	m.fields[newEpisodeFieldAuthor].Width = fullWidth
	m.fields[newEpisodeFieldImage].Width = fullWidth
	m.fields[newEpisodeFieldInput].Width = fullWidth
	m.fields[newEpisodeFieldChapters].Width = fullWidth
	m.desc.SetWidth(fullWidth)

	const reservedRows = 24
	descHeight := m.height - reservedRows
	if descHeight < 6 {
		descHeight = 6
	}
	m.desc.SetHeight(descHeight)
}

func (m *newEpisodeTUIModel) focusNext() {
	next := m.focus + 1
	if next >= newEpisodeFieldCount {
		next = 0
	}
	m.focusField(next)
}

func (m *newEpisodeTUIModel) focusPrev() {
	prev := m.focus - 1
	if prev < 0 {
		prev = newEpisodeFieldCount - 1
	}
	m.focusField(prev)
}

func (m *newEpisodeTUIModel) focusField(next int) {
	for i := range m.fields {
		m.fields[i].Blur()
	}
	m.desc.Blur()
	m.focus = next
	if m.focus == newEpisodeFieldDescription {
		m.desc.Focus()
		return
	}
	if m.focus >= 0 && m.focus < len(m.fields) {
		m.fields[m.focus].Focus()
	}
}

func (m newEpisodeTUIModel) renderTopFields(width int) string {
	full := lipgloss.NewStyle().Width(width)
	leftWidth := (width - 6) / 2
	rightWidth := width - leftWidth - 6
	left := lipgloss.NewStyle().Width(leftWidth)
	right := lipgloss.NewStyle().Width(rightWidth)

	rows := []string{
		left.Render(m.renderInput(newEpisodeFieldUID)),
		full.Render(m.renderInput(newEpisodeFieldTitle)),
		full.Render(m.renderInput(newEpisodeFieldLink)),
		full.Render(m.renderInput(newEpisodeFieldSubtitle)),
		full.Render(m.renderInput(newEpisodeFieldAuthor)),
		full.Render(m.renderInput(newEpisodeFieldImage)),
		full.Render(m.renderInput(newEpisodeFieldInput)),
		lipgloss.JoinHorizontal(lipgloss.Top,
			left.Render(m.renderInput(newEpisodeFieldFormat)),
			"      ",
			right.Render(m.renderInput(newEpisodeFieldEncodingLanguage)),
		),
		full.Render(m.renderInput(newEpisodeFieldChapters)),
	}
	return strings.Join(rows, "\n\n")
}

func (m newEpisodeTUIModel) renderInput(field int) string {
	label := newEpisodeFieldLabels[field]
	if field == m.focus {
		label = ">" + label
	}
	var note string
	switch field {
	case newEpisodeFieldImage:
		note = " relative to localStorageDir"
	case newEpisodeFieldInput:
		note = " edited master, relative to localStorageDir"
	case newEpisodeFieldEncodingLanguage:
		if strings.TrimSpace(m.inputs.InheritedEncodingLanguage) != "" && strings.TrimSpace(m.fields[field].Value()) == "" {
			note = " inherits " + m.inputs.InheritedEncodingLanguage
		}
	case newEpisodeFieldChapters:
		note = " optional"
	}
	labelLine := m.labelStyle.Render(label) + m.helpStyle.Render(note)
	value := m.fields[field].View()
	if field == m.focus {
		value = m.focusedStyle.Render(value)
	} else {
		value = m.blurredStyle.Render(value)
	}
	return labelLine + "\n" + value
}

func (m newEpisodeTUIModel) renderDescription(width int) string {
	label := "Description"
	if m.focus == newEpisodeFieldDescription {
		label = ">Description"
	}
	return m.labelStyle.Render(label) + "\n" + m.descriptionStyle.Width(width-2).Render(m.desc.View())
}

func (m newEpisodeTUIModel) helpText() string {
	if m.picking {
		return "enter select/open  arrows/j/k move  backspace/left parent  esc close picker"
	}
	if m.focus == newEpisodeFieldDescription {
		return "ctrl+s save plan  esc cancel  ctrl+j/ctrl+k move fields  enter newline"
	}
	if m.isFileField(m.focus) {
		return "enter/tab choose file  ctrl+j next  ctrl+k previous  ctrl+s save plan  esc cancel"
	}
	return "enter/ctrl+j next  ctrl+k previous  ctrl+s save plan  esc cancel"
}

func (m newEpisodeTUIModel) renderPicker(width int) string {
	title := "Choose " + strings.ToLower(newEpisodeFieldLabels[m.pickField])
	if !m.pickerAllowsOutsideLocalStorage() {
		title += " from localStorageDir"
	}
	body := strings.TrimRight(m.picker.View(), "\n")
	return m.labelStyle.Render(title) + "\n" + m.pickerStyle.Width(width-2).Render(body)
}

func (m newEpisodeTUIModel) isFileField(field int) bool {
	return field == newEpisodeFieldImage || field == newEpisodeFieldInput || field == newEpisodeFieldChapters
}

func (m *newEpisodeTUIModel) startFilePicker(field int) tea.Cmd {
	picker := filepicker.New()
	picker.ShowPermissions = false
	picker.ShowSize = false
	picker.FileAllowed = true
	picker.DirAllowed = false
	picker.CurrentDirectory = m.pickerStartDirectory(field)
	picker.SetHeight(m.pickerHeight())
	m.picker = picker
	m.picking = true
	m.pickField = field
	return m.picker.Init()
}

func (m newEpisodeTUIModel) updateFilePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.picking = false
		return m, nil
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if !m.pickerAllowsOutsideLocalStorage() && !pathInsideRoot(m.localStorageRoot(), m.picker.CurrentDirectory) {
		m.picker.CurrentDirectory = m.localStorageRoot()
		cmd = m.picker.Init()
	}
	if selected, path := m.picker.DidSelectFile(msg); selected {
		value := path
		if !m.pickerAllowsOutsideLocalStorage() {
			rel, err := localStorageRelativePath(m.atom, path, strings.ToLower(newEpisodeFieldLabels[m.pickField]), true)
			if err != nil {
				m.message = err.Error()
				return m, cmd
			}
			value = rel
		}
		m.fields[m.pickField].SetValue(filepath.ToSlash(value))
		m.fields[m.pickField].CursorEnd()
		m.picking = false
	}
	return m, cmd
}

func (m newEpisodeTUIModel) pickerAllowsOutsideLocalStorage() bool {
	return m.pickField == newEpisodeFieldChapters
}

func (m newEpisodeTUIModel) pickerHeight() int {
	height := m.height / 3
	if height < 6 {
		return 6
	}
	if height > 14 {
		return 14
	}
	return height
}

func (m newEpisodeTUIModel) pickerStartDirectory(field int) string {
	value := ""
	if field >= 0 && field < len(m.fields) {
		value = strings.TrimSpace(m.fields[field].Value())
	}
	if field == newEpisodeFieldChapters {
		return existingDirectoryForPicker(value, homeDirOrDot())
	}
	root := m.localStorageRoot()
	if value == "" {
		return root
	}
	if filepath.IsAbs(value) {
		if pathInsideRoot(root, value) {
			return existingDirectoryForPicker(value, root)
		}
		return root
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return root
	}
	return existingDirectoryForPicker(filepath.Join(root, clean), root)
}

func (m newEpisodeTUIModel) localStorageRoot() string {
	if m.atom != nil && strings.TrimSpace(m.atom.LocalStorageDirExpanded()) != "" {
		return m.atom.LocalStorageDirExpanded()
	}
	return "."
}

func existingDirectoryForPicker(path, fallback string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return fallback
	}
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return path
	}
	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return fallback
}

func homeDirOrDot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func pathInsideRoot(root, path string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel))
}

func (m newEpisodeTUIModel) collectInputs() newEpisodeInputs {
	inputs := m.inputs
	inputs.UID = m.fields[newEpisodeFieldUID].Value()
	inputs.Title = m.fields[newEpisodeFieldTitle].Value()
	inputs.Link = m.fields[newEpisodeFieldLink].Value()
	inputs.Subtitle = m.fields[newEpisodeFieldSubtitle].Value()
	inputs.Author = m.fields[newEpisodeFieldAuthor].Value()
	inputs.Image = m.fields[newEpisodeFieldImage].Value()
	inputs.Input = m.fields[newEpisodeFieldInput].Value()
	inputs.Format = m.fields[newEpisodeFieldFormat].Value()
	inputs.EncodingLanguage = m.fields[newEpisodeFieldEncodingLanguage].Value()
	inputs.Chapters = m.fields[newEpisodeFieldChapters].Value()
	inputs.Description = m.desc.Value()
	return inputs
}

func clampInt(value, minValue, maxValue int) int {
	return minInt(maxInt(value, minValue), maxValue)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

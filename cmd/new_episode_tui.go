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
	"github.com/charmbracelet/x/ansi"
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
		"relative image path",
		"relative edited master",
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
			m.focusNext()
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
	if m.picking {
		return m.renderPickerScreen()
	}
	bodyWidth := clampInt(m.width-4, 60, 120)

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
	lines = append(lines, m.helpStyle.Render(m.helpText()))
	return m.renderScreen(lines)
}

func (m *newEpisodeTUIModel) resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	bodyWidth := clampInt(m.width-4, 60, 120)
	labelWidth := newEpisodeLabelWidth()
	inputWidth := maxInt(8, ((bodyWidth-6)/2)-labelWidth)
	for i := range m.fields {
		m.fields[i].Width = inputWidth
	}
	fullWidth := maxInt(20, bodyWidth-4)
	fullInputWidth := maxInt(8, fullWidth-labelWidth)
	m.fields[newEpisodeFieldTitle].Width = fullInputWidth
	m.fields[newEpisodeFieldLink].Width = fullInputWidth
	m.fields[newEpisodeFieldSubtitle].Width = fullInputWidth
	m.fields[newEpisodeFieldAuthor].Width = fullInputWidth
	m.fields[newEpisodeFieldImage].Width = fullInputWidth
	m.fields[newEpisodeFieldInput].Width = fullInputWidth
	m.fields[newEpisodeFieldChapters].Width = fullInputWidth
	m.desc.SetWidth(fullWidth)

	topHeight := lipgloss.Height(m.renderTopFields(bodyWidth))
	const outerPaddingRows = 0
	const headerRows = 3
	const descriptionChromeRows = 4
	const helpRows = 1
	fixedRows := outerPaddingRows + headerRows + topHeight + descriptionChromeRows + helpRows
	if m.message != "" {
		fixedRows++
	}
	descHeight := m.height - fixedRows
	if descHeight < 3 {
		descHeight = 3
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
		left.Render(m.renderInput(newEpisodeFieldUID, leftWidth)),
		full.Render(m.renderInput(newEpisodeFieldTitle, width)),
		full.Render(m.renderInput(newEpisodeFieldLink, width)),
		full.Render(m.renderInput(newEpisodeFieldSubtitle, width)),
		full.Render(m.renderInput(newEpisodeFieldAuthor, width)),
		full.Render(m.renderInput(newEpisodeFieldImage, width)),
		full.Render(m.renderInput(newEpisodeFieldInput, width)),
		lipgloss.JoinHorizontal(lipgloss.Top,
			left.Render(m.renderInput(newEpisodeFieldFormat, leftWidth)),
			"      ",
			right.Render(m.renderInput(newEpisodeFieldEncodingLanguage, rightWidth)),
		),
		full.Render(m.renderInput(newEpisodeFieldChapters, width)),
	}
	return strings.Join(rows, "\n")
}

func (m newEpisodeTUIModel) renderInput(field int, width int) string {
	label := newEpisodeFieldLabels[field]
	if field == m.focus {
		label = ">" + label
	}
	labelWidth := newEpisodeLabelWidth()
	labelLine := m.labelStyle.Width(labelWidth).MaxWidth(labelWidth).Render(label)
	value := m.fields[field].View()
	if field == m.focus {
		value = m.focusedStyle.Render(value)
	} else {
		value = m.blurredStyle.Render(value)
	}
	return labelLine + value
}

func newEpisodeLabelWidth() int {
	return 20
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
		return "enter choose file  tab/ctrl+j next  ctrl+k previous  ctrl+s save plan  esc cancel"
	}
	return "enter/ctrl+j next  ctrl+k previous  ctrl+s save plan  esc cancel"
}

func (m newEpisodeTUIModel) renderPickerScreen() string {
	bodyWidth := clampInt(m.width-4, 60, 120)
	title := "Choose " + strings.ToLower(newEpisodeFieldLabels[m.pickField])
	if !m.pickerAllowsOutsideLocalStorage() {
		title += " from localStorageDir"
	}
	body := strings.TrimRight(m.picker.View(), "\n")
	lines := []string{
		m.titleStyle.Render("mkpod new episode"),
		m.subtitleStyle.Render(title),
		"",
		m.helpStyle.Render("Current directory: " + filepath.ToSlash(m.picker.CurrentDirectory)),
		"",
		m.pickerStyle.Width(bodyWidth - 2).Render(body),
		m.helpStyle.Render(m.helpText()),
	}
	return m.renderScreen(lines)
}

func (m newEpisodeTUIModel) renderScreen(lines []string) string {
	width := maxInt(1, m.width)
	height := maxInt(1, m.height)
	style := lipgloss.NewStyle().Width(width).Background(lipgloss.Color("0"))
	content := strings.Join(lines, "\n")
	plainLines := strings.Split(content, "\n")
	if len(plainLines) > height {
		plainLines = plainLines[:height]
	}
	for len(plainLines) < height {
		plainLines = append(plainLines, "")
	}
	for i, line := range plainLines {
		line = ansi.Truncate(line, width, "")
		if pad := width - ansi.StringWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		plainLines[i] = style.Render(line)
	}
	return strings.Join(plainLines, "\n")
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
	height := m.height - 8
	if height < 8 {
		return 8
	}
	if height > 24 {
		return 24
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

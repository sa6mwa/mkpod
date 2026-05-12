package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/sa6mwa/mkpod/internal/app/model"
)

const (
	newEpisodeFieldUID = iota
	newEpisodeFieldTitle
	newEpisodeFieldSubtitle
	newEpisodeFieldLink
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
	"Subtitle",
	"Link",
	"Author",
	"Image",
	"Input",
	"Format",
	"Encoding Language",
	"Chapters File",
	"Description",
}

var newEpisodeFocusOrder = []int{
	newEpisodeFieldUID,
	newEpisodeFieldAuthor,
	newEpisodeFieldTitle,
	newEpisodeFieldSubtitle,
	newEpisodeFieldLink,
	newEpisodeFieldImage,
	newEpisodeFieldInput,
	newEpisodeFieldFormat,
	newEpisodeFieldEncodingLanguage,
	newEpisodeFieldChapters,
	newEpisodeFieldDescription,
}

type newEpisodeTUIModel struct {
	atom      *model.Podcast
	inputs    newEpisodeInputs
	fields    []textinput.Model
	desc      textarea.Model
	focus     int
	width     int
	height    int
	scroll    int
	message   string
	cancelled bool
	submitted bool
	picking   bool
	pickField int
	picker    *huh.FilePicker
	pickValue string

	titleStyle       lipgloss.Style
	subtitleStyle    lipgloss.Style
	labelStyle       lipgloss.Style
	focusedStyle     lipgloss.Style
	blurredStyle     lipgloss.Style
	helpStyle        lipgloss.Style
	errorStyle       lipgloss.Style
	descriptionStyle lipgloss.Style
	screenStyle      lipgloss.Style
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
		inputs.Subtitle,
		inputs.Link,
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
		"short episode subtitle",
		"https://example.com/episode",
		authorPlaceholder(inputs),
		"enter to choose image file",
		"enter to choose edited master",
		"m4a",
		inputs.InheritedEncodingLanguage,
		"enter to choose optional chapters file",
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
	desc.Placeholder = "episode description markdown"
	desc.SetValue(inputs.Description)

	m := newEpisodeTUIModel{
		atom:             atom,
		inputs:           inputs,
		fields:           fields,
		desc:             desc,
		focus:            newEpisodeFieldTitle,
		width:            100,
		height:           32,
		titleStyle:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("215")),
		subtitleStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		labelStyle:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("215")),
		focusedStyle:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("215")),
		blurredStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		helpStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		errorStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		descriptionStyle: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1),
		screenStyle:      lipgloss.NewStyle().Background(lipgloss.Color("0")),
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
		if m.picking && m.picker != nil {
			m.picker = m.picker.WithWidth(m.pickerWidth()).(*huh.FilePicker)
			m.picker = m.picker.Height(m.pickerHeight())
		}
		return m, tea.ClearScreen
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
		if m.picking {
			return m.updateFilePicker(msg)
		}
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
	lines := m.formLines()
	lines = visualLines(lines)
	lines = scrollLines(lines, m.scroll)
	if m.picking {
		if len(lines) > 0 {
			lines[len(lines)-1] = ""
		}
		lines = m.overlayPicker(lines)
	}
	return m.renderScreen(lines)
}

func (m newEpisodeTUIModel) formLines() []string {
	bodyWidth := m.bodyWidth()
	var lines []string
	lines = append(lines,
		m.renderTitleBar(),
		m.subtitleStyle.Render("Prepare a plan for a new episode. Nothing is written to podspec.yaml until apply."),
		"",
		m.renderTopFields(bodyWidth),
		"",
		m.renderDescription(m.contentWidth()),
	)
	if m.message != "" {
		lines = append(lines, m.errorStyle.Render(m.message))
	}
	lines = append(lines, m.helpStyle.Render(m.helpText()))
	return lines
}

func (m newEpisodeTUIModel) renderTitleBar() string {
	return m.titleStyle.Width(m.width).Render(" " + m.formHeaderTitle())
}

func (m newEpisodeTUIModel) formHeaderTitle() string {
	if m.atom != nil && strings.TrimSpace(m.atom.Title) != "" {
		return strings.TrimSpace(m.atom.Title)
	}
	return "New episode"
}

func (m *newEpisodeTUIModel) resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	bodyWidth := m.bodyWidth()
	inputWidth := maxInt(12, (bodyWidth-8)/2)
	for i := range m.fields {
		m.fields[i].Width = inputWidth
	}
	fullWidth := maxInt(20, bodyWidth)
	fullInputWidth := maxInt(12, fullWidth-4)
	m.fields[newEpisodeFieldTitle].Width = fullInputWidth
	m.fields[newEpisodeFieldLink].Width = fullInputWidth
	m.fields[newEpisodeFieldSubtitle].Width = fullInputWidth
	m.fields[newEpisodeFieldAuthor].Width = fullInputWidth
	m.fields[newEpisodeFieldImage].Width = fullInputWidth
	m.fields[newEpisodeFieldInput].Width = fullInputWidth
	m.fields[newEpisodeFieldChapters].Width = fullInputWidth
	m.desc.SetWidth(maxInt(20, m.contentWidth()-4))

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
	m.ensureFocusVisible()
}

func (m *newEpisodeTUIModel) focusNext() {
	m.focusField(newEpisodeAdjacentFocus(m.focus, 1))
}

func (m *newEpisodeTUIModel) focusPrev() {
	m.focusField(newEpisodeAdjacentFocus(m.focus, -1))
}

func newEpisodeAdjacentFocus(current, delta int) int {
	for i, field := range newEpisodeFocusOrder {
		if field != current {
			continue
		}
		next := i + delta
		if next < 0 {
			next = len(newEpisodeFocusOrder) - 1
		}
		if next >= len(newEpisodeFocusOrder) {
			next = 0
		}
		return newEpisodeFocusOrder[next]
	}
	return newEpisodeFieldTitle
}

func (m *newEpisodeTUIModel) focusField(next int) {
	for i := range m.fields {
		m.fields[i].Blur()
		m.fields[i].PromptStyle = lipgloss.NewStyle()
		m.fields[i].TextStyle = m.blurredStyle
		m.fields[i].PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}
	m.desc.Blur()
	m.focus = next
	if m.focus == newEpisodeFieldDescription {
		m.desc.Focus()
		m.ensureFocusVisible()
		return
	}
	if m.focus >= 0 && m.focus < len(m.fields) {
		m.fields[m.focus].Focus()
		m.fields[m.focus].PromptStyle = lipgloss.NewStyle()
		m.fields[m.focus].TextStyle = m.blurredStyle
		m.fields[m.focus].PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}
	m.ensureFocusVisible()
}

func (m newEpisodeTUIModel) renderTopFields(width int) string {
	full := lipgloss.NewStyle().Width(width)
	leftWidth := (width - 8) / 2
	rightWidth := width - leftWidth - 8
	left := lipgloss.NewStyle().Width(leftWidth)
	right := lipgloss.NewStyle().Width(rightWidth)

	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			left.Render(m.renderInput(newEpisodeFieldUID, leftWidth)),
			"        ",
			right.Render(m.renderInput(newEpisodeFieldAuthor, rightWidth)),
		),
		full.Render(m.renderInput(newEpisodeFieldTitle, width)),
		full.Render(m.renderInput(newEpisodeFieldSubtitle, width)),
		full.Render(m.renderInput(newEpisodeFieldLink, width)),
		full.Render(m.renderInput(newEpisodeFieldImage, width)),
		full.Render(m.renderInput(newEpisodeFieldInput, width)),
		lipgloss.JoinHorizontal(lipgloss.Top,
			left.Render(m.renderInput(newEpisodeFieldFormat, leftWidth)),
			"        ",
			right.Render(m.renderInput(newEpisodeFieldEncodingLanguage, rightWidth)),
		),
		full.Render(m.renderInput(newEpisodeFieldChapters, width)),
	}
	return strings.Join(rows, "\n\n")
}

func (m newEpisodeTUIModel) renderInput(field int, width int) string {
	label := newEpisodeFieldLabels[field]
	labelStyle := m.labelStyle
	if field == m.focus {
		labelStyle = m.focusedStyle.Bold(true)
	}
	labelLine := labelStyle.Render(label)
	if m.isFileField(field) {
		labelLine += " " + m.helpStyle.Render("enter to choose")
	}
	value := m.fields[field].View()
	value = lipgloss.NewStyle().Width(maxInt(8, width)).Render(value)
	return labelLine + "\n" + value
}

func newEpisodeLabelWidth() int {
	return 18
}

func (m newEpisodeTUIModel) renderDescription(width int) string {
	label := "Description"
	labelStyle := m.labelStyle
	if m.focus == newEpisodeFieldDescription {
		labelStyle = m.focusedStyle.Bold(true)
	}
	return labelStyle.Render(label) + "\n" + m.descriptionStyle.Width(maxInt(1, width-2)).Render(m.desc.View())
}

func (m newEpisodeTUIModel) helpText() string {
	if m.picking {
		return "arrows/j/k move  enter select/open  h/left parent  esc close"
	}
	if m.focus == newEpisodeFieldDescription {
		return "ctrl+s save plan  esc cancel  ctrl+j/ctrl+k move fields  enter newline"
	}
	if m.isFileField(m.focus) {
		return "enter choose file  tab/ctrl+j next  ctrl+k previous  ctrl+s save plan  esc cancel"
	}
	return "enter/ctrl+j next  ctrl+k previous  ctrl+s save plan  esc cancel"
}

func (m newEpisodeTUIModel) overlayPicker(lines []string) []string {
	body := ""
	if m.picker != nil {
		body = m.picker.View()
	}
	help := m.pickerHelpLine()
	overlay := visualLines([]string{
		m.overlayBox(clampInt(m.width-4, 60, 120), body),
		help,
	})
	if len(overlay) > 0 {
		overlay[len(overlay)-1] = help
	}
	return overlayLines(lines, overlay, m.pickerOverlayTop())
}

func (m newEpisodeTUIModel) overlayBox(width int, body string) string {
	return lipgloss.PlaceHorizontal(
		width,
		lipgloss.Center,
		lipgloss.NewStyle().Width(m.pickerWidth()).Render(body),
	)
}

func (m newEpisodeTUIModel) pickerHelpLine() string {
	return lipgloss.NewStyle().
		MarginLeft(4).
		Foreground(lipgloss.Color("244")).
		MaxWidth(maxInt(20, m.width-8)).
		Render(m.helpText())
}

func overlayLines(base []string, overlay []string, top int) []string {
	out := append([]string(nil), base...)
	if top < 0 {
		top = 0
	}
	for len(out) < top {
		out = append(out, "")
	}
	for i, line := range overlay {
		idx := top + i
		if idx < len(out) {
			out[idx] = line
			continue
		}
		out = append(out, line)
	}
	return out
}

func visualLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		out = append(out, strings.Split(line, "\n")...)
	}
	return out
}

func scrollLines(lines []string, offset int) []string {
	if offset <= 0 {
		return lines
	}
	if offset >= len(lines) {
		return []string{}
	}
	return lines[offset:]
}

func authorPlaceholder(inputs newEpisodeInputs) string {
	if strings.TrimSpace(inputs.InheritedAuthor) != "" {
		return inputs.InheritedAuthor
	}
	return "episode author"
}

func (m *newEpisodeTUIModel) ensureFocusVisible() {
	row := m.fieldVisualRow(m.focus)
	bottomMargin := 2
	visibleBottom := m.scroll + m.height - bottomMargin
	if row < m.scroll {
		m.scroll = row
	}
	if row+2 > visibleBottom {
		m.scroll = row + 2 - (m.height - bottomMargin)
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m newEpisodeTUIModel) pickerOverlayTop() int {
	top := m.fieldVisualRow(m.pickField) - m.scroll
	height := m.pickerHeight()
	maxTop := m.height - height - 1
	if maxTop < 2 {
		maxTop = 2
	}
	if top > maxTop {
		return maxTop
	}
	if top < 2 {
		return 2
	}
	return top
}

func (m newEpisodeTUIModel) fieldVisualRow(field int) int {
	const topFieldsStart = 3
	switch field {
	case newEpisodeFieldUID, newEpisodeFieldAuthor:
		return topFieldsStart
	case newEpisodeFieldTitle:
		return topFieldsStart + 3
	case newEpisodeFieldSubtitle:
		return topFieldsStart + 6
	case newEpisodeFieldLink:
		return topFieldsStart + 9
	case newEpisodeFieldImage:
		return topFieldsStart + 12
	case newEpisodeFieldInput:
		return topFieldsStart + 15
	case newEpisodeFieldFormat, newEpisodeFieldEncodingLanguage:
		return topFieldsStart + 18
	case newEpisodeFieldChapters:
		return topFieldsStart + 21
	case newEpisodeFieldDescription:
		return topFieldsStart + lipgloss.Height(m.renderTopFields(m.bodyWidth())) + 1
	default:
		return topFieldsStart
	}
}

func (m newEpisodeTUIModel) renderScreen(lines []string) string {
	contentWidth := m.contentWidth()
	height := maxInt(1, m.height)
	content := strings.Join(lines, "\n")
	plainLines := strings.Split(content, "\n")
	if len(plainLines) > height {
		plainLines = plainLines[:height]
	}
	for len(plainLines) < height {
		plainLines = append(plainLines, "")
	}
	for i, line := range plainLines {
		if i == 0 {
			plainLines[i] = ansi.Truncate(line, m.width, "")
			if pad := m.width - ansi.StringWidth(plainLines[i]); pad > 0 {
				plainLines[i] += strings.Repeat(" ", pad)
			}
			continue
		}
		if i == height-1 {
			line = ansi.Truncate(line, contentWidth, "")
			if pad := contentWidth - ansi.StringWidth(line); pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			plainLines[i] = m.renderScreenRow(" " + line + " ")
			continue
		}
		line = ansi.Truncate(line, contentWidth, "")
		if pad := contentWidth - ansi.StringWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		plainLines[i] = m.renderScreenRow(" " + line + " ")
	}
	return strings.Join(plainLines, "\n")
}

func (m newEpisodeTUIModel) renderScreenRow(line string) string {
	line = ansi.Truncate(line, m.width, "")
	if pad := m.width - ansi.StringWidth(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return m.screenStyle.Width(m.width).Render(line)
}

func (m newEpisodeTUIModel) bodyWidth() int {
	return clampInt(m.contentWidth(), 64, 118)
}

func (m newEpisodeTUIModel) contentWidth() int {
	return maxInt(1, m.width-2)
}

func (m newEpisodeTUIModel) isFileField(field int) bool {
	return field == newEpisodeFieldImage || field == newEpisodeFieldInput || field == newEpisodeFieldChapters
}

func (m *newEpisodeTUIModel) startFilePicker(field int) tea.Cmd {
	m.picking = true
	m.pickField = field
	m.pickValue = ""
	startDirectory := m.pickerStartDirectory(field)
	picker := huh.NewFilePicker().
		Title(newEpisodeFieldLabels[field]).
		Value(&m.pickValue).
		FileAllowed(true).
		DirAllowed(false).
		Picking(true)
	if !m.pickerAllowsOutsideLocalStorage() {
		picker = picker.Description("from localStorageDir")
	}
	picker = picker.WithWidth(m.pickerWidth()).(*huh.FilePicker)
	picker = picker.WithTheme(huh.ThemeCharm()).(*huh.FilePicker)
	picker = picker.WithKeyMap(huh.NewDefaultKeyMap()).(*huh.FilePicker)
	picker = picker.Height(m.pickerHeightForDirectory(field, startDirectory))
	picker = picker.CurrentDirectory(startDirectory)
	m.picker = picker
	return m.picker.Focus()
}

func (m newEpisodeTUIModel) updateFilePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "esc" {
			m.picking = false
			return m, nil
		}
	}
	if m.picker == nil {
		m.picking = false
		return m, nil
	}
	next, cmd := m.picker.Update(msg)
	if picker, ok := next.(*huh.FilePicker); ok {
		m.picker = picker
	}
	if value := m.selectedPickerValue(); value != "" {
		if !m.pickerAllowsOutsideLocalStorage() {
			rel, err := localStorageRelativePath(m.atom, value, strings.ToLower(newEpisodeFieldLabels[m.pickField]), true)
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

func (m newEpisodeTUIModel) selectedPickerValue() string {
	if value := strings.TrimSpace(m.pickValue); value != "" {
		return value
	}
	if m.picker == nil {
		return ""
	}
	value, ok := m.picker.GetValue().(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func (m newEpisodeTUIModel) pickerAllowsOutsideLocalStorage() bool {
	return m.pickField == newEpisodeFieldChapters
}

func (m newEpisodeTUIModel) pickerHeight() int {
	return m.pickerHeightForDirectory(m.pickField, m.pickerStartDirectory(m.pickField))
}

func (m newEpisodeTUIModel) pickerHeightForDirectory(field int, dir string) int {
	visibleEntries := countVisiblePickerEntries(dir)
	if visibleEntries < 3 {
		visibleEntries = 3
	}
	if visibleEntries > 8 {
		visibleEntries = 8
	}
	chrome := 2
	if field != newEpisodeFieldChapters {
		chrome++
	}
	height := visibleEntries + chrome
	top := m.fieldVisualRow(field)
	maxHeight := m.height - top - 1
	if maxHeight < 6 {
		maxHeight = 6
	}
	if height > maxHeight {
		return maxHeight
	}
	return height
}

func (m newEpisodeTUIModel) pickerWidth() int {
	return maxInt(44, m.bodyWidth()-8)
}

func (m newEpisodeTUIModel) pickerStartDirectory(field int) string {
	value := ""
	if field >= 0 && field < len(m.fields) {
		value = strings.TrimSpace(m.fields[field].Value())
	}
	if field == newEpisodeFieldChapters {
		if value == "" {
			return m.inputPickerDirectory()
		}
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

func (m newEpisodeTUIModel) inputPickerDirectory() string {
	input := strings.TrimSpace(m.fields[newEpisodeFieldInput].Value())
	if input == "" {
		return homeDirOrDot()
	}
	if filepath.IsAbs(input) {
		return existingDirectoryForPicker(input, homeDirOrDot())
	}
	clean := filepath.Clean(filepath.FromSlash(input))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return homeDirOrDot()
	}
	return existingDirectoryForPicker(filepath.Join(m.localStorageRoot(), clean), homeDirOrDot())
}

func countVisiblePickerEntries(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		count++
	}
	return count
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

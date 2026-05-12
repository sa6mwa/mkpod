package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	atom       *model.Podcast
	inputs     newEpisodeInputs
	fields     []textinput.Model
	desc       textarea.Model
	focus      int
	width      int
	height     int
	message    string
	completion string
	cancelled  bool
	submitted  bool

	titleStyle       lipgloss.Style
	subtitleStyle    lipgloss.Style
	labelStyle       lipgloss.Style
	focusedStyle     lipgloss.Style
	blurredStyle     lipgloss.Style
	helpStyle        lipgloss.Style
	errorStyle       lipgloss.Style
	borderStyle      lipgloss.Style
	descriptionStyle lipgloss.Style
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
		borderStyle:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1),
		descriptionStyle: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1),
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
		m.completion = ""
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
			if m.focus == newEpisodeFieldDescription {
				var cmd tea.Cmd
				m.desc, cmd = m.desc.Update(msg)
				return m, cmd
			}
			m.focusNext()
		case "shift+tab":
			m.focusPrev()
		case "tab":
			if m.focus == newEpisodeFieldImage || m.focus == newEpisodeFieldInput || m.focus == newEpisodeFieldChapters {
				m.completeFocusedPath()
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
	if m.completion != "" {
		lines = append(lines, m.helpStyle.Render(m.completion))
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
	m.fields[newEpisodeFieldLink].Width = fullWidth
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
	leftWidth := (width - 2) / 2
	rightWidth := width - leftWidth - 2
	left := lipgloss.NewStyle().Width(leftWidth)
	right := lipgloss.NewStyle().Width(rightWidth)

	rows := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			left.Render(m.renderInput(newEpisodeFieldUID)),
			"  ",
			right.Render(m.renderInput(newEpisodeFieldAuthor)),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			left.Render(m.renderInput(newEpisodeFieldTitle)),
			"  ",
			right.Render(m.renderInput(newEpisodeFieldSubtitle)),
		),
		full.Render(m.renderInput(newEpisodeFieldLink)),
		full.Render(m.renderInput(newEpisodeFieldImage)),
		full.Render(m.renderInput(newEpisodeFieldInput)),
		lipgloss.JoinHorizontal(lipgloss.Top,
			left.Render(m.renderInput(newEpisodeFieldFormat)),
			"  ",
			right.Render(m.renderInput(newEpisodeFieldEncodingLanguage)),
		),
		full.Render(m.renderInput(newEpisodeFieldChapters)),
	}
	return strings.Join(rows, "\n")
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
	if m.focus == newEpisodeFieldDescription {
		return "ctrl+s save plan  esc cancel  ctrl+j/ctrl+k move fields  enter newline"
	}
	if m.focus == newEpisodeFieldImage || m.focus == newEpisodeFieldInput || m.focus == newEpisodeFieldChapters {
		return "tab complete path  enter/ctrl+j next  ctrl+k previous  ctrl+s save plan  esc cancel"
	}
	return "enter/ctrl+j next  ctrl+k previous  ctrl+s save plan  esc cancel"
}

func (m *newEpisodeTUIModel) completeFocusedPath() {
	if m.focus < 0 || m.focus >= len(m.fields) {
		return
	}
	current := m.fields[m.focus].Value()
	completed, ambiguous, err := m.completePath(current, m.focus == newEpisodeFieldChapters)
	if err != nil {
		m.message = err.Error()
		return
	}
	if completed != current {
		m.fields[m.focus].SetValue(completed)
		m.fields[m.focus].CursorEnd()
	}
	if len(ambiguous) > 0 {
		m.completion = "matches: " + strings.Join(ambiguous, "  ")
	}
}

func (m newEpisodeTUIModel) completePath(current string, allowOutsideLocalStorage bool) (string, []string, error) {
	current = filepath.ToSlash(strings.TrimSpace(current))
	root := "."
	if m.atom != nil && strings.TrimSpace(m.atom.LocalStorageDirExpanded()) != "" {
		root = m.atom.LocalStorageDirExpanded()
	}
	if allowOutsideLocalStorage {
		if current == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				root = home
			}
		} else if filepath.IsAbs(current) {
			root = string(filepath.Separator)
		}
	}
	dirPart, filePart := splitCompletionPath(current)
	if !allowOutsideLocalStorage && strings.HasPrefix(filepath.Clean(filepath.FromSlash(dirPart)), "..") {
		return current, nil, fmt.Errorf("path must stay inside localStorageDir")
	}
	searchDir := filepath.Join(root, filepath.FromSlash(dirPart))
	if filepath.IsAbs(current) && allowOutsideLocalStorage {
		searchDir = filepath.Join(string(filepath.Separator), filepath.FromSlash(dirPart))
	}
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return current, nil, fmt.Errorf("complete %s: %w", current, err)
	}
	matches := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, filePart) {
			suffix := ""
			if entry.IsDir() {
				suffix = "/"
			}
			matches = append(matches, name+suffix)
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return current, nil, nil
	}
	prefix := longestCommonPrefix(matches)
	next := filepath.ToSlash(filepath.Join(dirPart, prefix))
	if dirPart == "" {
		next = prefix
	}
	if len(matches) == 1 {
		return next, nil, nil
	}
	return next, matches, nil
}

func splitCompletionPath(path string) (string, string) {
	if strings.HasSuffix(path, "/") {
		return path, ""
	}
	dir, file := filepath.Split(filepath.FromSlash(path))
	return filepath.ToSlash(strings.TrimSuffix(dir, string(filepath.Separator))), file
}

func longestCommonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, value := range values[1:] {
		for !strings.HasPrefix(value, prefix) && prefix != "" {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
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

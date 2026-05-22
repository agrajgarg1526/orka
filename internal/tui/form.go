package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FormResult struct {
	Title        string
	Agent        string
	Plugin       string
	SkipResearch bool
	Notes        string
	Cancelled    bool
}

type FormSubmitMsg struct{ Result FormResult }
type FormCancelMsg struct{}

type formStep int

const (
	stepTitle formStep = iota
	stepAgent
	stepPlugin
	stepSkipResearch
	stepNotes
	stepCount
)

var stepTitles = []string{
	"What's the task?",
	"Which agent?",
	"Use a plugin?",
	"Skip research phase?",
	"Any notes? (optional)",
}

var agentOptions = []string{"claude-code", "claude-bedrock", "codex", "codex-foundry"}
var pluginOptions = []string{"none", "superpowers", "gsd", "custom"}

type FormModel struct {
	step       formStep
	titleInput textinput.Model
	notesInput textinput.Model
	agentIdx   int
	pluginIdx  int
	skipRes    bool
	width      int
	height     int
}

func NewFormModel() FormModel {
	ti := textinput.New()
	ti.Placeholder = "e.g. Fix auth bug"
	ti.Focus()
	ti.CharLimit = 80
	ti.Width = 36

	ni := textinput.New()
	ni.Placeholder = "press enter to skip"
	ni.CharLimit = 200
	ni.Width = 36

	return FormModel{
		titleInput: ti,
		notesInput: ni,
	}
}

func (m FormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m FormModel) submit() (tea.Model, tea.Cmd) {
	result := FormResult{
		Title:        m.titleInput.Value(),
		Agent:        agentOptions[m.agentIdx],
		Plugin:       pluginOptions[m.pluginIdx],
		SkipResearch: m.skipRes,
		Notes:        m.notesInput.Value(),
	}
	return m, func() tea.Msg { return FormSubmitMsg{Result: result} }
}

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.step == stepTitle {
				return m, func() tea.Msg { return FormCancelMsg{} }
			}
			m.step--
			m = m.syncFocus()
			return m, nil

		case "enter":
			if m.step == stepTitle && m.titleInput.Value() == "" {
				return m, nil // require a title
			}
			if m.step == formStep(stepCount-1) {
				return m.submit()
			}
			m.step++
			m = m.syncFocus()
			return m, nil

		case "left", "up":
			switch m.step {
			case stepAgent:
				m.agentIdx = (m.agentIdx - 1 + len(agentOptions)) % len(agentOptions)
			case stepPlugin:
				m.pluginIdx = (m.pluginIdx - 1 + len(pluginOptions)) % len(pluginOptions)
			case stepSkipResearch:
				m.skipRes = !m.skipRes
			}
			return m, nil

		case "right", "down":
			switch m.step {
			case stepAgent:
				m.agentIdx = (m.agentIdx + 1) % len(agentOptions)
			case stepPlugin:
				m.pluginIdx = (m.pluginIdx + 1) % len(pluginOptions)
			case stepSkipResearch:
				m.skipRes = !m.skipRes
			}
			return m, nil

		case " ":
			if m.step == stepSkipResearch {
				m.skipRes = !m.skipRes
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case stepTitle:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case stepNotes:
		m.notesInput, cmd = m.notesInput.Update(msg)
	}
	return m, cmd
}

func (m FormModel) syncFocus() FormModel {
	m.titleInput.Blur()
	m.notesInput.Blur()
	switch m.step {
	case stepTitle:
		m.titleInput.Focus()
	case stepNotes:
		m.notesInput.Focus()
	}
	return m
}

func (m FormModel) View() string {
	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	innerW := 44

	// Progress bar: filled blocks for done, current, empty for ahead
	progressBar := ""
	for i := formStep(0); i < stepCount; i++ {
		var seg string
		switch {
		case i < m.step:
			seg = lipgloss.NewStyle().Foreground(colorSuccess).Render("██")
		case i == m.step:
			seg = lipgloss.NewStyle().Foreground(colorPrimary).Render("██")
		default:
			seg = lipgloss.NewStyle().Foreground(lipgloss.Color("#2D2D2D")).Render("██")
		}
		progressBar += seg
		if i < stepCount-1 {
			progressBar += " "
		}
	}

	// Step question — large, prominent
	question := lipgloss.NewStyle().
		Bold(true).
		Width(innerW).
		Foreground(lipgloss.Color("#F9FAFB")).
		Render(stepTitles[m.step])

	// Input area
	var inputArea string
	var hint string

	switch m.step {
	case stepTitle:
		inputArea = lipgloss.NewStyle().
			Width(innerW).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorPrimary).
			Render("  " + m.titleInput.View())
		hint = "enter to continue   esc to cancel"

	case stepAgent:
		inputArea = renderSelector(agentOptions, m.agentIdx, innerW)
		hint = "↑/↓ to move   enter to select"

	case stepPlugin:
		inputArea = renderSelector(pluginOptions, m.pluginIdx, innerW)
		hint = "↑/↓ to move   enter to select"

	case stepSkipResearch:
		inputArea = renderYesNo(m.skipRes, innerW)
		hint = "←/→ or space to toggle   enter to confirm"

	case stepNotes:
		inputArea = lipgloss.NewStyle().
			Width(innerW).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorPrimary).
			Render("  " + m.notesInput.View())
		hint = "enter to create   esc to go back"
	}

	// Summary of confirmed steps shown as small pills above the question
	var pills []string
	if m.step > stepTitle {
		pills = append(pills, pill(m.titleInput.Value()))
	}
	if m.step > stepAgent {
		pills = append(pills, pill(agentOptions[m.agentIdx]))
	}
	if m.step > stepPlugin {
		pills = append(pills, pill(pluginOptions[m.pluginIdx]))
	}
	if m.step > stepSkipResearch {
		if m.skipRes {
			pills = append(pills, pill("skip research"))
		} else {
			pills = append(pills, pill("with research"))
		}
	}

	pillRow := ""
	if len(pills) > 0 {
		pillRow = strings.Join(pills, " ") + "\n\n"
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		progressBar,
		"",
		pillRow+question,
		"",
		inputArea,
		"",
		"",
		StyleHelp.Render(hint),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 3).
		Width(innerW+8)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box.Render(content))
}

func renderSelector(options []string, selected int, width int) string {
	var rows []string
	for i, opt := range options {
		if i == selected {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true).
				Width(width).
				Render("  ▸ "+opt))
		} else {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(colorMuted).
				Width(width).
				Render("    "+opt))
		}
	}
	return strings.Join(rows, "\n")
}

func renderYesNo(skipRes bool, width int) string {
	yes := lipgloss.NewStyle().Padding(0, 2)
	no := lipgloss.NewStyle().Padding(0, 2)
	if skipRes {
		yes = yes.Background(colorPrimary).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		no = no.Foreground(colorMuted)
	} else {
		no = no.Background(colorPrimary).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		yes = yes.Foreground(colorMuted)
	}
	_ = width
	return lipgloss.JoinHorizontal(lipgloss.Top,
		yes.Render("Yes, skip"),
		lipgloss.NewStyle().Foreground(colorMuted).Render("  "),
		no.Render("No, include"),
	)
}

func pill(s string) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#1E1E2E")).
		Foreground(colorSuccess).
		Padding(0, 1).
		Render("✓ " + s)
}

package tui

import (
	"fmt"
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

	boxW := 52
	if boxW > w-4 {
		boxW = w - 4
	}

	// Progress dots
	dots := ""
	for i := formStep(0); i < stepCount; i++ {
		if i == m.step {
			dots += lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("●")
		} else if i < m.step {
			dots += lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
		} else {
			dots += lipgloss.NewStyle().Foreground(colorMuted).Render("○")
		}
		if i < stepCount-1 {
			dots += lipgloss.NewStyle().Foreground(colorMuted).Render(" ─ ")
		}
	}

	progress := fmt.Sprintf("Step %d of %d", int(m.step)+1, int(stepCount))

	// Step question
	question := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Render(stepTitles[m.step])

	// Step-specific input area
	var inputArea string
	var hint string

	switch m.step {
	case stepTitle:
		inputArea = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1).
			Width(boxW - 6).
			Render(m.titleInput.View())
		hint = "enter to continue   esc to cancel"

	case stepAgent:
		inputArea = renderSelector(agentOptions, m.agentIdx, boxW-6)
		hint = "←/→ or ↑/↓ to select   enter to confirm"

	case stepPlugin:
		inputArea = renderSelector(pluginOptions, m.pluginIdx, boxW-6)
		hint = "←/→ or ↑/↓ to select   enter to confirm"

	case stepSkipResearch:
		yesStyle := lipgloss.NewStyle().Padding(0, 2)
		noStyle := lipgloss.NewStyle().Padding(0, 2)
		if m.skipRes {
			yesStyle = yesStyle.Background(colorPrimary).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		} else {
			noStyle = noStyle.Background(colorPrimary).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		}
		inputArea = lipgloss.JoinHorizontal(lipgloss.Top,
			yesStyle.Render("Yes, skip"),
			lipgloss.NewStyle().Foreground(colorMuted).Render("   "),
			noStyle.Render("No, include"),
		)
		hint = "←/→ or space to toggle   enter to confirm"

	case stepNotes:
		inputArea = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1).
			Width(boxW - 6).
			Render(m.notesInput.View())
		hint = "enter to create task   esc to go back"
	}

	// Summary of previous steps
	var summary []string
	if m.step > stepTitle {
		summary = append(summary, StyleStatusMuted.Render("Task:   ")+m.titleInput.Value())
	}
	if m.step > stepAgent {
		summary = append(summary, StyleStatusMuted.Render("Agent:  ")+agentOptions[m.agentIdx])
	}
	if m.step > stepPlugin {
		summary = append(summary, StyleStatusMuted.Render("Plugin: ")+pluginOptions[m.pluginIdx])
	}
	if m.step > stepSkipResearch {
		skipLabel := "no"
		if m.skipRes {
			skipLabel = "yes"
		}
		summary = append(summary, StyleStatusMuted.Render("Skip research: ")+skipLabel)
	}

	summaryStr := ""
	if len(summary) > 0 {
		summaryStr = strings.Join(summary, "\n") + "\n\n"
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(colorMuted).Render(progress),
		"",
		dots,
		"",
		summaryStr+question,
		"",
		inputArea,
		"",
		StyleHelp.Render(hint),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 3).
		Width(boxW)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box.Render(content))
}

func renderSelector(options []string, selected int, width int) string {
	var rows []string
	for i, opt := range options {
		if i == selected {
			rows = append(rows, lipgloss.NewStyle().
				Background(colorPrimary).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				Width(width).
				Padding(0, 1).
				Render("▶  "+opt))
		} else {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(colorMuted).
				Width(width).
				Padding(0, 1).
				Render("   "+opt))
		}
	}
	return strings.Join(rows, "\n")
}

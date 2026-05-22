package tui

import (
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

type formField int

const (
	fieldTitle formField = iota
	fieldAgent
	fieldPlugin
	fieldSkipResearch
	fieldNotes
	fieldCount
)

var agentOptions = []string{"claude-code", "claude-bedrock", "codex", "codex-foundry"}
var pluginOptions = []string{"none", "superpowers", "gsd", "custom"}

type FormModel struct {
	titleInput textinput.Model
	notesInput textinput.Model
	agentIdx   int
	pluginIdx  int
	skipRes    bool
	focused    formField
}

func NewFormModel() FormModel {
	ti := textinput.New()
	ti.Placeholder = "Task title"
	ti.Focus()
	ti.CharLimit = 80
	ti.Width = 30

	ni := textinput.New()
	ni.Placeholder = "Notes (optional)"
	ni.CharLimit = 200
	ni.Width = 30

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
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return FormCancelMsg{} }

		case "ctrl+s":
			// submit from anywhere
			return m.submit()

		case "tab", "down":
			m.focused = (m.focused + 1) % fieldCount
			m = m.syncFocus()
			return m, nil

		case "shift+tab", "up":
			m.focused = (m.focused - 1 + fieldCount) % fieldCount
			m = m.syncFocus()
			return m, nil

		case "enter":
			if m.focused == fieldTitle || m.focused == fieldNotes {
				// advance to next field on enter in text inputs
				m.focused = (m.focused + 1) % fieldCount
				m = m.syncFocus()
				return m, nil
			}
			if m.focused == fieldNotes {
				return m.submit()
			}
			// on non-text fields enter advances too
			m.focused = (m.focused + 1) % fieldCount
			m = m.syncFocus()
			// if we wrapped back to title, submit instead
			if m.focused == fieldTitle {
				return m.submit()
			}
			return m, nil

		case "left":
			switch m.focused {
			case fieldAgent:
				m.agentIdx = (m.agentIdx - 1 + len(agentOptions)) % len(agentOptions)
			case fieldPlugin:
				m.pluginIdx = (m.pluginIdx - 1 + len(pluginOptions)) % len(pluginOptions)
			}
			return m, nil

		case "right":
			switch m.focused {
			case fieldAgent:
				m.agentIdx = (m.agentIdx + 1) % len(agentOptions)
			case fieldPlugin:
				m.pluginIdx = (m.pluginIdx + 1) % len(pluginOptions)
			}
			return m, nil

		case " ":
			if m.focused == fieldSkipResearch {
				m.skipRes = !m.skipRes
				return m, nil
			}
		}
	}

	// Only pass messages to the active text input
	var cmd tea.Cmd
	if m.focused == fieldTitle {
		m.titleInput, cmd = m.titleInput.Update(msg)
	} else if m.focused == fieldNotes {
		m.notesInput, cmd = m.notesInput.Update(msg)
	}
	return m, cmd
}

func (m FormModel) syncFocus() FormModel {
	m.titleInput.Blur()
	m.notesInput.Blur()
	if m.focused == fieldTitle {
		m.titleInput.Focus()
	} else if m.focused == fieldNotes {
		m.notesInput.Focus()
	}
	return m
}

func (m FormModel) View() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 3).
		Width(48)

	focused := func(s string, active bool) string {
		if active {
			return lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true).
				Render("▶ " + s)
		}
		return "  " + s
	}

	checkmark := "[ ] skip research"
	if m.skipRes {
		checkmark = "[x] skip research"
	}

	agentVal := "◀  " + agentOptions[m.agentIdx] + "  ▶"
	pluginVal := "◀  " + pluginOptions[m.pluginIdx] + "  ▶"

	rows := []string{
		StyleTitle.Render("New Task"),
		"",
		focused("Title   "+m.titleInput.View(), m.focused == fieldTitle),
		"",
		focused("Agent   "+agentVal, m.focused == fieldAgent),
		"",
		focused("Plugin  "+pluginVal, m.focused == fieldPlugin),
		"",
		focused(checkmark, m.focused == fieldSkipResearch),
		"",
		focused("Notes   "+m.notesInput.View(), m.focused == fieldNotes),
		"",
		StyleHelp.Render("tab/↓↑ navigate   ←/→ cycle options   space toggle   ctrl+s submit   esc cancel"),
	}

	content := ""
	for i, row := range rows {
		if i > 0 {
			content += "\n"
		}
		content += row
	}

	return box.Render(content)
}

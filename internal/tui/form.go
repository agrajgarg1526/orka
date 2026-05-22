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

	ni := textinput.New()
	ni.Placeholder = "Notes (optional)"
	ni.CharLimit = 200

	return FormModel{
		titleInput: ti,
		notesInput: ni,
	}
}

func (m FormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return FormCancelMsg{} }
		case "tab", "down":
			m.focused = (m.focused + 1) % fieldCount
			m = m.syncFocus()
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + fieldCount) % fieldCount
			m = m.syncFocus()
		case "left", "h":
			if m.focused == fieldAgent {
				m.agentIdx = (m.agentIdx - 1 + len(agentOptions)) % len(agentOptions)
			} else if m.focused == fieldPlugin {
				m.pluginIdx = (m.pluginIdx - 1 + len(pluginOptions)) % len(pluginOptions)
			}
		case "right", "l":
			if m.focused == fieldAgent {
				m.agentIdx = (m.agentIdx + 1) % len(agentOptions)
			} else if m.focused == fieldPlugin {
				m.pluginIdx = (m.pluginIdx + 1) % len(pluginOptions)
			}
		case " ":
			if m.focused == fieldSkipResearch {
				m.skipRes = !m.skipRes
			}
		case "enter":
			if m.focused == fieldNotes || m.focused == fieldSkipResearch {
				result := FormResult{
					Title:        m.titleInput.Value(),
					Agent:        agentOptions[m.agentIdx],
					Plugin:       pluginOptions[m.pluginIdx],
					SkipResearch: m.skipRes,
					Notes:        m.notesInput.Value(),
				}
				return m, func() tea.Msg { return FormSubmitMsg{Result: result} }
			}
		}
	}

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
		Padding(1, 2).
		Width(42)

	highlight := func(s string, active bool) string {
		if active {
			return lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(s)
		}
		return s
	}

	checkmark := "[ ]"
	if m.skipRes {
		checkmark = "[x]"
	}

	content := StyleTitle.Render("New Task") + "\n\n" +
		"Title:  " + m.titleInput.View() + "\n\n" +
		highlight("Agent:  ["+agentOptions[m.agentIdx]+"]", m.focused == fieldAgent) + "\n\n" +
		highlight("Plugin: ["+pluginOptions[m.pluginIdx]+"]", m.focused == fieldPlugin) + "\n\n" +
		highlight("Skip research: "+checkmark, m.focused == fieldSkipResearch) + "\n\n" +
		"Notes:  " + m.notesInput.View() + "\n\n" +
		StyleHelp.Render("tab/↑↓ navigate • ←→ change option • space toggle • enter confirm • esc cancel")

	return box.Render(content)
}

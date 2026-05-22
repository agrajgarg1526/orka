package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/agrajgarg/orka/internal/agent"
	"github.com/agrajgarg/orka/internal/state"
)

type BackToBoardMsg struct{}
type AgentDoneMsg struct {
	TaskID string
	Err    error
}

type TaskModel struct {
	task       state.Task
	st         *state.State
	statePath  string
	runner     *agent.Runner
	doneCh     chan error
	outputVP   viewport.Model
	editMode   bool
	notesInput textinput.Model
	confirm    *confirmDialog
	width      int
	height     int
}

func NewTaskModel(t state.Task, st *state.State, statePath string, w, h int) TaskModel {
	vp := viewport.New(w/2, h-6)
	vp.SetContent("")

	ni := textinput.New()
	ni.SetValue(t.Notes)
	ni.CharLimit = 500

	return TaskModel{
		task:       t,
		st:         st,
		statePath:  statePath,
		outputVP:   vp,
		notesInput: ni,
		width:      w,
		height:     h,
		runner:     agent.NewRunner(),
	}
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} })
}

func (m TaskModel) Init() tea.Cmd { return tickCmd() }

func (m TaskModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.confirm != nil {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "y", "Y":
				m.confirm.onYes()
				m.confirm = nil
				_ = m.st.Save(m.statePath)
			case "n", "N", "esc":
				m.confirm = nil
			}
		}
		return m, nil
	}

	if m.editMode {
		var cmd tea.Cmd
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "esc", "enter":
				m.task.Notes = m.notesInput.Value()
				for i := range m.st.Tasks {
					if m.st.Tasks[i].ID == m.task.ID {
						m.st.Tasks[i].Notes = m.task.Notes
					}
				}
				_ = m.st.Save(m.statePath)
				m.editMode = false
				return m, nil
			}
		}
		m.notesInput, cmd = m.notesInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tickMsg:
		lines := m.runner.Lines()
		m.outputVP.SetContent(strings.Join(lines, "\n"))
		m.outputVP.GotoBottom()
		return m, tickCmd()

	case AgentDoneMsg:
		if msg.Err != nil {
			errMsg := m.runner.LastError()
			if errMsg == "" {
				errMsg = msg.Err.Error()
			}
			m.st.SetTaskError(m.task.ID, errMsg)
		} else {
			next := m.task.NextPhase()
			if next != "" {
				m.st.UpdateTaskPhase(m.task.ID, next)
				m.task.Phase = next
			}
		}
		_ = m.st.Save(m.statePath)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, TaskKeys.Back):
			return m, func() tea.Msg { return BackToBoardMsg{} }
		case key.Matches(msg, TaskKeys.Edit):
			m.editMode = true
			m.notesInput.Focus()
		case key.Matches(msg, TaskKeys.Stop):
			m.confirm = &confirmDialog{
				message: "Stop the agent? (y/n)",
				onYes: func() {
					_ = m.runner.Stop()
				},
			}
		case key.Matches(msg, TaskKeys.Restart):
			_ = m.runner.Stop()
		case key.Matches(msg, TaskKeys.Advance):
			next := m.task.NextPhase()
			if next != "" {
				nextPhase := next
				m.confirm = &confirmDialog{
					message: fmt.Sprintf("Advance to %s? (y/n)", phaseLabels[nextPhase]),
					onYes: func() {
						m.st.UpdateTaskPhase(m.task.ID, nextPhase)
						m.task.Phase = nextPhase
					},
				}
			}
		case key.Matches(msg, TaskKeys.Retreat):
			prev := m.task.PrevPhase()
			if prev != "" {
				prevPhase := prev
				m.confirm = &confirmDialog{
					message: fmt.Sprintf("Retreat to %s? (y/n)", phaseLabels[prevPhase]),
					onYes: func() {
						m.st.UpdateTaskPhase(m.task.ID, prevPhase)
						m.task.Phase = prevPhase
					},
				}
			}
		}

		var vpCmd tea.Cmd
		m.outputVP, vpCmd = m.outputVP.Update(msg)
		return m, vpCmd
	}
	return m, nil
}

func (m TaskModel) View() string {
	elapsed := time.Since(m.task.PhaseStartedAt).Round(time.Second).String()

	phaseName := phaseLabels[m.task.Phase]
	next := m.task.NextPhase()
	advanceHint := ""
	if next != "" {
		advanceHint = " → " + phaseLabels[next]
	}
	header := fmt.Sprintf("← back  |  %s  |  %s%s   %s  %s",
		m.task.Title, phaseName, advanceHint, m.task.Agent, elapsed)

	statusStr := StyleStatusMuted.Render("waiting")
	if m.task.Error != nil {
		statusStr = StyleStatusError.Render("✗ " + *m.task.Error)
	}

	notesView := m.task.Notes
	if m.editMode {
		notesView = m.notesInput.View()
	}

	details := lipgloss.JoinVertical(lipgloss.Left,
		StylePaneHeader.Render("DETAILS"),
		fmt.Sprintf("Title:   %s", m.task.Title),
		fmt.Sprintf("Agent:   %s", m.task.Agent),
		fmt.Sprintf("Plugin:  %s", m.task.Plugin),
		fmt.Sprintf("Phase:   %s", phaseName),
		fmt.Sprintf("Branch:  %s", m.task.Branch),
		fmt.Sprintf("Created: %s", m.task.CreatedAt.Format("2006-01-02 15:04")),
		fmt.Sprintf("Elapsed: %s", elapsed),
		fmt.Sprintf("Status:  %s", statusStr),
		"",
		StylePaneHeader.Render("NOTES"),
		notesView,
	)

	outputPane := lipgloss.JoinVertical(lipgloss.Left,
		StylePaneHeader.Render("AGENT OUTPUT"),
		m.outputVP.View(),
	)

	leftW := m.width / 3
	rightW := m.width - leftW - 3

	left := lipgloss.NewStyle().Width(leftW).Render(details)
	right := lipgloss.NewStyle().Width(rightW).Render(outputPane)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left,
		lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorMuted).Render(""),
		right)

	footerHelp := "L advance  H retreat  r restart  s stop  e edit notes  esc back"
	if m.confirm != nil {
		footerHelp = StyleConfirmPrompt.Render(m.confirm.message)
	}
	footer := StyleHelp.Render(footerHelp)

	return lipgloss.JoinVertical(lipgloss.Left,
		StyleTitle.Render(header),
		body,
		footer,
	)
}

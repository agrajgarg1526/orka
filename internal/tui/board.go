package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/agrajgarg/orka/internal/state"
)

type OpenTaskMsg struct{ TaskID string }
type StateChangedMsg struct{}

type BoardModel struct {
	st          *state.State
	projectID   string
	statePath   string
	colIdx      int
	rowIdx      int
	showForm    bool
	form        FormModel
	searchMode  bool
	searchQuery string
	confirm     *confirmDialog
	showHelp    bool
}

type confirmDialog struct {
	message string
	onYes   func()
}

var boardPhases = []state.Phase{
	state.PhaseToBePicked,
	state.PhaseResearch,
	state.PhasePlanning,
	state.PhaseRunning,
	state.PhaseReview,
	state.PhaseDone,
}

var phaseLabels = map[state.Phase]string{
	state.PhaseToBePicked: "TO BE PICKED",
	state.PhaseResearch:   "RESEARCH",
	state.PhasePlanning:   "PLANNING",
	state.PhaseRunning:    "RUNNING",
	state.PhaseReview:     "REVIEW",
	state.PhaseDone:       "DONE",
}

func NewBoardModel(st *state.State, projectID, statePath string) BoardModel {
	return BoardModel{
		st:        st,
		projectID: projectID,
		statePath: statePath,
	}
}

func (m BoardModel) Init() tea.Cmd { return nil }

func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showForm {
		updated, cmd := m.form.Update(msg)
		m.form = updated.(FormModel)
		switch msg := msg.(type) {
		case FormSubmitMsg:
			if msg.Result.Title != "" {
				m.st.AddTask(m.projectID, msg.Result.Title, msg.Result.Agent, msg.Result.Plugin, msg.Result.SkipResearch)
				_ = m.st.Save(m.statePath)
			}
			m.showForm = false
		case FormCancelMsg:
			m.showForm = false
		}
		return m, cmd
	}

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

	if m.searchMode {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "esc", "enter":
				m.searchMode = false
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				}
			default:
				if len(k.String()) == 1 {
					m.searchQuery += k.String()
				}
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		tasks := m.visibleTasksInCol(boardPhases[m.colIdx])
		switch {
		case key.Matches(msg, BoardKeys.Quit):
			return m, tea.Quit
		case key.Matches(msg, BoardKeys.Help):
			m.showHelp = !m.showHelp
		case key.Matches(msg, BoardKeys.New):
			m.form = NewFormModel()
			m.showForm = true
		case key.Matches(msg, BoardKeys.Search):
			m.searchMode = true
			m.searchQuery = ""
		case key.Matches(msg, BoardKeys.Left):
			if m.colIdx > 0 {
				m.colIdx--
				m.rowIdx = 0
			}
		case key.Matches(msg, BoardKeys.Right):
			if m.colIdx < len(boardPhases)-1 {
				m.colIdx++
				m.rowIdx = 0
			}
		case key.Matches(msg, BoardKeys.Up):
			if m.rowIdx > 0 {
				m.rowIdx--
			}
		case key.Matches(msg, BoardKeys.Down):
			if m.rowIdx < len(tasks)-1 {
				m.rowIdx++
			}
		case key.Matches(msg, BoardKeys.Open):
			if len(tasks) > 0 && m.rowIdx < len(tasks) {
				return m, func() tea.Msg { return OpenTaskMsg{TaskID: tasks[m.rowIdx].ID} }
			}
		case key.Matches(msg, BoardKeys.Advance):
			if len(tasks) > 0 && m.rowIdx < len(tasks) {
				t := tasks[m.rowIdx]
				next := t.NextPhase()
				if next != "" {
					taskID := t.ID
					m.confirm = &confirmDialog{
						message: fmt.Sprintf("Advance '%s' to %s? (y/n)", t.Title, phaseLabels[next]),
						onYes: func() {
							m.st.UpdateTaskPhase(taskID, next)
						},
					}
				}
			}
		case key.Matches(msg, BoardKeys.Retreat):
			if len(tasks) > 0 && m.rowIdx < len(tasks) {
				t := tasks[m.rowIdx]
				prev := t.PrevPhase()
				if prev != "" {
					taskID := t.ID
					m.confirm = &confirmDialog{
						message: fmt.Sprintf("Retreat '%s' to %s? (y/n)", t.Title, phaseLabels[prev]),
						onYes: func() {
							m.st.UpdateTaskPhase(taskID, prev)
						},
					}
				}
			}
		}
	}
	return m, nil
}

func (m BoardModel) visibleTasksInCol(phase state.Phase) []state.Task {
	tasks := m.st.TasksByPhase(m.projectID, phase)
	if m.searchQuery == "" {
		return tasks
	}
	var filtered []state.Task
	q := strings.ToLower(m.searchQuery)
	for _, t := range tasks {
		if strings.Contains(strings.ToLower(t.Title), q) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func (m BoardModel) View() string {
	if m.showForm {
		return lipgloss.Place(80, 24, lipgloss.Center, lipgloss.Center, m.form.View())
	}

	cols := make([]string, len(boardPhases))
	for i, phase := range boardPhases {
		tasks := m.visibleTasksInCol(phase)
		label := fmt.Sprintf("%s (%d)", phaseLabels[phase], len(tasks))
		header := StyleColumnHeader.Render(label)

		var cards []string
		for j, t := range tasks {
			selected := i == m.colIdx && j == m.rowIdx
			cards = append(cards, renderCard(t, selected))
		}

		col := lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, cards...)...)
		cols[i] = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(colorMuted).
			PaddingRight(1).
			Render(col)
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	statusBar := StyleHelp.Render("n new  L advance  H retreat  enter open  / search  ? help  q quit")
	if m.searchMode {
		statusBar = StyleStatusLive.Render("search: ") + m.searchQuery + "█"
	}
	if m.confirm != nil {
		statusBar = StyleConfirmPrompt.Render(m.confirm.message)
	}

	if m.showHelp {
		helpText := "Board keys:\n" +
			"  n        new task\n" +
			"  enter    open task view\n" +
			"  L        advance phase\n" +
			"  H        retreat phase\n" +
			"  /        search tasks\n" +
			"  j/k      navigate cards\n" +
			"  h/l      navigate columns\n" +
			"  ?        toggle help\n" +
			"  q        quit"
		overlay := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2).
			Render(helpText)
		board = lipgloss.Place(80, 20, lipgloss.Center, lipgloss.Center, overlay)
	}

	header := StyleTitle.Render("orka") + StyleHelp.Render("  agent kanban")
	return lipgloss.JoinVertical(lipgloss.Left, header, board, statusBar)
}

func renderCard(t state.Task, selected bool) string {
	style := StyleCard
	if selected {
		style = StyleCardSelected
	}
	if t.Error != nil {
		style = StyleCardError
	}

	elapsed := time.Since(t.PhaseStartedAt).Round(time.Minute).String()

	statusLine := StyleStatusMuted.Render(elapsed)
	if t.Error != nil {
		msg := *t.Error
		if len(msg) > 16 {
			msg = msg[:16] + "…"
		}
		statusLine = StyleStatusError.Render("✗ " + msg)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		t.Title,
		StyleStatusMuted.Render(t.Agent),
		statusLine,
	)
	return style.Render(content)
}

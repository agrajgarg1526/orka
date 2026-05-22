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
	width       int
	height      int
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
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
	w := m.width
	if w == 0 {
		w = 120
	}
	h := m.height
	if h == 0 {
		h = 30
	}

	if m.showForm {
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, m.form.View())
	}

	// Layout: 1 app header + 1 header border + 1 col-header row + 1 help-border + 1 help row = 5 fixed rows
	// The remaining rows are for cards.
	boardHeight := h - 5
	if boardHeight < 4 {
		boardHeight = 4
	}

	// Each column gets an equal share of width minus 1px left-border per col (except first)
	numCols := len(boardPhases)
	colWidth := (w - (numCols - 1)) / numCols
	if colWidth < 18 {
		colWidth = 18
	}

	// Build each column as a fixed-height block.
	// Columns 1..N get a left border as the divider — this renders correctly
	// because lipgloss applies the border before width padding.
	cols := make([]string, numCols)
	for i, phase := range boardPhases {
		tasks := m.visibleTasksInCol(phase)
		isActive := i == m.colIdx

		// Header
		headerFg := lipgloss.Color("#9CA3AF") // muted white
		if isActive {
			headerFg = colorPrimary
		}
		hdr := lipgloss.NewStyle().
			Bold(true).
			Foreground(headerFg).
			Width(colWidth).
			Padding(0, 1).
			Render(fmt.Sprintf("%s (%d)", phaseLabels[phase], len(tasks)))

		// Cards — render up to boardHeight lines worth
		var rendered []string
		usedRows := 0
		for j, t := range tasks {
			if usedRows >= boardHeight {
				break
			}
			card := renderCard(t, isActive && j == m.rowIdx, colWidth-4)
			h := strings.Count(card, "\n") + 1
			if usedRows+h > boardHeight {
				break
			}
			rendered = append(rendered, card)
			usedRows += h
		}

		// Pad remaining space so all columns are the same height
		body := lipgloss.NewStyle().
			Width(colWidth).
			Height(boardHeight).
			Padding(0, 1).
			Render(strings.Join(rendered, "\n"))

		colContent := lipgloss.JoinVertical(lipgloss.Left, hdr, body)

		colStyle := lipgloss.NewStyle().Width(colWidth)
		if i > 0 {
			// Left border acts as the column divider
			colStyle = colStyle.
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorMuted)
		}
		cols[i] = colStyle.Render(colContent)
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	if m.showHelp {
		helpText := "  n        new task\n" +
			"  enter    open task\n" +
			"  L / H    advance / retreat phase\n" +
			"  j / k    navigate cards\n" +
			"  h / l    navigate columns\n" +
			"  /        search\n" +
			"  ?        close help\n" +
			"  q        quit"
		overlay := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 3).
			Render(StyleTitle.Render("keyboard shortcuts") + "\n\n" + helpText)
		board = lipgloss.Place(w, boardHeight+1, lipgloss.Center, lipgloss.Center, overlay)
	}

	// App header — bold title + subtitle, bottom border
	appHeader := lipgloss.NewStyle().
		Width(w).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		Render(StyleTitle.Render("orka") + StyleHelp.Render("  agent kanban"))

	// Help bar — always at the bottom, top border
	helpContent := "n new   L advance   H retreat   enter open   / search   ? help   q quit"
	if m.searchMode {
		helpContent = StyleStatusLive.Render("search:") + " " + m.searchQuery + "█"
	}
	if m.confirm != nil {
		helpContent = StyleConfirmPrompt.Render(m.confirm.message)
	}
	helpBar := lipgloss.NewStyle().
		Width(w).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		Render(StyleHelp.Render(helpContent))

	return lipgloss.JoinVertical(lipgloss.Left, appHeader, board, helpBar)
}

func renderCard(t state.Task, selected bool, width int) string {
	if width < 10 {
		width = 10
	}
	style := StyleCard.Width(width)
	if selected {
		style = StyleCardSelected.Width(width)
	}
	if t.Error != nil {
		style = StyleCardError.Width(width)
	}

	elapsed := time.Since(t.PhaseStartedAt).Round(time.Minute).String()

	statusLine := StyleStatusMuted.Render(elapsed)
	if t.Error != nil {
		msg := *t.Error
		if len(msg) > 20 {
			msg = msg[:20] + "…"
		}
		statusLine = StyleStatusError.Render("✗ " + msg)
	}

	title := t.Title
	if len(title) > width-2 {
		title = title[:width-3] + "…"
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		StyleStatusMuted.Render(t.Agent),
		statusLine,
	)
	return style.Render(content)
}

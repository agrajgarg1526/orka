package tui

import (
	"fmt"
	"strings"

	githubpr "github.com/agrajgarg/orka/internal/github"
	"github.com/agrajgarg/orka/internal/state"
	"github.com/agrajgarg/orka/internal/worktree"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type OpenTaskMsg struct {
	TaskID     string
	AutoLaunch bool
}
type StateChangedMsg struct{}
type BackToProjectsMsg struct{}

type BoardModel struct {
	st          *state.State
	projectID   string
	statePath   string
	colIdx      int
	rowIdx      int
	colScroll   map[int]int // scroll offset (in cards) per column index
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
	message        string
	onYes          func()
	afterYes       func() error
	errorTaskID    string
	startOnConfirm bool // task view: start agent after confirm
	resetSession   bool // task view: clear sessionStarted after confirm
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
		colScroll: make(map[int]int),
	}
}

func (m BoardModel) Init() tea.Cmd { return nil }

func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Always capture window size so both board and form know the terminal dimensions.
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sz.Width
		m.height = sz.Height
	}

	// Handle form result messages emitted as commands by the form.
	switch msg := msg.(type) {
	case FormSubmitMsg:
		var cmds []tea.Cmd
		if msg.Result.Title != "" {
			task := m.st.AddTask(
				m.projectID,
				msg.Result.Title,
				msg.Result.Description,
				msg.Result.Branch,
				msg.Result.Agent,
				msg.Result.Plugin,
				msg.Result.SkipResearch,
				msg.Result.AutoRun,
				msg.Result.PRBaseBranch,
			)
			_ = m.st.Save(m.statePath)
			// Pre-create the worktree in the background so it's ready when the user presses r.
			if msg.Result.Branch != "" {
				projectDir := ""
				for _, p := range m.st.Projects {
					if p.ID == m.projectID {
						projectDir = p.Path
						break
					}
				}
				if projectDir != "" {
					branch := msg.Result.Branch
					dir := projectDir
					cmds = append(cmds, func() tea.Msg {
						worktree.Setup(dir, branch) //nolint:errcheck
						return nil
					})
				}
			}
			if msg.Result.AutoRun {
				taskID := task.ID
				cmds = append(cmds, func() tea.Msg { return OpenTaskMsg{TaskID: taskID, AutoLaunch: true} })
			}
		}
		m.showForm = false
		return m, tea.Batch(cmds...)
	case FormCancelMsg:
		m.showForm = false
		return m, nil
	}

	if m.showForm {
		updated, cmd := m.form.Update(msg)
		m.form = updated.(FormModel)
		return m, cmd
	}

	if m.confirm != nil {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "y", "Y":
				m.confirm.onYes()
				if m.confirm.afterYes != nil {
					if err := m.confirm.afterYes(); err != nil {
						if m.confirm.errorTaskID != "" {
							m.st.SetTaskError(m.confirm.errorTaskID, err.Error())
						}
					}
				}
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
		case key.Matches(msg, BoardKeys.Back):
			return m, func() tea.Msg { return BackToProjectsMsg{} }
		case key.Matches(msg, BoardKeys.Help):
			m.showHelp = !m.showHelp
		case key.Matches(msg, BoardKeys.New):
			m.form = NewFormModel(m.width, m.height)
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
				m.clampScroll()
			}
		case key.Matches(msg, BoardKeys.Down):
			if m.rowIdx < len(tasks)-1 {
				m.rowIdx++
				m.clampScroll()
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
					projectDir := ""
					for _, p := range m.st.Projects {
						if p.ID == m.projectID {
							projectDir = p.Path
							break
						}
					}
					taskCopy := t
					m.confirm = &confirmDialog{
						message: fmt.Sprintf("Advance '%s' to %s? (y/n)", t.Title, phaseLabels[next]),
						onYes: func() {
							m.st.UpdateTaskPhase(taskID, next)
						},
						errorTaskID: taskID,
						afterYes: func() error {
							if next != state.PhaseDone || projectDir == "" {
								return nil
							}
							url, err := githubpr.EnsureTaskPR(projectDir, &taskCopy, true)
							if err != nil {
								return err
							}
							if url != "" {
								for i := range m.st.Tasks {
									if m.st.Tasks[i].ID == taskID {
										m.st.Tasks[i].PRURL = url
									}
								}
							}
							return nil
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
		case key.Matches(msg, BoardKeys.Delete):
			if len(tasks) > 0 && m.rowIdx < len(tasks) {
				t := tasks[m.rowIdx]
				taskID := t.ID
				branch := t.Branch
				projectDir := ""
				for _, p := range m.st.Projects {
					if p.ID == m.projectID {
						projectDir = p.Path
						break
					}
				}
				m.confirm = &confirmDialog{
					message: fmt.Sprintf("Delete '%s'? (y/n)", t.Title),
					onYes: func() {
						m.st.DeleteTask(taskID)
						if projectDir != "" && branch != "" {
							worktree.Remove(projectDir, branch) //nolint:errcheck
						}
						if m.rowIdx > 0 {
							m.rowIdx--
						}
					},
				}
			}
		}
	}
	return m, nil
}

// boardHeight returns the available card rows for the current terminal height.
func (m BoardModel) boardHeight() int {
	h := m.height
	if h == 0 {
		h = 30
	}
	bh := h - 5
	if bh < 4 {
		bh = 4
	}
	return bh
}

// clampScroll ensures colScroll[colIdx] keeps m.rowIdx visible.
func (m *BoardModel) clampScroll() {
	if m.colScroll == nil {
		m.colScroll = make(map[int]int)
	}
	tasks := m.visibleTasksInCol(boardPhases[m.colIdx])
	bh := m.boardHeight()

	// Compute the line-offset of each card so we know which rows are visible.
	lineOf := make([]int, len(tasks))
	line := 0
	for i, t := range tasks {
		lineOf[i] = line
		card := renderCard(t, false, 20) // width doesn't matter for line count
		line += strings.Count(card, "\n") + 1
	}

	scroll := m.colScroll[m.colIdx]

	if m.rowIdx >= len(tasks) {
		return
	}

	cardTop := lineOf[m.rowIdx]
	cardH := 1
	if m.rowIdx+1 < len(tasks) {
		cardH = lineOf[m.rowIdx+1] - cardTop
	}
	cardBot := cardTop + cardH - 1

	// Scroll down if card bottom is below the visible window.
	if cardBot >= scroll+bh {
		scroll = cardBot - bh + 1
	}
	// Scroll up if card top is above the visible window.
	if cardTop < scroll {
		scroll = cardTop
	}
	if scroll < 0 {
		scroll = 0
	}
	m.colScroll[m.colIdx] = scroll
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
		return m.form.View()
	}

	boardHeight := m.boardHeight()

	// Each column gets an equal share of width minus 1px left-border per col (except first).
	// If the terminal is too narrow to fit all columns at min width, show only the active column.
	numCols := len(boardPhases)
	const minColWidth = 12
	narrowMode := w > 0 && (w-(numCols-1))/numCols < minColWidth
	colWidth := (w - (numCols - 1)) / numCols
	if narrowMode {
		colWidth = w
	}

	// Build each column as a fixed-height block.
	// Columns 1..N get a left border as the divider — this renders correctly
	// because lipgloss applies the border before width padding.
	cols := make([]string, numCols)
	for i, phase := range boardPhases {
		tasks := m.visibleTasksInCol(phase)
		isActive := i == m.colIdx

		// In narrow mode only render the active column.
		if narrowMode && !isActive {
			cols[i] = ""
			continue
		}

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

		// Cards — render within the scroll window.
		scroll := m.colScroll[i]
		var rendered []string
		linesSeen := 0
		linesRendered := 0
		for j, t := range tasks {
			card := renderCard(t, isActive && j == m.rowIdx, colWidth-4)
			cardH := strings.Count(card, "\n") + 1
			cardEnd := linesSeen + cardH
			// Skip cards entirely above the scroll offset.
			if cardEnd <= scroll {
				linesSeen = cardEnd
				continue
			}
			// Stop if we've filled the visible area.
			if linesRendered >= boardHeight {
				break
			}
			rendered = append(rendered, card)
			linesRendered += cardH
			linesSeen = cardEnd
		}

		// Pad remaining space so all columns are the same height
		bodyText := strings.Join(rendered, "\n")
		if narrowMode && len(tasks) == 0 {
			bodyText = StyleStatusMuted.Render("no tasks here · press l to go to next column")
		}
		body := lipgloss.NewStyle().
			Width(colWidth).
			Height(boardHeight).
			Padding(0, 1).
			Render(bodyText)

		colContent := lipgloss.JoinVertical(lipgloss.Left, hdr, body)

		colStyle := lipgloss.NewStyle().Width(colWidth)
		if i > 0 && !narrowMode {
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
		helpText := "  n          new task\n" +
			"  enter      open task\n" +
			"  d          delete task\n" +
			"  l / h      advance / retreat phase\n" +
			"  ↑ / ↓      navigate cards\n" +
			"  ← / →      navigate columns\n" +
			"  /          search\n" +
			"  ?          close help\n" +
			"  q          quit"
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
	advanceLabel := ""
	retreatLabel := ""
	selectedTasks := m.visibleTasksInCol(boardPhases[m.colIdx])
	if len(selectedTasks) > 0 && m.rowIdx < len(selectedTasks) {
		t := selectedTasks[m.rowIdx]
		if next := t.NextPhase(); next != "" {
			advanceLabel = "l →" + phaseLabels[next]
		}
		if prev := t.PrevPhase(); prev != "" {
			retreatLabel = "h →" + phaseLabels[prev]
		}
	}
	phaseControls := ""
	if advanceLabel != "" || retreatLabel != "" {
		phaseControls = "   " + retreatLabel + "   " + advanceLabel
	}
	taskSelected := len(selectedTasks) > 0 && m.rowIdx < len(selectedTasks)
	taskControls := ""
	if taskSelected {
		taskControls = "   enter open   d delete"
	}
	helpContent := "n new" + phaseControls + taskControls + "   / search   ? help   esc projects   q quit"
	if narrowMode {
		helpContent = fmt.Sprintf("←/→ cols  (%d/%d)  ", m.colIdx+1, numCols) + helpContent
	}
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
	if t.Error != nil {
		style = StyleCardError.Width(width)
	}
	if selected {
		style = StyleCardSelected.Width(width)
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F9FAFB")).Width(width).Render(t.Title),
	}

	if t.Error != nil {
		msg := *t.Error
		if len(msg) > width-2 {
			msg = msg[:width-3] + "…"
		}
		lines = append(lines, StyleStatusError.Render("✗ "+msg))
	} else {
		lines = append(lines, StyleStatusMuted.Render(t.Agent))
		if t.Branch != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Render("⎇ "+t.Branch))
		}
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

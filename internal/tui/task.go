package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/agrajgarg/orka/internal/agent"
	"github.com/agrajgarg/orka/internal/config"
	"github.com/agrajgarg/orka/internal/state"
	"github.com/agrajgarg/orka/internal/worktree"
)

type BackToBoardMsg struct{}
type AgentDoneMsg struct {
	TaskID string
	Err    error
}

type TaskModel struct {
	task           state.Task
	st             *state.State
	statePath      string
	cfg            *config.Config
	sessionStarted bool // true after the first agent launch; subsequent r presses resume
	editMode       bool
	notesInput     textinput.Model
	confirm        *confirmDialog
	width          int
	height         int
}

func NewTaskModel(t state.Task, st *state.State, statePath string, cfg *config.Config, w, h int) TaskModel {
	ni := textinput.New()
	ni.SetValue(t.Notes)
	ni.CharLimit = 500

	return TaskModel{
		task:           t,
		st:             st,
		statePath:      statePath,
		cfg:            cfg,
		notesInput:     ni,
		sessionStarted: t.SessionStarted,
		width:          w,
		height:         h,
	}
}

func (m TaskModel) Init() tea.Cmd { return nil }

// isCleanExit returns true for exit codes that mean the user intentionally
// ended the session (ctrl+c = 130, ctrl+d = 0).
func isCleanExit(err error) bool {
	if err == nil {
		return true
	}
	msg := err.Error()
	return msg == "exit status 130" || msg == "signal: interrupt"
}

func isAgentPhase(p state.Phase) bool {
	switch p {
	case state.PhaseResearch, state.PhasePlanning, state.PhaseRunning, state.PhaseReview:
		return true
	}
	return false
}

// launchAgent suspends the TUI and hands the terminal to the agent process.
// On first launch it sends the task prompt; on resume it continues the last session.
// Returns the (possibly mutated) model and the tea.Cmd to execute.
func (m TaskModel) launchAgent() (TaskModel, tea.Cmd) {
	projectDir := ""
	for _, p := range m.st.Projects {
		if p.ID == m.task.ProjectID {
			projectDir = p.Path
			break
		}
	}

	dir := projectDir
	if projectDir != "" && m.task.Branch != "" {
		wtPath, err := worktree.Setup(projectDir, m.task.Branch)
		if err != nil {
			errMsg := fmt.Sprintf("worktree setup: %s", err)
			return m, func() tea.Msg {
				return AgentDoneMsg{TaskID: m.task.ID, Err: fmt.Errorf("%s", errMsg)}
			}
		}
		dir = wtPath
	}

	if _, err := exec.LookPath("tmux"); err != nil {
		errMsg := "tmux is not installed — run: brew install tmux"
		return m, func() tea.Msg {
			return AgentDoneMsg{TaskID: m.task.ID, Err: fmt.Errorf("%s", errMsg)}
		}
	}

	var cmdName string
	var args []string
	if m.sessionStarted {
		cmdName, args = agent.TmuxResume(&m.task, dir)
	} else {
		phase := m.task.Phase
		if !isAgentPhase(phase) {
			phase = state.PhaseRunning
		}
		prompt := m.cfg.ResolvePrompt(&m.task, phase)
		cmdName, args = agent.TmuxLaunch(&m.task, prompt, dir)
		m.sessionStarted = true
		m.task.SessionStarted = true
		for i := range m.st.Tasks {
			if m.st.Tasks[i].ID == m.task.ID {
				m.st.Tasks[i].SessionStarted = true
			}
		}
		_ = m.st.Save(m.statePath)
	}

	c := exec.Command(cmdName, args...)
	c.Dir = dir
	taskID := m.task.ID
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return AgentDoneMsg{TaskID: taskID, Err: err}
	})
}

func (m TaskModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.confirm != nil {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "y", "Y":
				startAgent := m.confirm.startOnConfirm
				m.confirm.onYes()
				m.confirm = nil
				_ = m.st.Save(m.statePath)
				if startAgent {
					m, cmd := m.launchAgent()
					return m, cmd
				}
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case AgentDoneMsg:
		if msg.Err != nil && !isCleanExit(msg.Err) {
			errMsg := msg.Err.Error()
			m.st.SetTaskError(m.task.ID, errMsg)
		} else {
			next := m.task.NextPhase()
			if next != "" {
				m.st.UpdateTaskPhase(m.task.ID, next)
				m.task.Phase = next
			}
		}
		_ = m.st.Save(m.statePath)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, TaskKeys.Back):
			return m, func() tea.Msg { return BackToBoardMsg{} }
		case key.Matches(msg, TaskKeys.Edit):
			m.editMode = true
			m.notesInput.Focus()
		case key.Matches(msg, TaskKeys.Stop):
			// Nothing to stop — agent runs in foreground, user exits it directly.
		case key.Matches(msg, TaskKeys.Restart):
			m, cmd := m.launchAgent()
			return m, cmd
		case key.Matches(msg, TaskKeys.Advance):
			next := m.task.NextPhase()
			if next != "" {
				nextPhase := next
				m.confirm = &confirmDialog{
					message:        fmt.Sprintf("Advance to %s? (y/n)", phaseLabels[nextPhase]),
					startOnConfirm: isAgentPhase(nextPhase),
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
	}
	return m, nil
}

// gitDiffStat returns a short summary of uncommitted changes in dir, e.g. "3 files  +42 −7".
// Returns empty string if dir is empty, not a git repo, or has no changes.
func gitDiffStat(dir string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", "diff", "--shortstat", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		// Try staged+unstaged vs initial commit
		cmd2 := exec.Command("git", "diff", "--shortstat")
		cmd2.Dir = dir
		out, err = cmd2.Output()
		if err != nil || len(out) == 0 {
			return ""
		}
	}
	return strings.TrimSpace(string(out))
}

func renderPhaseBar(current state.Phase, skipResearch bool, width int) string {
	phases := []state.Phase{
		state.PhaseResearch,
		state.PhasePlanning,
		state.PhaseRunning,
		state.PhaseReview,
		state.PhaseDone,
	}
	var segs []string
	for _, p := range phases {
		if p == state.PhaseResearch && skipResearch {
			continue
		}
		label := phaseLabels[p]
		switch {
		case p == current:
			segs = append(segs, lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("● "+label))
		case phaseIndex(p) < phaseIndex(current):
			segs = append(segs, lipgloss.NewStyle().Foreground(colorSuccess).Render("✓ "+label))
		default:
			segs = append(segs, StyleStatusMuted.Render("○ "+label))
		}
	}
	bar := strings.Join(segs, StyleStatusMuted.Render("  →  "))
	// wrap if too wide
	if lipgloss.Width(bar) > width {
		bar = strings.Join(segs, "\n")
	}
	return bar
}

func phaseIndex(p state.Phase) int {
	for i, ph := range state.PhaseOrder {
		if ph == p {
			return i
		}
	}
	return -1
}

func (m TaskModel) View() string {
	w := m.width
	if w == 0 {
		w = 120
	}
	h := m.height
	if h == 0 {
		h = 30
	}

	phaseName := phaseLabels[m.task.Phase]

	// ── header ────────────────────────────────────────────────────────────────
	phaseBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(colorPrimary).
		Padding(0, 1).
		Render(phaseName)
	headerLeft := lipgloss.JoinHorizontal(lipgloss.Center,
		StyleStatusMuted.Render("← back"),
		"  ",
		phaseBadge,
	)
	headerRight := StyleStatusMuted.Render(m.task.Agent)
	gap := w - 2 - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	if gap < 1 {
		gap = 1
	}
	headerContent := headerLeft + lipgloss.NewStyle().Render(fmt.Sprintf("%*s", gap, "")) + headerRight
	header := lipgloss.NewStyle().
		Width(w).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		Render(headerContent)

	// ── body ──────────────────────────────────────────────────────────────────
	cardW := 72
	if cardW > w-4 {
		cardW = w - 4
	}
	innerW := cardW - 6 // border(2) + padding(4)

	// resolve worktree dir for git diff
	worktreeDir := ""
	for _, p := range m.st.Projects {
		if p.ID == m.task.ProjectID {
			if m.task.Branch != "" {
				worktreeDir = worktree.WorktreePath(p.Path, m.task.Branch)
			}
			break
		}
	}

	label := lipgloss.NewStyle().Foreground(colorMuted).Width(10)
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))

	var statusStr string
	if m.task.Error != nil {
		statusStr = StyleStatusError.Render("✗  " + *m.task.Error)
	} else if isAgentPhase(m.task.Phase) {
		statusStr = StyleStatusLive.Render("● running")
	} else {
		statusStr = StyleStatusMuted.Render("○ idle")
	}

	divider := lipgloss.NewStyle().Foreground(colorMuted).Render(strings.Repeat("─", innerW))

	descContent := m.task.Description
	if descContent == "" {
		descContent = StyleStatusMuted.Render("no description")
	}

	phaseBar := renderPhaseBar(m.task.Phase, m.task.SkipResearch, innerW)

	diffStat := gitDiffStat(worktreeDir)
	var diffLine string
	if diffStat != "" {
		diffLine = lipgloss.NewStyle().Foreground(colorSuccess).Render(diffStat)
	} else {
		diffLine = StyleStatusMuted.Render("no changes")
	}

	cardRows := []string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F9FAFB")).Width(innerW).Render(m.task.Title),
		"",
		phaseBar,
		"",
		divider,
		"",
		label.Render("branch") + value.Render(m.task.Branch),
		label.Render("agent") + value.Render(m.task.Agent),
		label.Render("plugin") + value.Render(m.task.Plugin),
		label.Render("created") + value.Render(m.task.CreatedAt.Format("2006-01-02 15:04")),
		label.Render("status") + statusStr,
		"",
		divider,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("changes"),
		"",
		diffLine,
		"",
		divider,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("description"),
		"",
		descContent,
	}

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Width(cardW).
		Render(lipgloss.JoinVertical(lipgloss.Left, cardRows...))

	bodyH := h - lipgloss.Height(header) - 3
	body := lipgloss.Place(w, bodyH, lipgloss.Center, lipgloss.Top,
		lipgloss.NewStyle().MarginTop(2).Render(card),
	)

	// ── footer ────────────────────────────────────────────────────────────────
	sessionHint := StyleStatusMuted.Render("inside session: ") + lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("ctrl+q") + StyleStatusMuted.Render(" detach")

	next := m.task.NextPhase()
	prev := m.task.PrevPhase()
	advanceStr := "L advance"
	retreatStr := "H retreat"
	if next != "" {
		advanceStr = "L → " + phaseLabels[next]
	}
	if prev != "" {
		retreatStr = "H → " + phaseLabels[prev]
	}

	footerHelp := "r launch/resume   " + sessionHint + "   " + advanceStr + "   " + retreatStr + "   esc back"
	if m.confirm != nil {
		footerHelp = StyleConfirmPrompt.Render(m.confirm.message)
	}
	footer := lipgloss.NewStyle().
		Width(w).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		Render(StyleHelp.Render(footerHelp))

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

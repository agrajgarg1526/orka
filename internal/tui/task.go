package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

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
type tickMsg time.Time

type diffFile struct {
	name string
	diff string // the diff chunk for this file only
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
	tmuxOutput     string
	diffRaw        string     // full git diff output, cached on tick
	diffStat       string     // shortstat line, cached on tick
	diffFiles      []diffFile // parsed per-file diffs
	diffOpen       bool       // diff panel visible
	diffFocusRight bool       // true when diff content pane is focused (vs file list)
	diffCursor     int        // selected file index in diff panel
	diffScroll     int        // scroll offset for the file diff view
	cardScroll     int
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

func (m TaskModel) Init() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func captureTmuxPane(sessionName string, maxW int) string {
	cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p", "-S", "-")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	// Keep last 100 lines — view will trim further to fit
	if len(lines) > 100 {
		lines = lines[len(lines)-100:]
	}
	// Filter and truncate lines
	result := make([]string, 0, len(lines))
	for _, l := range lines {
		// Skip tmux UI chrome: box-drawing-only lines and the claude status bar lines
		trimmed := strings.TrimFunc(l, func(r rune) bool {
			return r == '─' || r == '│' || r == '┼' || r == '└' || r == '┘' ||
				r == '┌' || r == '┐' || r == ' ' || r == '\t'
		})
		if trimmed == "" {
			result = append(result, "")
			continue
		}
		// Skip the bypass permissions / status bar lines
		if strings.Contains(l, "bypass permissions") ||
			strings.Contains(l, "shift+tab to cycle") ||
			strings.Contains(l, "until auto-compact") {
			continue
		}
		runes := []rune(l)
		if len(runes) > maxW {
			runes = runes[:maxW-1]
			l = string(runes) + "…"
		}
		result = append(result, l)
	}
	// Trim leading/trailing blank lines from result
	start, end := 0, len(result)
	for start < end && result[start] == "" {
		start++
	}
	for end > start && result[end-1] == "" {
		end--
	}
	return strings.Join(result[start:end], "\n")
}

// isCleanExit returns true for exit codes that mean the user intentionally
// ended the session (ctrl+c = 130, ctrl+d = 0).
func isCleanExit(err error) bool {
	if err == nil {
		return true
	}
	msg := err.Error()
	return msg == "exit status 130" || msg == "signal: interrupt"
}

func agentExitError(agent string, err error) string {
	code := err.Error() // e.g. "exit status 1"
	switch code {
	case "exit status 1":
		switch agent {
		case "codex":
			return "codex exited (check auth: codex login)"
		default:
			return "agent exited unexpectedly (exit 1)"
		}
	case "exit status 2":
		return agent + ": bad arguments or missing command"
	default:
		return agent + " failed: " + code
	}
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
	// Clear any stale error whenever the agent is launched or resumed.
	m.task.Error = nil
	for i := range m.st.Tasks {
		if m.st.Tasks[i].ID == m.task.ID {
			m.st.Tasks[i].Error = nil
		}
	}

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
	}
	_ = m.st.Save(m.statePath)

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
				doReset := m.confirm.resetSession
				m.confirm.onYes()
				m.confirm = nil
				if doReset {
					m.sessionStarted = false
					m.task.SessionStarted = false
					for i := range m.st.Tasks {
						if m.st.Tasks[i].ID == m.task.ID {
							m.st.Tasks[i].SessionStarted = false
						}
					}
				}
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

	case tickMsg:
		sessionName := agent.TmuxSessionName(&m.task)
		if agent.TmuxSessionExists(sessionName) {
			m.tmuxOutput = captureTmuxPane(sessionName, m.width)
		} else {
			m.tmuxOutput = ""
		}
		worktreeDir := ""
		for _, p := range m.st.Projects {
			if p.ID == m.task.ProjectID {
				if m.task.Branch != "" {
					worktreeDir = worktree.WorktreePath(p.Path, m.task.Branch)
				}
				break
			}
		}
		m.diffStat = gitShortStat(worktreeDir)
		raw := gitDiff(worktreeDir)
		if raw != m.diffRaw {
			m.diffRaw = raw
			m.diffFiles = parseDiffFiles(raw)
			if m.diffCursor >= len(m.diffFiles) {
				m.diffCursor = 0
				m.diffScroll = 0
			}
		}
		if worktreeDir != "" {
			if actual := currentBranch(worktreeDir); actual != "" && actual != m.task.Branch {
				m.task.Branch = actual
				for i := range m.st.Tasks {
					if m.st.Tasks[i].ID == m.task.ID {
						m.st.Tasks[i].Branch = actual
					}
				}
				_ = m.st.Save(m.statePath)
			}
		}
		return m, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })

	case AgentDoneMsg:
		if msg.Err != nil && !isCleanExit(msg.Err) {
			errMsg := agentExitError(m.task.Agent, msg.Err)
			m.st.SetTaskError(m.task.ID, errMsg)
			_ = m.st.Save(m.statePath)
		}
		return m, nil

	case tea.KeyMsg:
		// Diff panel captures navigation keys when open.
		if m.diffOpen {
			switch msg.String() {
			case "esc":
				if m.diffFocusRight {
					m.diffFocusRight = false
				} else {
					m.diffOpen = false
				}
			case "d":
				m.diffOpen = false
			case "right":
				m.diffFocusRight = true
			case "left":
				m.diffFocusRight = false
			case "e":
				if len(m.diffFiles) > 0 && m.diffCursor < len(m.diffFiles) {
					worktreeDir := ""
					for _, p := range m.st.Projects {
						if p.ID == m.task.ProjectID {
							if m.task.Branch != "" {
								worktreeDir = worktree.WorktreePath(p.Path, m.task.Branch)
							} else {
								worktreeDir = p.Path
							}
							break
						}
					}
					filePath := m.diffFiles[m.diffCursor].name
					if worktreeDir != "" {
						filePath = worktreeDir + "/" + filePath
					}
					c := exec.Command("vim", filePath)
					return m, tea.ExecProcess(c, func(err error) tea.Msg { return nil })
				}
			case "down":
				if m.diffFocusRight {
					m.diffScroll++
				} else {
					if m.diffCursor < len(m.diffFiles)-1 {
						m.diffCursor++
						m.diffScroll = 0
					}
				}
			case "up":
				if m.diffFocusRight {
					if m.diffScroll > 0 {
						m.diffScroll--
					}
				} else {
					if m.diffCursor > 0 {
						m.diffCursor--
						m.diffScroll = 0
					}
				}
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, TaskKeys.Back):
			return m, func() tea.Msg { return BackToBoardMsg{} }
		case key.Matches(msg, TaskKeys.Up):
			if m.cardScroll > 0 {
				m.cardScroll--
			}
		case key.Matches(msg, TaskKeys.Down):
			m.cardScroll++
		case key.Matches(msg, TaskKeys.Edit):
			m.editMode = true
			m.notesInput.Focus()
		case key.Matches(msg, TaskKeys.Stop):
			// Nothing to stop — agent runs in foreground, user exits it directly.
		case key.Matches(msg, TaskKeys.Diff):
			if len(m.diffFiles) > 0 {
				m.diffOpen = true
				m.diffCursor = 0
				m.diffScroll = 0
			}
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
					resetSession: true,
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
					resetSession: true,
				}
			}
		}
	}
	return m, nil
}

// currentBranch returns the current git branch name in dir, or "" on error.
func currentBranch(dir string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitShortStat returns a one-line summary like "3 files changed, 42 insertions(+), 7 deletions(-)".
func gitShortStat(dir string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", "diff", "--shortstat", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		cmd2 := exec.Command("git", "diff", "--shortstat")
		cmd2.Dir = dir
		out, err = cmd2.Output()
		if err != nil || len(out) == 0 {
			return ""
		}
	}
	return strings.TrimSpace(string(out))
}

// gitDiff returns the full diff of uncommitted changes in dir.
func gitDiff(dir string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		cmd2 := exec.Command("git", "diff")
		cmd2.Dir = dir
		out, err = cmd2.Output()
		if err != nil || len(out) == 0 {
			return ""
		}
	}
	return strings.TrimRight(string(out), "\n")
}

// parseDiffFiles splits a full git diff into per-file chunks.
func parseDiffFiles(raw string) []diffFile {
	if raw == "" {
		return nil
	}
	var files []diffFile
	var current *diffFile
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				current.diff = strings.TrimRight(current.diff, "\n")
				files = append(files, *current)
			}
			// Extract filename from "diff --git a/foo b/foo"
			parts := strings.Fields(line)
			name := ""
			if len(parts) >= 4 {
				name = strings.TrimPrefix(parts[3], "b/")
			}
			current = &diffFile{name: name}
		}
		if current != nil {
			current.diff += line + "\n"
		}
	}
	if current != nil {
		current.diff = strings.TrimRight(current.diff, "\n")
		files = append(files, *current)
	}
	return files
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

// renderDiffPanel renders a two-pane diff viewer: file list on the left, diff on the right.
func (m TaskModel) renderDiffPanel(w, bodyH, cardW int) string {
	fileListW := 30
	if fileListW > w/3 {
		fileListW = w / 3
	}
	diffPaneW := w - fileListW - 3 // divider(1) + margins(2)
	if diffPaneW < 20 {
		diffPaneW = 20
	}
	innerH := bodyH - 4 // border top+bottom(2) + top margin(2)

	// ── file list ──
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4ADE80"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	fileHeaderStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	var fileListLines []string
	for i, f := range m.diffFiles {
		name := f.name
		runes := []rune(name)
		maxName := fileListW - 4
		if len(runes) > maxName {
			name = "…" + string(runes[len(runes)-maxName+1:])
		}
		if i == m.diffCursor {
			fileListLines = append(fileListLines, lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorPrimary).
				Width(fileListW-2).
				Render(name))
		} else {
			fileListLines = append(fileListLines, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF")).
				Width(fileListW-2).
				Render(name))
		}
	}
	// Pad to innerH
	for len(fileListLines) < innerH {
		fileListLines = append(fileListLines, "")
	}
	if len(fileListLines) > innerH {
		// scroll file list to keep cursor visible
		start := m.diffCursor - innerH + 1
		if start < 0 {
			start = 0
		}
		if m.diffCursor-start < innerH {
			fileListLines = fileListLines[start:]
		}
		if len(fileListLines) > innerH {
			fileListLines = fileListLines[:innerH]
		}
	}
	fileListBorder := colorPrimary
	if m.diffFocusRight {
		fileListBorder = colorMuted
	}
	fileListPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(fileListBorder).
		Padding(0, 1).
		Width(fileListW).
		Height(bodyH - 4).
		Render(lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("files") + "\n\n" + strings.Join(fileListLines, "\n"))

	// ── diff pane ──
	var diffPaneContent string
	textW := diffPaneW - 4
	if len(m.diffFiles) > 0 && m.diffCursor < len(m.diffFiles) {
		rawLines := strings.Split(m.diffFiles[m.diffCursor].diff, "\n")
		// apply scroll
		scroll := m.diffScroll
		maxDiffScroll := len(rawLines) - 1
		if scroll > maxDiffScroll {
			scroll = maxDiffScroll
		}
		if scroll < 0 {
			scroll = 0
		}
		visLines := innerH - 2 // header(1) + blank(1)
		if visLines < 1 {
			visLines = 1
		}
		visible := rawLines[scroll:]
		if len(visible) > visLines {
			visible = visible[:visLines]
		}
		var rendered []string
		for _, dl := range visible {
			runes := []rune(dl)
			if len(runes) > textW {
				dl = string(runes[:textW-1]) + "…"
			}
			switch {
			case strings.HasPrefix(dl, "+++") || strings.HasPrefix(dl, "---"):
				rendered = append(rendered, fileHeaderStyle.Render(dl))
			case strings.HasPrefix(dl, "diff ") || strings.HasPrefix(dl, "index "):
				rendered = append(rendered, fileHeaderStyle.Render(dl))
			case strings.HasPrefix(dl, "@@"):
				rendered = append(rendered, hunkStyle.Render(dl))
			case strings.HasPrefix(dl, "+"):
				rendered = append(rendered, addStyle.Render(dl))
			case strings.HasPrefix(dl, "-"):
				rendered = append(rendered, delStyle.Render(dl))
			default:
				rendered = append(rendered, StyleStatusMuted.Render(dl))
			}
		}
		diffPaneContent = strings.Join(rendered, "\n")
	} else {
		diffPaneContent = StyleStatusMuted.Render("no diff")
	}

	fileName := ""
	if len(m.diffFiles) > 0 && m.diffCursor < len(m.diffFiles) {
		fileName = m.diffFiles[m.diffCursor].name
	}
	diffPaneBorder := colorMuted
	if m.diffFocusRight {
		diffPaneBorder = colorPrimary
	}
	diffPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(diffPaneBorder).
		Padding(0, 1).
		Width(diffPaneW).
		Height(bodyH - 4).
		Render(lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render(fileName) + "\n\n" + diffPaneContent)

	// Merge panels line-by-line
	fileLines := strings.Split(fileListPanel, "\n")
	diffLines := strings.Split(diffPanel, "\n")
	emptyFile := strings.Repeat(" ", lipgloss.Width(fileListPanel))
	emptyDiff := strings.Repeat(" ", lipgloss.Width(diffPanel))
	var rows []string
	for i := 0; i < bodyH; i++ {
		if i < 2 {
			rows = append(rows, "")
			continue
		}
		ci := i - 2
		var fl, dl string
		if ci < len(fileLines) {
			fl = fileLines[ci]
		} else {
			fl = emptyFile
		}
		if ci < len(diffLines) {
			dl = diffLines[ci]
		} else {
			dl = emptyDiff
		}
		rows = append(rows, "  "+fl+" "+dl)
	}
	return strings.Join(rows, "\n")
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

	var diffStatLine string
	if m.diffStat != "" {
		diffStatLine = lipgloss.NewStyle().Foreground(colorSuccess).Render(m.diffStat)
		if len(m.diffFiles) > 0 {
			diffStatLine += "  " + StyleStatusMuted.Render("d to browse")
		}
	} else {
		diffStatLine = StyleStatusMuted.Render("no changes")
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
		diffStatLine,
		"",
		divider,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("description"),
		"",
		descContent,
	}

	headerH := lipgloss.Height(header)
	footerH := 3
	bodyH := h - headerH - footerH
	// card border(2) + padding top+bottom(2) + margin top(2) = 6
	visibleLines := bodyH - 6
	if visibleLines < 3 {
		visibleLines = 3
	}

	// Expand all card rows into individual terminal lines for smooth line-by-line scrolling.
	allLines := strings.Split(lipgloss.JoinVertical(lipgloss.Left, cardRows...), "\n")

	// Allow scrolling until the last content line is visible at the top of the window.
	maxScroll := len(allLines) - 1
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.cardScroll > maxScroll {
		m.cardScroll = maxScroll
	}
	visibleSlice := allLines[m.cardScroll:]
	if len(visibleSlice) > visibleLines {
		visibleSlice = visibleSlice[:visibleLines]
	}

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Width(cardW).
		Render(strings.Join(visibleSlice, "\n"))

	var bodyContent string
	if m.diffOpen {
		bodyContent = m.renderDiffPanel(w, bodyH, cardW)
	} else if m.tmuxOutput != "" {
		// Pad card to fixed height only in side-by-side mode to keep output panel stable.
		for len(visibleSlice) < visibleLines {
			visibleSlice = append(visibleSlice, "")
		}
		card = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2).
			Width(cardW).
			Render(strings.Join(visibleSlice, "\n"))
		// Two independent panels merged line-by-line so scrolling one never affects the other.
		cardLeft := 2  // left margin
		cardRight := cardLeft + lipgloss.Width(card)
		outputLeft := cardRight + 2

		outputW := w - outputLeft - 1
		if outputW < 20 {
			outputW = 20
		}

		// Build output panel lines — cap to inner height: bodyH-4 (panel height) minus header(1)+blank(1) = bodyH-6
		maxOutputLines := bodyH - 6
		if maxOutputLines < 1 {
			maxOutputLines = 1
		}
		userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C4B5FD")).Bold(true)
		agentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
		textW := outputW - 4 // border(2) + padding(2)
		rawOutputLines := strings.Split(m.tmuxOutput, "\n")
		if len(rawOutputLines) > maxOutputLines {
			rawOutputLines = rawOutputLines[len(rawOutputLines)-maxOutputLines:]
		}
		var outputRendered []string
		for _, ol := range rawOutputLines {
			runes := []rune(ol)
			if len(runes) > textW {
				ol = string(runes[:textW-1]) + "…"
			}
			trimmed := strings.TrimSpace(ol)
			if strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "❯") {
				outputRendered = append(outputRendered, userStyle.Render(ol))
			} else {
				outputRendered = append(outputRendered, agentStyle.Render(ol))
			}
		}
		panelHeader := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("agent output")
		outputPanel := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1).
			Width(outputW).
			Height(bodyH - 4). // bodyH - 2 (top margin) - 2 (border top+bottom)
			Render(panelHeader + "\n\n" + strings.Join(outputRendered, "\n"))

		// Split both panels into lines.
		cardLines := strings.Split(card, "\n")
		outputPanelLines := strings.Split(outputPanel, "\n")

		// Merge line-by-line into a canvas of bodyH lines × w cols.
		emptyCard := strings.Repeat(" ", lipgloss.Width(card))
		emptyOutput := strings.Repeat(" ", lipgloss.Width(outputPanel))
		var rows []string
		for i := 0; i < bodyH; i++ {
			// 2 lines of top margin before card/output
			if i < 2 {
				rows = append(rows, "")
				continue
			}
			ci := i - 2
			var cl, ol string
			if ci < len(cardLines) {
				cl = cardLines[ci]
			} else {
				cl = emptyCard
			}
			if ci < len(outputPanelLines) {
				ol = outputPanelLines[ci]
			} else {
				ol = emptyOutput
			}
			gap := strings.Repeat(" ", cardLeft) + cl + strings.Repeat(" ", 2) + ol
			rows = append(rows, gap)
		}
		bodyContent = strings.Join(rows, "\n")
	} else {
		bodyContent = lipgloss.Place(w, bodyH, lipgloss.Center, lipgloss.Top,
			lipgloss.NewStyle().MarginTop(2).Render(card),
		)
	}

	// ── footer ────────────────────────────────────────────────────────────────
	var footerParts []string

	if m.diffOpen {
		if m.diffFocusRight {
			footerParts = append(footerParts, "↑/↓ scroll")
			footerParts = append(footerParts, "← files")
			footerParts = append(footerParts, "e vim")
			footerParts = append(footerParts, "esc back")
		} else {
			footerParts = append(footerParts, "↑/↓ files")
			footerParts = append(footerParts, "→ diff")
			footerParts = append(footerParts, "e vim")
			footerParts = append(footerParts, "d/esc close")
		}
	} else {
		footerParts = append(footerParts, "r launch/resume")

		sessionName := agent.TmuxSessionName(&m.task)
		if agent.TmuxSessionExists(sessionName) {
			sessionHint := StyleStatusMuted.Render("inside session: ") +
				lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("ctrl+q") +
				StyleStatusMuted.Render(" detach")
			footerParts = append(footerParts, sessionHint)
		}

		if prev := m.task.PrevPhase(); prev != "" {
			footerParts = append(footerParts, "h → "+phaseLabels[prev])
		}
		if next := m.task.NextPhase(); next != "" {
			footerParts = append(footerParts, "l → "+phaseLabels[next])
		}
		if len(m.diffFiles) > 0 {
			footerParts = append(footerParts, "d diff")
		}
		footerParts = append(footerParts, "↑/↓ scroll")
		footerParts = append(footerParts, "esc back")
	}

	footerHelp := strings.Join(footerParts, "   ")
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

	return lipgloss.JoinVertical(lipgloss.Left, header, bodyContent, footer)
}

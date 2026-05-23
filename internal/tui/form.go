package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── types ─────────────────────────────────────────────────────────────────────

type FormResult struct {
	Title        string
	Description  string
	Branch       string
	Agent        string
	Plugin       string
	SkipResearch bool
	Cancelled    bool
}

type FormSubmitMsg struct{ Result FormResult }
type FormCancelMsg struct{}

type formStep int

const (
	stepTitle        formStep = iota // textarea(h=1) — enter advances
	stepBranch                       // textarea(h=1) — enter advances
	stepAgent                        // selector      — enter advances
	stepSkipResearch                 // toggle        — enter advances
	stepDesc                         // textarea      — enter = newline, ctrl+d advances
	stepCount
)

var agentOptions = []string{"claude-code", "codex"}


var stepQuestion = []string{
	"What's the task title?",
	"Branch name for this task?",
	"Which agent should run it?",
	"Skip the research phase?",
	"Describe the task  (enter → newline  ·  ctrl+d → create task)",
}

var stepLabel = []string{"Title", "Branch", "Agent", "Research", "Description"}

const formChrome = 10 // fixed lines of chrome above/below the input box
const inputBoxMinH = 8

// ── model ─────────────────────────────────────────────────────────────────────

type FormModel struct {
	step      formStep
	titleIn   textarea.Model
	branchIn  textarea.Model
	descArea  textarea.Model
	agentIdx  int
	pluginIdx int
	skipRes   bool
	width     int
	height    int
}

// formBoxInnerW is the content width inside the outer box — capped at 90, matches View().
func formBoxInnerW(termW int) int {
	w := termW - 8 // border(2) + padding(6)
	if w > 90 {
		w = 90
	}
	if w < 40 {
		w = 40
	}
	return w
}

func inputBoxH(termH int) int {
	h := termH - 6 - formChrome
	if h < inputBoxMinH {
		h = inputBoxMinH
	}
	return h
}

// taW returns the textarea content width: biW minus input-box border(2)+padding(4).
func taW(termW int) int {
	w := formBoxInnerW(termW) - 6
	if w < 10 {
		w = 10
	}
	return w
}

func newSingleLineArea(placeholder string, charLimit int, w, h int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.CharLimit = charLimit
	ta.SetWidth(w)
	ta.SetHeight(h)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.KeyMap.InsertNewline.SetEnabled(false) // enter must not insert newline
	return ta
}

func NewFormModel(w, h int) FormModel {
	tw := taW(w)
	ibH := inputBoxH(h)

	ti := newSingleLineArea("e.g. Fix the SSO login bug", 300, tw, ibH)
	ti.Focus()

	bi := newSingleLineArea("e.g. fix/sso-login-bug", 100, tw, ibH)

	da := textarea.New()
	da.Placeholder = "Context, constraints, acceptance criteria…"
	da.CharLimit = 4000
	da.SetWidth(tw)
	da.SetHeight(ibH - 2)
	da.ShowLineNumbers = false
	da.FocusedStyle.CursorLine = lipgloss.NewStyle()
	da.FocusedStyle.Base = lipgloss.NewStyle()
	da.BlurredStyle.Base = lipgloss.NewStyle()

	return FormModel{titleIn: ti, branchIn: bi, descArea: da, skipRes: true, width: w, height: h}
}

func (m FormModel) Init() tea.Cmd { return textarea.Blink }

func (m FormModel) submit() (tea.Model, tea.Cmd) {
	r := FormResult{
		Title:        strings.TrimSpace(strings.ReplaceAll(m.titleIn.Value(), "\n", "")),
		Branch:       strings.TrimSpace(strings.ReplaceAll(m.branchIn.Value(), "\n", "")),
		Description:  strings.TrimSpace(m.descArea.Value()),
		Agent:        agentOptions[m.agentIdx],
		Plugin:       "none",
		SkipResearch: m.skipRes,
	}
	return m, func() tea.Msg { return FormSubmitMsg{Result: r} }
}

// ── update ────────────────────────────────────────────────────────────────────

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		tw := taW(m.width)
		ibH := inputBoxH(m.height)
		m.titleIn.SetWidth(tw)
		m.titleIn.SetHeight(ibH)
		m.branchIn.SetWidth(tw)
		m.branchIn.SetHeight(ibH)
		m.descArea.SetWidth(tw)
		m.descArea.SetHeight(ibH - 2)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.step == stepTitle {
				return m, func() tea.Msg { return FormCancelMsg{} }
			}
			m.step--
			var focusCmd tea.Cmd
			m, focusCmd = m.syncFocus()
			return m, focusCmd

		case "enter":
			// desc step: enter = newline, fall through to textarea update
			if m.step == stepDesc {
				break
			}
			if m.step == stepTitle && strings.TrimSpace(m.titleIn.Value()) == "" {
				return m, nil
			}
			if m.step == stepBranch && strings.TrimSpace(m.branchIn.Value()) == "" {
				return m, nil
			}
			return m.advance()

		case "ctrl+d":
			// advance / submit from any step (primary key for desc step)
			if m.step == stepTitle && strings.TrimSpace(m.titleIn.Value()) == "" {
				return m, nil
			}
			if m.step == stepBranch && strings.TrimSpace(m.branchIn.Value()) == "" {
				return m, nil
			}
			return m.advance()

		case "left", "up":
			switch m.step {
			case stepAgent:
				m.agentIdx = (m.agentIdx - 1 + len(agentOptions)) % len(agentOptions)
				return m, nil
			case stepSkipResearch:
				m.skipRes = !m.skipRes
				return m, nil
			}

		case "right", "down":
			switch m.step {
			case stepAgent:
				m.agentIdx = (m.agentIdx + 1) % len(agentOptions)
				return m, nil
			case stepSkipResearch:
				m.skipRes = !m.skipRes
				return m, nil
			}

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
		m.titleIn, cmd = m.titleIn.Update(msg)
	case stepBranch:
		m.branchIn, cmd = m.branchIn.Update(msg)
	case stepDesc:
		m.descArea, cmd = m.descArea.Update(msg)
	}
	return m, cmd
}


func (m FormModel) advance() (tea.Model, tea.Cmd) {
	if m.step == formStep(stepCount-1) {
		return m.submit()
	}
	m.step++
	var focusCmd tea.Cmd
	m, focusCmd = m.syncFocus()
	return m, focusCmd
}

func (m FormModel) syncFocus() (FormModel, tea.Cmd) {
	m.titleIn.Blur()
	m.branchIn.Blur()
	m.descArea.Blur()
	var cmd tea.Cmd
	switch m.step {
	case stepTitle:
		cmd = m.titleIn.Focus()
	case stepBranch:
		cmd = m.branchIn.Focus()
	case stepDesc:
		cmd = m.descArea.Focus()
	}
	return m, cmd
}


// ── view ──────────────────────────────────────────────────────────────────────

func (m FormModel) View() string {
	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	biW := formBoxInnerW(w)
	ibH := inputBoxH(h)

	// ── Row 1: "New Task" title + step counter + breadcrumb ───────────────────
	titlePart := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("New Task")
	counterPart := lipgloss.NewStyle().Foreground(colorMuted).
		Render(fmt.Sprintf("  step %d of %d  ", int(m.step)+1, int(stepCount)))
	crumbPart := renderCrumbs(m.step)

	gap := biW - visLen(titlePart) - visLen(counterPart) - visLen(crumbPart) - 2
	if gap < 1 {
		gap = 1
	}
	divLine := lipgloss.NewStyle().Foreground(colorMuted).Render(strings.Repeat("─", gap))
	headerRow := titlePart + counterPart + divLine + "  " + crumbPart

	// ── Row 2: full-width separator ───────────────────────────────────────────
	sep := lipgloss.NewStyle().Foreground(colorMuted).Render(strings.Repeat("─", biW))

	// ── Row 3: progress dots ──────────────────────────────────────────────────
	progress := renderProgress(m.step, biW)

	// ── Row 4: question ───────────────────────────────────────────────────────
	question := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Width(biW).
		Render(stepQuestion[m.step])

	// ── Row 5: input box (fixed height) ───────────────────────────────────────
	inputContent := m.buildInputContent(biW, ibH)
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(biW - 2).
		Height(ibH).
		Render(inputContent)

	// ── Row 6: button bar ─────────────────────────────────────────────────────
	buttons := renderButtons(m.step, biW)

	content := lipgloss.JoinVertical(lipgloss.Left,
		headerRow,
		sep,
		"",
		progress,
		"",
		question,
		"",
		inputBox,
		"",
		buttons,
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 3).
		Width(biW + 8) // +2 border +6 padding

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box.Render(content))
}

func (m FormModel) buildInputContent(biW, ibH int) string {
	// inside the input box: box border(2) + padding(2 each side=4) = 6
	contentW := biW - 6

	var raw string
	switch m.step {
	case stepTitle:
		raw = m.titleIn.View()
	case stepBranch:
		raw = m.branchIn.View()
	case stepAgent:
		raw = renderSelector(agentOptions, m.agentIdx, contentW)
	case stepSkipResearch:
		raw = "\n" + renderYesNo(m.skipRes, contentW)
	case stepDesc:
		raw = m.descArea.View()
	}

	lines := strings.Split(raw, "\n")
	for len(lines) < ibH {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderButtons(current formStep, innerW int) string {
	dimStyle := lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 1)
	activeStyle := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		Padding(0, 1)
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(0, 1)

	// Back / Cancel
	var backLabel string
	if current == stepTitle {
		backLabel = "cancel"
	} else {
		backLabel = "← back"
	}
	backBtn := dimStyle.Render(backLabel) + " " + keyStyle.Render("esc")

	// Next / Create
	var nextLabel, nextKey string
	if current == formStep(stepCount-1) {
		nextLabel = "✓ create task"
		nextKey = "ctrl+d"
	} else if current == stepDesc {
		nextLabel = "next →"
		nextKey = "ctrl+d"
	} else {
		nextLabel = "next →"
		nextKey = "enter"
	}
	nextBtn := activeStyle.Render(nextLabel) + " " + keyStyle.Render(nextKey)

	// separator
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#2D2D2D")).
		Render(strings.Repeat("─", innerW))

	pad := innerW - visLen(backBtn) - visLen(nextBtn) - 2
	if pad < 1 {
		pad = 1
	}
	btnRow := backBtn + strings.Repeat(" ", pad) + nextBtn
	return sep + "\n" + btnRow
}

// ── helpers ───────────────────────────────────────────────────────────────────

func renderProgress(current formStep, width int) string {
	var segs []string
	for i := formStep(0); i < stepCount; i++ {
		var dot string
		switch {
		case i < current:
			dot = lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
		case i == current:
			dot = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("◉")
		default:
			dot = lipgloss.NewStyle().Foreground(lipgloss.Color("#3D3D3D")).Render("○")
		}
		segs = append(segs, dot)
	}
	bar := strings.Join(segs, "  ")
	pct := lipgloss.NewStyle().Foreground(colorMuted).
		Render(fmt.Sprintf("step %d of %d", int(current)+1, int(stepCount)))
	pad := width - visLen(bar) - visLen(pct)
	if pad < 1 {
		pad = 1
	}
	return bar + strings.Repeat(" ", pad) + pct
}

func renderCrumbs(current formStep) string {
	sep := lipgloss.NewStyle().Foreground(colorMuted).Render(" › ")
	var parts []string
	for i, lbl := range stepLabel {
		switch {
		case formStep(i) < current:
			parts = append(parts, lipgloss.NewStyle().Foreground(colorSuccess).Render(lbl))
		case formStep(i) == current:
			parts = append(parts, lipgloss.NewStyle().Bold(true).
				Foreground(lipgloss.Color("#F9FAFB")).Render(lbl))
		default:
			parts = append(parts, lipgloss.NewStyle().Foreground(colorMuted).Render(lbl))
		}
	}
	return strings.Join(parts, sep)
}

func renderSelector(options []string, selected int, width int) string {
	var rows []string
	for i, opt := range options {
		if i == selected {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(colorPrimary).Bold(true).Width(width).
				Render("  ▶  "+opt))
		} else {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(colorMuted).Width(width).
				Render("     "+opt))
		}
	}
	return strings.Join(rows, "\n")
}

func renderYesNo(skipRes bool, _ int) string {
	yes := lipgloss.NewStyle().Padding(0, 3)
	no := lipgloss.NewStyle().Padding(0, 3)
	if skipRes {
		yes = yes.Background(colorPrimary).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		no = no.Foreground(colorMuted)
	} else {
		no = no.Background(colorPrimary).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		yes = yes.Foreground(colorMuted)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		yes.Render("Yes, skip"),
		lipgloss.NewStyle().Foreground(colorMuted).Render("    "),
		no.Render("No, include"),
	)
}

func visLen(s string) int { return len(stripANSI(s)) }

func stripANSI(s string) string {
	out := make([]byte, 0, len(s))
	esc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			esc = true
			continue
		}
		if esc {
			if s[i] == 'm' {
				esc = false
			}
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}

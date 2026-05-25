package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FormResult struct {
	Title        string
	Description  string
	Branch       string
	Agent        string
	Plugin       string
	SkipResearch bool
	AutoRun      bool
	PRBaseBranch string
	Cancelled    bool
}

type FormSubmitMsg struct{ Result FormResult }
type FormCancelMsg struct{}

type formStep int

const (
	stepTitle formStep = iota
	stepBranch
	stepAgent
	stepConfig
	stepDesc
	stepCount
)

type configField int

const (
	configSkipResearch configField = iota
	configAutoRun
	configPRBaseBranch
	configFieldCount
)

var agentOptions = []string{"claude-code", "codex"}

var stepQuestion = []string{
	"What's the task title?",
	"Branch name for this task?",
	"Which agent should run it?",
	"Configuration",
	"Describe the task  (enter → newline  ·  ctrl+d → create task)",
}

var stepLabel = []string{"Title", "Branch", "Agent", "Configuration", "Description"}

const formChrome = 10
const inputBoxMinH = 8

type FormModel struct {
	step      formStep
	titleIn   textarea.Model
	branchIn  textarea.Model
	prBaseIn  textarea.Model
	descArea  textarea.Model
	agentIdx  int
	pluginIdx int
	configIdx configField
	skipRes   bool
	autoRun   bool
	width     int
	height    int
}

func formBoxInnerW(termW int) int {
	w := termW - 8
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
	ta.KeyMap.InsertNewline.SetEnabled(false)
	return ta
}

func NewFormModel(w, h int) FormModel {
	tw := taW(w)
	ibH := inputBoxH(h)

	ti := newSingleLineArea("e.g. Fix the SSO login bug", 300, tw, ibH)
	ti.Focus()

	bi := newSingleLineArea("e.g. fix/sso-login-bug", 100, tw, ibH)
	pi := newSingleLineArea("master", 100, tw-4, 1)
	pi.SetValue("master")

	da := textarea.New()
	da.Placeholder = "Context, constraints, acceptance criteria…"
	da.CharLimit = 4000
	da.SetWidth(tw)
	da.SetHeight(ibH - 2)
	da.ShowLineNumbers = false
	da.FocusedStyle.CursorLine = lipgloss.NewStyle()
	da.FocusedStyle.Base = lipgloss.NewStyle()
	da.BlurredStyle.Base = lipgloss.NewStyle()

	return FormModel{
		titleIn:  ti,
		branchIn: bi,
		prBaseIn: pi,
		descArea: da,
		skipRes:  true,
		width:    w,
		height:   h,
	}
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
		AutoRun:      m.autoRun,
		PRBaseBranch: strings.TrimSpace(strings.ReplaceAll(m.prBaseIn.Value(), "\n", "")),
	}
	return m, func() tea.Msg { return FormSubmitMsg{Result: r} }
}

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
		m.prBaseIn.SetWidth(tw - 4)
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
			return m.syncFocus()

		case "enter":
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
			if m.step == stepTitle && strings.TrimSpace(m.titleIn.Value()) == "" {
				return m, nil
			}
			if m.step == stepBranch && strings.TrimSpace(m.branchIn.Value()) == "" {
				return m, nil
			}
			return m.advance()

		case "up":
			switch m.step {
			case stepAgent:
				m.agentIdx = (m.agentIdx - 1 + len(agentOptions)) % len(agentOptions)
				return m, nil
			case stepConfig:
				if m.configIdx > 0 {
					m.configIdx--
					return m.syncConfigFocus()
				}
				return m, nil
			}

		case "down":
			switch m.step {
			case stepAgent:
				m.agentIdx = (m.agentIdx + 1) % len(agentOptions)
				return m, nil
			case stepConfig:
				if m.configIdx < configFieldCount-1 {
					m.configIdx++
					return m.syncConfigFocus()
				}
				return m, nil
			}

		case "left", "right", " ":
			if m.step == stepConfig {
				switch m.configIdx {
				case configSkipResearch:
					m.skipRes = !m.skipRes
				case configAutoRun:
					m.autoRun = !m.autoRun
				}
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
	case stepConfig:
		if m.configIdx == configPRBaseBranch {
			m.prBaseIn, cmd = m.prBaseIn.Update(msg)
		}
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
	return m.syncFocus()
}

func (m FormModel) syncFocus() (FormModel, tea.Cmd) {
	m.titleIn.Blur()
	m.branchIn.Blur()
	m.prBaseIn.Blur()
	m.descArea.Blur()
	switch m.step {
	case stepTitle:
		return m, m.titleIn.Focus()
	case stepBranch:
		return m, m.branchIn.Focus()
	case stepConfig:
		return m.syncConfigFocus()
	case stepDesc:
		return m, m.descArea.Focus()
	default:
		return m, nil
	}
}

func (m FormModel) syncConfigFocus() (FormModel, tea.Cmd) {
	m.prBaseIn.Blur()
	if m.step == stepConfig && m.configIdx == configPRBaseBranch {
		return m, m.prBaseIn.Focus()
	}
	return m, nil
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

	biW := formBoxInnerW(w)
	ibH := inputBoxH(h)

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

	sep := lipgloss.NewStyle().Foreground(colorMuted).Render(strings.Repeat("─", biW))
	progress := renderProgress(m.step, biW)
	question := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Width(biW).
		Render(stepQuestion[m.step])

	inputContent := m.buildInputContent(biW, ibH)
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(biW - 2).
		Height(ibH).
		Render(inputContent)

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
		Width(biW + 8)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box.Render(content))
}

func (m FormModel) buildInputContent(biW, ibH int) string {
	contentW := biW - 6

	var raw string
	switch m.step {
	case stepTitle:
		raw = m.titleIn.View()
	case stepBranch:
		raw = m.branchIn.View()
	case stepAgent:
		raw = renderSelector(agentOptions, m.agentIdx, contentW)
	case stepConfig:
		raw = renderConfigStep(m, contentW, ibH)
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

	backLabel := "← back"
	if current == stepTitle {
		backLabel = "cancel"
	}
	backBtn := dimStyle.Render(backLabel) + " " + keyStyle.Render("esc")

	nextLabel := "next →"
	nextKey := "enter"
	if current == formStep(stepCount-1) {
		nextLabel = "✓ create task"
		nextKey = "ctrl+d"
	} else if current == stepDesc {
		nextKey = "ctrl+d"
	}
	nextBtn := activeStyle.Render(nextLabel) + " " + keyStyle.Render(nextKey)

	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#2D2D2D")).
		Render(strings.Repeat("─", innerW))

	pad := innerW - visLen(backBtn) - visLen(nextBtn) - 2
	if pad < 1 {
		pad = 1
	}
	btnRow := backBtn + strings.Repeat(" ", pad) + nextBtn
	return sep + "\n" + btnRow
}

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

func renderConfigStep(m FormModel, width, height int) string {
	rows := []string{
		renderConfigToggleRow("Skip research", "Yes, skip", "No, include", m.skipRes, m.configIdx == configSkipResearch),
		"",
		"",
		renderConfigToggleRow("Run as soon as ticket is made", "Yes, run", "No, wait", m.autoRun, m.configIdx == configAutoRun),
		"",
		"",
		renderConfigInputRow("Raise PR against branch", m.prBaseIn.View(), m.configIdx == configPRBaseBranch, width),
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	return strings.Join(rows[:height], "\n")
}

func renderConfigToggleRow(label, yesLabel, noLabel string, enabled, selected bool) string {
	title := lipgloss.NewStyle().Foreground(colorMuted).Render(label)
	if selected {
		title = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("▶ " + label)
	}
	options := renderRadioOptions(enabled, yesLabel, noLabel, selected)
	return title + "\n\n" + options
}

func renderConfigInputRow(label, value string, selected bool, width int) string {
	title := lipgloss.NewStyle().Foreground(colorMuted).Render(label)
	if selected {
		title = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("▶ " + label)
	}
	line := strings.TrimSpace(strings.ReplaceAll(value, "\n", ""))
	if line == "" {
		line = lipgloss.NewStyle().Foreground(colorMuted).Render("master")
	}
	border := colorMuted
	if selected {
		border = colorPrimary
	}
	input := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(width - 2).
		Render(line)
	return title + "\n" + input
}

func renderRadioOptions(enabled bool, yesLabel, noLabel string, selected bool) string {
	activeStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(colorMuted)

	yesMarker := "○"
	noMarker := "○"
	if enabled {
		yesMarker = "◉"
	} else {
		noMarker = "◉"
	}

	yesText := yesMarker + " " + yesLabel
	noText := noMarker + " " + noLabel
	if selected {
		if enabled {
			yesText = activeStyle.Render(yesText)
			noText = inactiveStyle.Render(noText)
		} else {
			yesText = inactiveStyle.Render(yesText)
			noText = activeStyle.Render(noText)
		}
	} else {
		yesText = inactiveStyle.Render(yesText)
		noText = inactiveStyle.Render(noText)
	}

	return "  " + yesText + "\n" + "  " + noText
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

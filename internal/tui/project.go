package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/agrajgarg/orka/internal/state"
)

type ProjectSelectedMsg struct{ ProjectID string }

type addProjectForm struct {
	picker  *filePicker
	confirm string // non-empty = waiting for confirm on this path
}

type ProjectModel struct {
	st            *state.State
	statePath     string
	idx           int
	addForm       *addProjectForm
	removeConfirm bool
	width         int
	height        int
}

func NewProjectModel(st *state.State, statePath string, w, h int) ProjectModel {
	return ProjectModel{st: st, statePath: statePath, width: w, height: h}
}

func (m ProjectModel) Init() tea.Cmd { return nil }

func (m ProjectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = sz.Width, sz.Height
	}

	if m.addForm != nil {
		return m.updateAddForm(msg)
	}

	if k, ok := msg.(tea.KeyMsg); ok {
		if m.removeConfirm {
			switch k.String() {
			case "y", "Y":
				if m.idx < len(m.st.Projects) {
					m.st.RemoveProject(m.st.Projects[m.idx].ID)
					_ = m.st.Save(m.statePath)
					if m.idx >= len(m.st.Projects) && m.idx > 0 {
						m.idx--
					}
				}
				m.removeConfirm = false
			case "n", "N", "esc":
				m.removeConfirm = false
			}
			return m, nil
		}

		switch k.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.idx > 0 {
				m.idx--
			}
		case "down", "j":
			if m.idx < len(m.st.Projects) {
				m.idx++
			}
		case "d", "backspace":
			if m.idx < len(m.st.Projects) {
				m.removeConfirm = true
			}
		case "enter":
			if m.idx == len(m.st.Projects) {
				home, _ := os.UserHomeDir()
				fp := newFilePicker(home, m.width, m.height)
				m.addForm = &addProjectForm{picker: &fp}
				return m, nil
			}
			return m, func() tea.Msg {
				return ProjectSelectedMsg{ProjectID: m.st.Projects[m.idx].ID}
			}
		}
	}
	return m, nil
}

func (m ProjectModel) updateAddForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	f := m.addForm
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			if f.confirm != "" {
				f.confirm = ""
				return m, nil
			}
			m.addForm = nil
			return m, nil
		case "y", "Y":
			if f.confirm != "" {
				name := filepath.Base(f.confirm)
				m.st.AddProject(name, f.confirm)
				_ = m.st.Save(m.statePath)
				m.idx = len(m.st.Projects) - 1
				m.addForm = nil
				return m, nil
			}
		case "n", "N":
			if f.confirm != "" {
				f.confirm = ""
				return m, nil
			}
		}
	}

	if f.confirm != "" {
		return m, nil
	}

	if picked, ok := msg.(FilePickedMsg); ok {
		f.confirm = picked.Path
		return m, nil
	}

	var cmd tea.Cmd
	*f.picker, cmd = f.picker.Update(msg)
	return m, cmd
}

func (m ProjectModel) View() string {
	w := m.width
	if w == 0 {
		w = 120
	}
	h := m.height
	if h == 0 {
		h = 30
	}

	header := lipgloss.NewStyle().
		Width(w).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("orka") +
			StyleStatusMuted.Render("  agent kanban"))

	footer := lipgloss.NewStyle().
		Width(w).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		Render(func() string {
			if m.removeConfirm && m.idx < len(m.st.Projects) {
				return StyleConfirmPrompt.Render(fmt.Sprintf("remove \"%s\" and all its tasks? (y/n)", m.st.Projects[m.idx].Name))
			}
			return StyleHelp.Render("j/k navigate   enter open   d remove   esc back   q quit")
		}())

	cardW := 56
	if cardW > w-4 {
		cardW = w - 4
	}
	bodyH := h - lipgloss.Height(header) - lipgloss.Height(footer)

	if m.addForm != nil {
		f := m.addForm
		formW := 56
		if formW > w-4 {
			formW = w - 4
		}

		var inner string
		if f.confirm != "" {
			inner = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("add project"),
				"",
				lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB")).Render(f.confirm),
				"",
				StyleConfirmPrompt.Render("add this project? (y/n)"),
			)
		} else {
			f.picker.width = m.width
			f.picker.height = m.height
			inner = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render("select directory"),
				"",
				f.picker.View(formW),
			)
		}
		form := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 3).
			Width(formW).
			Render(inner)
		body := lipgloss.Place(w, bodyH, lipgloss.Center, lipgloss.Center, form)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	}

	// ── project list ──────────────────────────────────────────────────────────
	title := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).
		Width(cardW).Align(lipgloss.Center).Render("select a project")

	innerW := cardW - 4 // padding(2 each side)
	var rows []string
	rows = append(rows, title, "")

	for i, p := range m.st.Projects {
		selected := i == m.idx
		taskCount := 0
		for _, t := range m.st.Tasks {
			if t.ProjectID == p.ID {
				taskCount++
			}
		}

		nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F9FAFB"))
		pathStyle := StyleStatusMuted
		countStyle := StyleStatusMuted
		borderColor := colorMuted
		if selected {
			nameStyle = nameStyle.Foreground(colorPrimary)
			borderColor = colorPrimary
		}

		taskLabel := fmt.Sprintf("%d task", taskCount)
		if taskCount != 1 {
			taskLabel += "s"
		}

		// right-align task count on same line as name
		nameText := nameStyle.Render(p.Name)
		countText := countStyle.Render(taskLabel)
		gap := innerW - lipgloss.Width(nameText) - lipgloss.Width(countText)
		if gap < 1 {
			gap = 1
		}
		nameLine := nameText + strings.Repeat(" ", gap) + countText

		inner := lipgloss.JoinVertical(lipgloss.Left,
			nameLine,
			pathStyle.Render(p.Path),
		)

		rows = append(rows, lipgloss.NewStyle().
			Width(cardW).Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Render(inner),
		)
	}

	// add new row
	addSelected := m.idx == len(m.st.Projects)
	addBorder := colorMuted
	addText := StyleStatusMuted.Render("+ add new project")
	if addSelected {
		addBorder = colorPrimary
		addText = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("+ add new project")
	}
	rows = append(rows, lipgloss.NewStyle().
		Width(cardW).Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(addBorder).
		Render(addText),
	)

	list := lipgloss.JoinVertical(lipgloss.Left, rows...)
	body := lipgloss.Place(w, bodyH, lipgloss.Center, lipgloss.Center, list)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

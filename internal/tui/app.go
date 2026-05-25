package tui

import (
	"github.com/agrajgarg/orka/internal/config"
	"github.com/agrajgarg/orka/internal/state"
	tea "github.com/charmbracelet/bubbletea"
)

type AppModel struct {
	project   *ProjectModel
	board     *BoardModel
	task      *TaskModel
	cfg       *config.Config
	st        *state.State
	statePath string
	width     int
	height    int
}

func NewAppModel(st *state.State, statePath string, cfg *config.Config, w, h int) AppModel {
	pm := NewProjectModel(st, statePath, w, h)
	return AppModel{
		project:   &pm,
		cfg:       cfg,
		st:        st,
		statePath: statePath,
		width:     w,
		height:    h,
	}
}

func (m AppModel) Init() tea.Cmd { return nil }

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sz.Width
		m.height = sz.Height
	}

	// ── task view ─────────────────────────────────────────────────────────────
	if m.task != nil {
		switch msg.(type) {
		case BackToBoardMsg:
			updated, cmd := m.board.Update(msg)
			b := updated.(BoardModel)
			m.board = &b
			m.task = nil
			return m, cmd
		}
		updated, cmd := m.task.Update(msg)
		t := updated.(TaskModel)
		m.task = &t
		return m, cmd
	}

	// ── board view ────────────────────────────────────────────────────────────
	if m.board != nil {
		switch msg := msg.(type) {
		case BackToProjectsMsg:
			pm := NewProjectModel(m.st, m.statePath, m.width, m.height)
			m.project = &pm
			m.board = nil
			return m, nil
		case OpenTaskMsg:
			var found *state.Task
			for i := range m.st.Tasks {
				if m.st.Tasks[i].ID == msg.TaskID {
					found = &m.st.Tasks[i]
					break
				}
			}
			if found != nil {
				tm := NewTaskModel(*found, m.st, m.statePath, m.cfg, m.width, m.height)
				m.task = &tm
				if msg.AutoLaunch {
					updated, launchCmd := tm.launchAgent()
					tm = updated
					m.task = &tm
					return m, tea.Batch(tm.Init(), launchCmd)
				}
				return m, tm.Init()
			}
			return m, nil
		}
		updated, cmd := m.board.Update(msg)
		b := updated.(BoardModel)
		m.board = &b
		return m, cmd
	}

	// ── project selector ──────────────────────────────────────────────────────
	if m.project != nil {
		switch msg := msg.(type) {
		case ProjectSelectedMsg:
			board := NewBoardModel(m.st, msg.ProjectID, m.statePath)
			board.width = m.width
			board.height = m.height
			m.board = &board
			m.project = nil
			return m, nil
		}
		updated, cmd := m.project.Update(msg)
		p := updated.(ProjectModel)
		m.project = &p
		return m, cmd
	}

	return m, nil
}

func (m AppModel) View() string {
	if m.task != nil {
		return m.task.View()
	}
	if m.board != nil {
		return m.board.View()
	}
	if m.project != nil {
		return m.project.View()
	}
	return ""
}

package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/agrajgarg/orka/internal/config"
	"github.com/agrajgarg/orka/internal/state"
	"github.com/agrajgarg/orka/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "orka",
	Short: "TUI kanban board for AI coding agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		statePath := state.DefaultStatePath()
		st, err := state.Load(statePath)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}

		cwd, _ := os.Getwd()
		projectID := ensureProject(st, cwd, statePath)

		_ = config.WriteDefaultPrompts(config.DefaultConfigPath())

		model := tui.NewBoardModel(st, projectID, statePath)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}

func ensureProject(st *state.State, path, statePath string) string {
	for _, p := range st.Projects {
		if p.Path == path {
			return p.ID
		}
	}
	name := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			name = path[i+1:]
			break
		}
	}
	p := st.AddProject(name, path)
	_ = st.Save(statePath)
	return p.ID
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

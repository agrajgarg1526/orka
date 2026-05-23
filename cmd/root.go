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

		cfgPath := config.DefaultConfigPath()
		_ = config.WriteDefaultPrompts(cfgPath)
		cfg, err := config.Load(cfgPath)
		if err != nil {
			cfg = config.Default()
		}

		model := tui.NewAppModel(st, statePath, cfg, 0, 0)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}


func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

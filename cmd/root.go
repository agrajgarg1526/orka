package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "orka",
	Short: "TUI kanban board for AI coding agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("board TUI — coming soon")
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/agrajgarg/orka/internal/mcp"
	"github.com/agrajgarg/orka/internal/state"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio)",
	RunE: func(cmd *cobra.Command, args []string) error {
		statePath := state.DefaultStatePath()
		st, err := state.Load(statePath)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}
		return mcp.Serve(st, statePath)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

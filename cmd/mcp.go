package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("mcp server — coming soon")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

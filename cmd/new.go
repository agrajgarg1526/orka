package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/agrajgarg/orka/internal/state"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Initialize orka in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		statePath := state.DefaultStatePath()
		st, err := state.Load(statePath)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}

		_ = ensureProject(st, cwd, statePath)

		mcpPath := filepath.Join(cwd, ".mcp.json")
		if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
			mcpContent := map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"orka": map[string]interface{}{
						"command": "orka",
						"args":    []string{"mcp"},
					},
				},
			}
			data, _ := json.MarshalIndent(mcpContent, "", "  ")
			if err := os.WriteFile(mcpPath, data, 0644); err != nil {
				return fmt.Errorf("write .mcp.json: %w", err)
			}
			fmt.Println("Created .mcp.json")
		}

		fmt.Printf("orka initialized for project at %s\n", cwd)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}

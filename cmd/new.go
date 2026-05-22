package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Initialize orka in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("new project — coming soon")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}

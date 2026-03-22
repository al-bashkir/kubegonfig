package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the currently active profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := mgr.GetCurrent()
		if err != nil {
			return err
		}
		if name == "" {
			info("No active profile")
			return nil
		}
		fmt.Println(name)
		return nil
	},
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all kubeconfig profiles",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := mgr.List()
		if err != nil {
			return err
		}

		if len(names) == 0 {
			info("No profiles found")
			return nil
		}

		current, _ := mgr.GetCurrent()

		for _, name := range names {
			marker := "  "
			if name == current {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, name)
		}
		return nil
	},
}

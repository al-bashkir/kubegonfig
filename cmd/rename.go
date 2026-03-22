package cmd

import (
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a kubeconfig profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := mgr.Rename(args[0], args[1]); err != nil {
			return err
		}
		info("Profile %q renamed to %q", args[0], args[1])
		return nil
	},
}

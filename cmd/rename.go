// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:               "rename <old-name> <new-name>",
	Short:             "Rename a kubeconfig profile",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeProfileNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := mgr.Rename(args[0], args[1]); err != nil {
			return err
		}
		info("Profile %q renamed to %q", args[0], args[1])
		return nil
	},
}

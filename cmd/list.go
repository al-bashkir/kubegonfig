// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

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

		current, err := mgr.GetCurrent()
		if err != nil {
			return fmt.Errorf("read current profile: %w", err)
		}

		if len(names) == 0 {
			info("No profiles found")
			return nil
		}

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

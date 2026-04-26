// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"kubegonfig/internal/tmpfile"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove stale temporary kubeconfig files",
	Long: `Remove stale activation kubeconfig files from the runtime directory.
Only valid profile-name-shaped *.yaml files in kubegonfig's runtime
directory are removed.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var count int
		if err := mgr.WithLock(func() error {
			var err error
			count, err = tmpfile.CleanupStale()
			return err
		}); err != nil {
			return err
		}

		if count == 0 {
			info("No temporary files to clean up")
		} else {
			info("Removed %d temporary file(s)", count)
		}
		return nil
	},
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"fmt"

	"kubegonfig/internal/shell"
	"kubegonfig/internal/tmpfile"

	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Activate a profile and print shell export",
	Long: `Decrypt the named profile to a temporary file, record it as the
active profile, and print a shell export command to stdout.

Use with eval to set KUBECONFIG in the current shell:

  eval "$(kubegonfig use my-cluster)"

This is the correct Unix approach: a child process cannot modify
the parent shell's environment. The eval pattern lets the shell
interpret the export command printed by kubegonfig.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		data, err := mgr.Decrypt(name)
		if err != nil {
			return err
		}

		path, err := tmpfile.Create(name, data)
		if err != nil {
			return err
		}

		if err := mgr.SetCurrent(name); err != nil {
			return err
		}

		// Print the export statement to stdout (for eval).
		fmt.Println(shell.FormatExport("KUBECONFIG", path))

		info("Switched to profile %q", name)
		return nil
	},
}

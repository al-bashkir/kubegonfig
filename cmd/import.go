// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"github.com/spf13/cobra"
)

var importFrom string

var importCmd = &cobra.Command{
	Use:   "import <name> --from <path>",
	Short: "Import a kubeconfig from a file",
	Long: `Read a kubeconfig file, validate it, encrypt it with GPG,
and store it as a named profile.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := mgr.Import(name, importFrom); err != nil {
			return err
		}

		info("Profile %q imported", name)
		return nil
	},
}

func init() {
	importCmd.Flags().StringVar(&importFrom, "from", "", "path to kubeconfig file (required)")
	// MarkFlagRequired only errors if the flag is unregistered, which cannot
	// happen here since it is defined on the line above.
	_ = importCmd.MarkFlagRequired("from")
}

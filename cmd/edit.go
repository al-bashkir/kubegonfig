// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"kubegonfig/internal/editor"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit an existing profile's kubeconfig",
	Long: `Decrypt the profile, open it in your editor, then re-encrypt and
save it. The temporary plaintext file is cleaned up immediately.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProfileNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		changed, err := mgr.Edit(name, editor.Edit)
		if err != nil {
			return err
		}
		if !changed {
			info("No changes made")
			return nil
		}

		info("Profile %q updated", name)
		return nil
	},
}

package cmd

import (
	"bytes"
	"fmt"

	"kubegonfig/internal/editor"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit an existing profile's kubeconfig",
	Long: `Decrypt the profile, open it in your editor, then re-encrypt and
save it. The temporary plaintext file is cleaned up immediately.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Decrypt current content.
		original, err := mgr.Decrypt(name)
		if err != nil {
			return err
		}

		// Open in editor.
		edited, err := editor.Edit(original)
		if err != nil {
			return fmt.Errorf("editor: %w", err)
		}

		// Check if content changed.
		if bytes.Equal(original, edited) {
			info("No changes made")
			return nil
		}

		// Re-encrypt and save.
		if err := mgr.Update(name, edited); err != nil {
			return err
		}

		info("Profile %q updated", name)
		return nil
	},
}

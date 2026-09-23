// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"fmt"
	"os"

	"kubegonfig/internal/editor"

	"github.com/spf13/cobra"
)

const createTemplate = `# Replace these placeholder values with your kubeconfig and save.
# The saved file is validated and encrypted with GPG.
apiVersion: v1
kind: Config
clusters:
- name: cluster
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: context
  context:
    cluster: cluster
    user: user
current-context: context
users:
- name: user
`

var createFrom string

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new kubeconfig profile",
	Long: `Create a new encrypted kubeconfig profile.

If --from is specified, the kubeconfig is read from the given file.
Otherwise, your $VISUAL or $EDITOR is opened to paste or write the kubeconfig content.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		var data []byte
		var err error
		if createFrom != "" {
			data, err = os.ReadFile(createFrom)
			if err != nil {
				return fmt.Errorf("read kubeconfig: %w", err)
			}
		} else {
			data, err = editor.Edit([]byte(createTemplate))
			if err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			if len(data) == 0 {
				return fmt.Errorf("empty content; profile not created")
			}
		}

		if err := mgr.Create(name, data); err != nil {
			return err
		}

		info("Profile %q created", name)
		return nil
	},
}

func init() {
	createCmd.Flags().StringVar(&createFrom, "from", "", "read the kubeconfig from a file instead of opening an editor")
}

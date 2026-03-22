package cmd

import (
	"fmt"
	"os"

	"kubegonfig/internal/editor"

	"github.com/spf13/cobra"
)

var createFrom string

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new kubeconfig profile",
	Long: `Create a new encrypted kubeconfig profile.

If --from is specified, the kubeconfig is read from the given file.
Otherwise, your $EDITOR is opened to paste or write the kubeconfig content.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if createFrom != "" {
			return mgr.Import(name, createFrom)
		}

		// Open editor for interactive creation.
		template := []byte(`# Paste your kubeconfig below and save.
# This file will be validated and encrypted with GPG.
apiVersion: v1
kind: Config
clusters: []
contexts: []
users: []
current-context: ""
`)
		data, err := editor.Edit(template)
		if err != nil {
			return fmt.Errorf("editor: %w", err)
		}

		if len(data) == 0 {
			return fmt.Errorf("empty content; profile not created")
		}

		if err := mgr.Create(name, data); err != nil {
			return err
		}

		info("Profile %q created", name)
		return nil
	},
}

func init() {
	createCmd.Flags().StringVar(&createFrom, "from", "", "import kubeconfig from file path")
	// Allow stdin detection for future pipe support.
	_ = os.Stdin
}

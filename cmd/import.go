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

		info("Profile %q imported from %s", name, importFrom)
		return nil
	},
}

func init() {
	importCmd.Flags().StringVar(&importFrom, "from", "", "path to kubeconfig file (required)")
	_ = importCmd.MarkFlagRequired("from")
}

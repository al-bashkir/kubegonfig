// Package cmd defines all Cobra commands for the kubegonfig CLI.
package cmd

import (
	"fmt"
	"os"

	"kubegonfig/internal/config"
	"kubegonfig/internal/crypto"
	"kubegonfig/internal/profile"
	"kubegonfig/internal/tmpfile"

	"github.com/spf13/cobra"
)

var (
	quiet bool
	cfg   *config.Config
	mgr   *profile.Manager
)

// rootCmd is the base command for kubegonfig.
var rootCmd = &cobra.Command{
	Use:   "kubegonfig",
	Short: "Secure kubeconfig profile manager",
	Long: `kubegonfig stores kubeconfig files encrypted with GPG and provides
safe shell integration for switching between Kubernetes contexts.

A child process cannot modify the parent shell's environment directly.
Use eval or a shell wrapper function to export KUBECONFIG:

  eval "$(kubegonfig env my-cluster)"

Or add a helper function to your shell rc file:

  kuse() { eval "$(kubegonfig env "$1")"; }`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip setup for commands that don't need it.
		switch cmd.Name() {
		case "init", "help", "completion", "version":
			return nil
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// Commands that need GPG and profile manager.
		switch cmd.Name() {
		case "list", "current", "cleanup":
			// These don't need GPG check.
		default:
			if err := crypto.CheckGPG(); err != nil {
				return err
			}
		}

		mgr, err = profile.NewManager(cfg)
		if err != nil {
			return fmt.Errorf("init profile manager: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")

	rootCmd.AddCommand(
		initCmd,
		createCmd,
		importCmd,
		listCmd,
		useCmd,
		currentCmd,
		deleteCmd,
		renameCmd,
		envCmd,
		execCmd,
		editCmd,
		cleanupCmd,
	)
}

// SetVersion sets the version string shown by --version.
func SetVersion(v string) {
	rootCmd.Version = v
}

// Execute is the main entry point for the CLI.
func Execute() {
	tmpfile.SetupSignalHandler()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// info prints a message to stderr unless quiet mode is on.
func info(format string, args ...any) {
	if !quiet {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

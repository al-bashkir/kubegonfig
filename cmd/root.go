// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package cmd defines all Cobra commands for the kubegonfig CLI.
package cmd

import (
	"fmt"
	"os"
	"strings"

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
		if skipSetup(cmd, args) {
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

func skipSetup(cmd *cobra.Command, args []string) bool {
	if cmd.DisableFlagParsing && isHelpRequest(args) {
		return true
	}
	if strings.HasPrefix(cmd.CommandPath(), "kubegonfig completion") {
		return true
	}
	switch cmd.Name() {
	case "init", "help", "completion", "version", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		return true
	default:
		return false
	}
}

func isHelpRequest(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
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
		unlockCmd,
		exportCmd,
		restoreCmd,
	)
}

// version is the running binary version, set by SetVersion at startup.
var version = "dev"

// SetVersion sets the version string shown by --version and recorded in
// exported archives.
func SetVersion(v string) {
	rootCmd.Version = v
	if v != "" {
		version = v
	}
}

// Version returns the current running version (set by SetVersion).
func Version() string { return version }

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

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"fmt"

	"kubegonfig/internal/shell"
	"kubegonfig/internal/tmpfile"

	"github.com/spf13/cobra"
)

var envShell string

var envCmd = &cobra.Command{
	Use:   "env <name>",
	Short: "Print shell export for a profile's kubeconfig",
	Long: `Decrypt the named profile to a temporary file and print a shell
command to export KUBECONFIG. The output is designed to be consumed
by eval:

  eval "$(kubegonfig env my-cluster)"

For fish shell:

  kubegonfig env my-cluster --shell fish | source

Why eval? A child process (kubegonfig) cannot modify the environment
of its parent process (your shell). This is a fundamental Unix
process model constraint. The eval pattern lets your shell interpret
the export command that kubegonfig prints to stdout.`,
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

		shellStyle := envShell
		if shellStyle == "" {
			shellStyle = cfg.ShellStyle
		}
		if shellStyle == "" {
			shellStyle = "posix"
		}

		switch shellStyle {
		case "fish":
			fmt.Println(shell.FormatFishSet("KUBECONFIG", path))
		default:
			fmt.Println(shell.FormatExport("KUBECONFIG", path))
		}

		return nil
	},
}

func init() {
	envCmd.Flags().StringVar(&envShell, "shell", "", "output format: posix (default), fish")
}

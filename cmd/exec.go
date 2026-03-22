// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"kubegonfig/internal/tmpfile"

	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <name> -- <command> [args...]",
	Short: "Run a command with KUBECONFIG set to the named profile",
	Long: `Decrypt the profile, set KUBECONFIG in a child process environment,
run the specified command, and clean up the temporary file afterward.

This is the most secure activation method: the decrypted kubeconfig
exists only for the duration of the child command and is removed
immediately after.

Example:
  kubegonfig exec prod -- kubectl get pods
  kubegonfig exec staging -- helm list`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse: first arg is profile name, rest after "--" is the command.
		name := args[0]
		var cmdArgs []string

		for i := 1; i < len(args); i++ {
			if args[i] == "--" {
				cmdArgs = args[i+1:]
				break
			}
		}

		if len(cmdArgs) == 0 {
			return fmt.Errorf("no command specified after --")
		}

		data, err := mgr.Decrypt(name)
		if err != nil {
			return err
		}

		path, err := tmpfile.Create(name, data)
		if err != nil {
			return err
		}
		// Always clean up after exec finishes.
		defer tmpfile.Remove(path)

		child := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		child.Env = appendEnv(os.Environ(), "KUBECONFIG", path)

		if err := child.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("exec: %w", err)
		}
		return nil
	},
}

// appendEnv returns env with key=value added or replaced.
func appendEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if len(e) > len(prefix) && e[:len(prefix)] == prefix {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

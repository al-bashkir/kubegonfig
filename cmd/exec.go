// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"kubegonfig/internal/tmpfile"

	"github.com/spf13/cobra"
)

const execKubeconfigFD = 3

var execCmd = &cobra.Command{
	Use:   "exec <name> -- <command> [args...]",
	Short: "Run a command with KUBECONFIG set to the named profile",
	Long: `Decrypt the profile, set KUBECONFIG in a child process environment,
and run the specified command.

This is the most secure activation method: the decrypted kubeconfig
is unlinked before the child command continues and is passed through
an inherited file descriptor.

Example:
  kubegonfig exec prod -- kubectl get pods
  kubegonfig exec staging -- helm list`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isHelpRequest(args) {
			return cmd.Help()
		}

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

		exitCode, err := runExecProfile(name, cmdArgs)
		if err != nil {
			return err
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

func runExecProfile(name string, cmdArgs []string) (int, error) {
	var kubeconfig *os.File
	if err := mgr.WithLock(func() error {
		data, err := mgr.Decrypt(name)
		if err != nil {
			return err
		}

		kubeconfig, err = tmpfile.OpenUnlinked(name, data)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		if kubeconfig != nil {
			_ = kubeconfig.Close()
		}
		return 0, err
	}

	return runExecWithKubeconfigFile(cmdArgs, kubeconfig)
}

func runExecWithKubeconfigFile(cmdArgs []string, kubeconfig *os.File) (int, error) {
	exitCode, runErr := runCommandWithKubeconfig(cmdArgs, kubeconfig)
	closeErr := kubeconfig.Close()
	if runErr != nil && closeErr != nil {
		return exitCode, errors.Join(runErr, fmt.Errorf("close temp kubeconfig: %w", closeErr))
	}
	if closeErr != nil {
		return exitCode, fmt.Errorf("close temp kubeconfig: %w", closeErr)
	}
	return exitCode, runErr
}

func runCommandWithKubeconfig(cmdArgs []string, kubeconfig *os.File) (int, error) {
	child := newExecCommandWithKubeconfig(cmdArgs, kubeconfig)

	if err := child.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 0, fmt.Errorf("exec: %w", err)
	}
	return 0, nil
}

func newExecCommandWithKubeconfig(cmdArgs []string, kubeconfig *os.File) *exec.Cmd {
	child := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = appendEnv(os.Environ(), "KUBECONFIG", execKubeconfigPath())
	child.ExtraFiles = []*os.File{kubeconfig}
	return child
}

func execKubeconfigPath() string {
	if runtime.GOOS == "linux" {
		return fmt.Sprintf("/proc/self/fd/%d", execKubeconfigFD)
	}
	return fmt.Sprintf("/dev/fd/%d", execKubeconfigFD)
}

// appendEnv returns env with key=value added or replaced.
func appendEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

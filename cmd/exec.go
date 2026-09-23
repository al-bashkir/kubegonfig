// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

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
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: completeExecProfileName,
	RunE: func(cmd *cobra.Command, args []string) error {
		// First arg is the profile name; everything after "--" is the command.
		dash := cmd.ArgsLenAtDash()
		if dash < 1 || dash == len(args) {
			return fmt.Errorf("no command specified after --")
		}

		exitCode, err := runExecProfile(args[0], args[dash:])
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

func runExecWithKubeconfigFile(cmdArgs []string, kubeconfig *os.File) (exitCode int, err error) {
	defer func() {
		if closeErr := kubeconfig.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close temp kubeconfig: %w", closeErr))
		}
	}()

	if err := newExecCommandWithKubeconfig(cmdArgs, kubeconfig).Run(); err != nil {
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
	// exec.Cmd keeps only the last value of a duplicated key.
	child.Env = append(os.Environ(), "KUBECONFIG="+execKubeconfigPath())
	child.ExtraFiles = []*os.File{kubeconfig}
	return child
}

func execKubeconfigPath() string {
	if runtime.GOOS == "linux" {
		return fmt.Sprintf("/proc/self/fd/%d", execKubeconfigFD)
	}
	return fmt.Sprintf("/dev/fd/%d", execKubeconfigFD)
}

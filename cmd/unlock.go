// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"errors"
	"fmt"

	"kubegonfig/internal/shell"
	"kubegonfig/internal/tmpfile"

	"github.com/spf13/cobra"
)

var unlockCmd = &cobra.Command{
	Use:   "unlock <name> [<name>...]",
	Short: "Decrypt one or more profiles to the runtime dir without activating any",
	Long: `Decrypt the named profiles to the runtime directory without changing
the active profile and without printing a shell export.

Useful when tooling needs several decrypted kubeconfigs available
simultaneously, for example to merge them via:

  KUBECONFIG="$(kubegonfig unlock a b c >/dev/null && \
    echo $XDG_RUNTIME_DIR/kubegonfig/a.yaml:$XDG_RUNTIME_DIR/kubegonfig/b.yaml:$XDG_RUNTIME_DIR/kubegonfig/c.yaml)" kubectl get pods

Successfully unlocked profiles are reported one per line on stderr
(suppressed by --quiet). The active profile, as reported by
"kubegonfig current", is not modified.`,
	// Pass nil args so names keep completing for every variadic position.
	ValidArgsFunction: func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeProfileNames(cmd, nil, toComplete)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return runUnlock(args)
	},
}

func runUnlock(args []string) error {
	var errs []error
	seen := make(map[string]struct{}, len(args))

	for _, name := range args {
		if err := shell.ValidateName(name); err != nil {
			errs = append(errs, fmt.Errorf("unlock %q: %w", name, err))
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}

		path, err := mgr.Unlock(name, func(data []byte) (string, error) {
			return tmpfile.Create(name, data)
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("unlock %q: %w", name, err))
			continue
		}
		info("Unlocked %q -> %s", name, path)
	}

	return errors.Join(errs...)
}

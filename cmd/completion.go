// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"fmt"
	"strings"

	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"

	"github.com/spf13/cobra"
)

// completeProfileNames loads config and the profile manager directly rather
// than reusing the package globals: cobra runs completion in a separate
// __complete invocation where PersistentPreRunE has not run, so cfg/mgr are nil.
func completeProfileNames(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg, err := config.Load()
	if err != nil {
		cobra.CompErrorln(fmt.Sprintf("load config: %v", err))
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	mgr, err := profile.NewManager(cfg)
	if err != nil {
		cobra.CompErrorln(fmt.Sprintf("init profile manager: %v", err))
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names, err := mgr.List()
	if err != nil {
		cobra.CompErrorln(fmt.Sprintf("list profiles: %v", err))
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	completions := make([]string, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(name, toComplete) {
			completions = append(completions, name)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeRenameOldProfile(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeProfileNames(cmd, args, toComplete)
}

func completeExecProfileName(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return completeProfileNames(cmd, args, toComplete)
}

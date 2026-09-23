// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExecRequiresCommandAfterSeparator(t *testing.T) {
	for _, args := range [][]string{{"prod"}, {"prod", "--"}, {"prod", "kubectl"}, {"--", "kubectl"}} {
		cmd := &cobra.Command{Use: "exec", RunE: execCmd.RunE}
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatalf("ParseFlags(%q) error = %v", args, err)
		}
		err := cmd.RunE(cmd, cmd.Flags().Args())
		if err == nil || !strings.Contains(err.Error(), "no command specified") {
			t.Fatalf("exec %q error = %v, want no command specified", args, err)
		}
	}
}

func TestSkipSetupForCompletionSubcommand(t *testing.T) {
	completion := &cobra.Command{Use: "completion"}
	bash := &cobra.Command{Use: "bash"}
	completion.AddCommand(bash)
	rootCmd.AddCommand(completion)
	t.Cleanup(func() { rootCmd.RemoveCommand(completion) })

	if !skipSetup(bash) {
		t.Fatal("skipSetup() = false, want true for completion subcommand")
	}
}

func TestSkipSetupForShellCompletionRequest(t *testing.T) {
	cmd := &cobra.Command{Use: cobra.ShellCompRequestCmd}

	if !skipSetup(cmd) {
		t.Fatal("skipSetup() = false, want true for shell completion request")
	}
}

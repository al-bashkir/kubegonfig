// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestSkipSetupForHelpRequest(t *testing.T) {
	cmd := &cobra.Command{Use: "exec", DisableFlagParsing: true}
	cmd.SetHelpCommand(&cobra.Command{Use: "help"})

	if !skipSetup(cmd, []string{"--help"}) {
		t.Fatal("skipSetup() = false, want true for help request")
	}
}

func TestSkipSetupDoesNotTreatParsedArgsAsHelpRequest(t *testing.T) {
	cmd := &cobra.Command{Use: "delete"}

	if skipSetup(cmd, []string{"-h"}) {
		t.Fatal("skipSetup() = true, want false for parsed positional help-like arg")
	}
}

func TestHelpRequestIgnoresArgsAfterSeparator(t *testing.T) {
	if isHelpRequest([]string{"prod", "--", "sh", "-h"}) {
		t.Fatal("isHelpRequest() = true, want false for child command help flag")
	}
}

func TestSkipSetupForCompletionSubcommand(t *testing.T) {
	completion := &cobra.Command{Use: "completion"}
	bash := &cobra.Command{Use: "bash"}
	completion.AddCommand(bash)
	rootCmd.AddCommand(completion)
	t.Cleanup(func() { rootCmd.RemoveCommand(completion) })

	if !skipSetup(bash, nil) {
		t.Fatal("skipSetup() = false, want true for completion subcommand")
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteProfileNames(t *testing.T) {
	setupCompletionProfiles(t, "beta", "alpha")

	got, directive := completeProfileNames(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion directive = %v, want NoFileComp", directive)
	}
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completeProfileNames() = %v, want %v", got, want)
	}
}

func TestCompleteProfileNamesFiltersPrefix(t *testing.T) {
	setupCompletionProfiles(t, "prod", "stage", "staging")

	got, directive := completeProfileNames(nil, nil, "sta")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion directive = %v, want NoFileComp", directive)
	}
	want := []string{"stage", "staging"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completeProfileNames() = %v, want %v", got, want)
	}
}

func TestExecCommandCompletesProfileName(t *testing.T) {
	setupCompletionProfiles(t, "prod")

	got, directive := execCmd.ValidArgsFunction(execCmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion directive = %v, want NoFileComp", directive)
	}
	want := []string{"prod"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exec completion = %v, want %v", got, want)
	}
}

func TestProfileCompletionStopsAfterProfileArg(t *testing.T) {
	setupCompletionProfiles(t, "prod")

	got, directive := completeProfileNames(nil, []string{"prod"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion directive = %v, want NoFileComp", directive)
	}
	if len(got) != 0 {
		t.Fatalf("completeProfileNames() after profile arg = %v, want none", got)
	}
}

func TestExecCompletionUsesDefaultAfterProfileArg(t *testing.T) {
	setupCompletionProfiles(t, "prod")

	got, directive := execCmd.ValidArgsFunction(execCmd, []string{"prod"}, "")
	if directive != cobra.ShellCompDirectiveDefault {
		t.Fatalf("completion directive = %v, want default", directive)
	}
	if len(got) != 0 {
		t.Fatalf("exec completion after profile arg = %v, want none", got)
	}
}

func TestRenameCompletesOnlyOldProfileName(t *testing.T) {
	setupCompletionProfiles(t, "prod")

	got, directive := renameCmd.ValidArgsFunction(renameCmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion directive = %v, want NoFileComp", directive)
	}
	want := []string{"prod"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rename old-name completion = %v, want %v", got, want)
	}

	got, directive = renameCmd.ValidArgsFunction(renameCmd, []string{"prod"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion directive = %v, want NoFileComp", directive)
	}
	if len(got) != 0 {
		t.Fatalf("rename new-name completion = %v, want none", got)
	}
}

func setupCompletionProfiles(t *testing.T, names ...string) {
	t.Helper()

	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(base, "data"))

	profilesDir := filepath.Join(base, "data", "kubegonfig", "profiles")
	if err := os.MkdirAll(profilesDir, 0700); err != nil {
		t.Fatalf("create profiles dir: %v", err)
	}
	for _, name := range names {
		path := filepath.Join(profilesDir, name+".yaml.gpg")
		if err := os.WriteFile(path, []byte("encrypted"), 0600); err != nil {
			t.Fatalf("write profile %q: %v", name, err)
		}
	}
}

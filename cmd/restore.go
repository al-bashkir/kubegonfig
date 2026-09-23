// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"fmt"
	"os"

	"kubegonfig/internal/bundle"

	"github.com/spf13/cobra"
)

var (
	restoreForce        bool
	restoreSkipExisting bool
	restoreMergeConfig  bool
	restoreProfilesOnly bool
	restoreDryRun       bool
)

var restoreCmd = &cobra.Command{
	Use:   "restore <archive>",
	Short: "Restore profiles from a tar archive",
	Long: `Read an archive produced by "kubegonfig export" and write its
profiles into the local store. Pass "-" to read the archive from stdin.

By default the command refuses if any archived profile name already
exists locally. Use --force to overwrite or --skip-existing to keep
local copies.

Plaintext archives are re-encrypted with the LOCAL GPG recipients before
being stored. The archive's recipient list is never used for re-encryption.

Restore is additive and re-runnable: a per-profile failure does not roll
back profiles already written. Use --dry-run to validate the archive and
preview the plan without writing.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRestore(args[0])
	},
}

func runRestore(archive string) error {
	var in *os.File
	if archive == "-" {
		in = os.Stdin
	} else {
		f, err := os.Open(archive)
		if err != nil {
			return fmt.Errorf("open archive: %w", err)
		}
		defer func() { _ = f.Close() }()
		in = f
	}

	plan, err := bundle.Restore(mgr, cfg, bundle.RestoreOptions{
		In:           in,
		Force:        restoreForce,
		SkipExisting: restoreSkipExisting,
		MergeConfig:  restoreMergeConfig,
		ProfilesOnly: restoreProfilesOnly,
		DryRun:       restoreDryRun,
	})
	if err != nil {
		return err
	}

	verb := "Restored"
	if restoreDryRun {
		verb = "Would restore"
	}
	info("%s %d profile(s); skipped %d; overwrote %d; config: %s",
		verb, len(plan.ToCreate), len(plan.ToSkip), len(plan.ToOverwrite), plan.ConfigAction)
	return nil
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "overwrite profiles whose names already exist locally")
	restoreCmd.Flags().BoolVar(&restoreSkipExisting, "skip-existing", false, "keep local copies; restore only new names")
	restoreCmd.Flags().BoolVar(&restoreMergeConfig, "merge-config", false, "union archived config recipients into the local config")
	restoreCmd.Flags().BoolVar(&restoreProfilesOnly, "profiles-only", false, "ignore archived config.yaml even with --merge-config")
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "parse and validate the archive, print the plan, and exit")
}

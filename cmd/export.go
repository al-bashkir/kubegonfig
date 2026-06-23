// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kubegonfig/internal/bundle"
	"kubegonfig/internal/shell"
	"kubegonfig/internal/storage"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	exportOutput       string
	exportAll          bool
	exportDecrypt      bool
	exportProfilesOnly bool
)

var exportCmd = &cobra.Command{
	Use:   "export [name...]",
	Short: "Export profiles to a tar archive",
	Long: `Write an uncompressed tar archive containing the selected profiles
and (by default) config.yaml.

The archive contains encrypted .yaml.gpg blobs by default. Pass --decrypt
to emit plaintext kubeconfigs instead — refused when stdout is a terminal
or when the destination resolves inside the kubegonfig data directory.

Restore with: kubegonfig restore <archive>`,
	ValidArgsFunction: completeProfileNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runExport(args)
	},
}

func runExport(args []string) error {
	if exportOutput == "" {
		return fmt.Errorf("--output/-o is required (use \"-\" for stdout)")
	}
	if exportAll && len(args) > 0 {
		return fmt.Errorf("--all is mutually exclusive with positional profile names")
	}
	if !exportAll && len(args) == 0 {
		return fmt.Errorf("specify profile names or --all")
	}

	names := args
	if exportAll {
		all, err := mgr.List()
		if err != nil {
			return fmt.Errorf("list profiles: %w", err)
		}
		if len(all) == 0 {
			return fmt.Errorf("no profiles to export")
		}
		names = all
	}
	for _, n := range names {
		if err := shell.ValidateName(n); err != nil {
			return fmt.Errorf("export %q: %w", n, err)
		}
	}

	if exportDecrypt {
		if err := guardPlaintextDestination(exportOutput); err != nil {
			return err
		}
	}

	opts := bundle.ExportOptions{
		Names:             names,
		Decrypt:           exportDecrypt,
		IncludeConfig:     !exportProfilesOnly,
		KubegonfigVersion: Version(),
	}

	if exportOutput == "-" {
		opts.Out = os.Stdout
		return bundle.Export(mgr, cfg, opts)
	}

	abs, err := filepath.Abs(exportOutput)
	if err != nil {
		return fmt.Errorf("resolve --output path: %w", err)
	}
	dir := filepath.Dir(abs)
	name := filepath.Base(abs)

	writeErr := storage.AtomicWriteStream(dir, name, 0600, func(w io.Writer) error {
		opts.Out = w
		return bundle.Export(mgr, cfg, opts)
	})
	if writeErr != nil {
		return writeErr
	}
	info("Wrote %s", abs)
	return nil
}

// guardPlaintextDestination refuses --decrypt output to a TTY or to a path
// inside the kubegonfig data directory.
func guardPlaintextDestination(out string) error {
	if out == "-" {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			return errors.New("refusing to write plaintext kubeconfigs to a terminal; redirect stdout or use --output <file>")
		}
		return nil
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return fmt.Errorf("resolve --output path: %w", err)
	}
	dataDir, err := cfg.ResolveDataDir()
	if err != nil {
		return err
	}
	dataAbs, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}
	rel, err := filepath.Rel(dataAbs, abs)
	escapes := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
	if err == nil && !filepath.IsAbs(rel) && !escapes {
		return fmt.Errorf("refusing to write plaintext archive inside the kubegonfig data directory %s", dataAbs)
	}
	return nil
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "tar output path; \"-\" for stdout (required)")
	exportCmd.Flags().BoolVar(&exportAll, "all", false, "export every profile")
	exportCmd.Flags().BoolVar(&exportDecrypt, "decrypt", false, "emit plaintext kubeconfigs (refused into a TTY or the data directory)")
	exportCmd.Flags().BoolVar(&exportProfilesOnly, "profiles-only", false, "omit config.yaml from the archive")
}

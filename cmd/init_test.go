// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"kubegonfig/internal/config"

	"github.com/spf13/cobra"
)

func TestInitRecipientFlag(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      string
		wantError bool
	}{
		{name: "trimmed", value: " user@example.com ", want: "user@example.com"},
		{name: "blank", value: " \t ", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpgDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(gpgDir, "gpg"), []byte("#!/bin/sh\n"), 0700); err != nil {
				t.Fatalf("write fake gpg: %v", err)
			}
			t.Setenv("PATH", gpgDir)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Cleanup(func() { initRecipient = "" })

			cmd := &cobra.Command{Use: "init", RunE: initCmd.RunE}
			cmd.Flags().StringVarP(&initRecipient, "recipient", "r", "", "")
			if err := cmd.ParseFlags([]string{"--recipient", tt.value}); err != nil {
				t.Fatalf("ParseFlags() error = %v", err)
			}
			err := cmd.RunE(cmd, nil)
			if (err != nil) != tt.wantError {
				t.Fatalf("init error = %v, wantError %v", err, tt.wantError)
			}

			c, err := config.Load()
			if err != nil {
				t.Fatalf("config.Load() error = %v", err)
			}
			if c.GPGRecipient != tt.want {
				t.Fatalf("saved recipient = %q, want %q", c.GPGRecipient, tt.want)
			}
		})
	}
}

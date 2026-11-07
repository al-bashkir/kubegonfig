// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
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

func initTestFakeGPG(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"gpg2", "gpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
			t.Fatalf("write fake gpg: %v", err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const initTestTwoKeys = `cat <<'EOF'
sec:u:255:22:AAAA1111AAAA1111:1700000000:::u:::scESC:::+:::23::0:
fpr:::::::::FPRONE:
uid:u::::1700000000::H1::Alice <a@x>:
uid:u::::1700000000::H2::Alice Work <a@w>:
sec:u:255:22:BBBB2222BBBB2222:1700000000:::u:::scESC:::+:::23::0:
uid:u::::1700000000::H3::Bob <b@x>:
EOF`

func TestSelectGPGKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "fingerprint preferred", input: "1\n", want: "FPRONE"},
		{name: "key id when no fingerprint", input: " 2 \n", want: "BBBB2222BBBB2222"},
		{name: "out of range high", input: "3\n", wantErr: "invalid selection: 3"},
		{name: "out of range zero", input: "0\n", wantErr: "invalid selection: 0"},
		{name: "not a number", input: "abc\n", wantErr: "invalid selection: abc"},
		{name: "empty line", input: "\n", wantErr: "invalid selection: "},
		{name: "no input", input: "", wantErr: "no input received"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initTestFakeGPG(t, initTestTwoKeys)
			var out strings.Builder
			got, err := selectGPGKey(strings.NewReader(tt.input), &out)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil || got != tt.want {
				t.Fatalf("selectGPGKey() = %q, %v; want %q", got, err, tt.want)
			}

			wantPrompt := "Available GPG secret keys:\n\n" +
				"  1) AAAA1111AAAA1111\n     Alice <a@x>\n     Alice Work <a@w>\n\n" +
				"  2) BBBB2222BBBB2222\n     Bob <b@x>\n\n" +
				"Select key [1-2]: "
			if out.String() != wantPrompt {
				t.Errorf("prompt = %q, want %q", out.String(), wantPrompt)
			}
		})
	}
}

func TestSelectGPGKeyNoKeys(t *testing.T) {
	initTestFakeGPG(t, "exit 0")
	var out strings.Builder
	_, err := selectGPGKey(strings.NewReader("1\n"), &out)
	want := "no GPG secret keys found; generate one with: gpg --gen-key"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected output %q", out.String())
	}
}

func TestSelectGPGKeyListFailure(t *testing.T) {
	initTestFakeGPG(t, `echo "gpg: keyring broken" >&2; exit 2`)
	_, err := selectGPGKey(strings.NewReader("1\n"), io.Discard)
	if err == nil || !strings.HasPrefix(err.Error(), "list GPG keys: gpg list-secret-keys: gpg: keyring broken") {
		t.Fatalf("error = %v", err)
	}
}

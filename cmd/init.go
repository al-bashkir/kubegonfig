// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"kubegonfig/internal/config"
	"kubegonfig/internal/crypto"

	"github.com/spf13/cobra"
)

var initRecipient string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize kubegonfig configuration",
	Long: `Create the kubegonfig config file with your GPG recipient.

When run without --recipient, lists available GPG secret keys
and lets you select one interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := crypto.LookupGPG(); err != nil {
			return err
		}

		c, err := config.Load()
		if err != nil {
			return err
		}

		var recipient string
		if cmd.Flags().Changed("recipient") {
			if recipient = strings.TrimSpace(initRecipient); recipient == "" {
				return fmt.Errorf("GPG recipient must not be blank")
			}
		} else if recipient, err = selectGPGKey(cmd.InOrStdin(), cmd.ErrOrStderr()); err != nil {
			return err
		}

		c.GPGRecipient = recipient
		if err := c.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		info("Configuration saved to %s", c.Path)
		return nil
	},
}

// selectGPGKey lists available GPG secret keys and prompts the user to pick one.
func selectGPGKey(in io.Reader, w io.Writer) (string, error) {
	keys, err := crypto.ListSecretKeys()
	if err != nil {
		return "", fmt.Errorf("list GPG keys: %w", err)
	}

	if len(keys) == 0 {
		return "", fmt.Errorf("no GPG secret keys found; generate one with: gpg --gen-key")
	}

	_, _ = io.WriteString(w, keyMenu(keys))

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return "", fmt.Errorf("no input received")
	}

	input := strings.TrimSpace(scanner.Text())
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(keys) {
		return "", fmt.Errorf("invalid selection: %s", input)
	}

	selected := keys[idx-1]

	// Prefer fingerprint over key ID for recipient — more specific.
	if selected.Fingerprint != "" {
		return selected.Fingerprint, nil
	}
	return selected.KeyID, nil
}

func keyMenu(keys []crypto.GPGKey) string {
	var w strings.Builder
	fmt.Fprintln(&w, "Available GPG secret keys:")
	fmt.Fprintln(&w)

	for i, k := range keys {
		fmt.Fprintf(&w, "  %d) %s\n", i+1, k.KeyID)
		for _, uid := range k.UIDs {
			fmt.Fprintf(&w, "     %s\n", uid)
		}
		fmt.Fprintln(&w)
	}

	fmt.Fprintf(&w, "Select key [1-%d]: ", len(keys))
	return w.String()
}

func init() {
	initCmd.Flags().StringVarP(&initRecipient, "recipient", "r", "", "GPG recipient key ID or email (skips selection)")
}

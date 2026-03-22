// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"bufio"
	"fmt"
	"os"
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
		if err := crypto.CheckGPG(); err != nil {
			return err
		}

		c, err := config.Load()
		if err != nil {
			return err
		}

		recipient := initRecipient
		if recipient == "" {
			selected, err := selectGPGKey()
			if err != nil {
				return err
			}
			recipient = selected
		}

		c.GPGRecipient = recipient
		if err := c.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		info("Configuration saved to %s", c.Path())
		return nil
	},
}

// selectGPGKey lists available GPG secret keys and prompts the user to pick one.
func selectGPGKey() (string, error) {
	keys, err := crypto.ListSecretKeys()
	if err != nil {
		return "", fmt.Errorf("list GPG keys: %w", err)
	}

	if len(keys) == 0 {
		return "", fmt.Errorf("no GPG secret keys found; generate one with: gpg --gen-key")
	}

	fmt.Fprintln(os.Stderr, "Available GPG secret keys:")
	fmt.Fprintln(os.Stderr)

	for i, k := range keys {
		uid := ""
		if len(k.UIDs) > 0 {
			uid = k.UIDs[0]
		}
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, k.KeyID)
		if uid != "" {
			fmt.Fprintf(os.Stderr, "     %s\n", uid)
		}
		if len(k.UIDs) > 1 {
			for _, extra := range k.UIDs[1:] {
				fmt.Fprintf(os.Stderr, "     %s\n", extra)
			}
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprintf(os.Stderr, "Select key [1-%d]: ", len(keys))

	scanner := bufio.NewScanner(os.Stdin)
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

func init() {
	initCmd.Flags().StringVarP(&initRecipient, "recipient", "r", "", "GPG recipient key ID or email (skips selection)")
}

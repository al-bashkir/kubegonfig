package cmd

import (
	"bufio"
	"fmt"
	"os"
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
The recipient is the GPG key ID or email used to encrypt kubeconfig files.`,
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
			fmt.Fprint(os.Stderr, "GPG recipient (key ID or email): ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				recipient = strings.TrimSpace(scanner.Text())
			}
			if recipient == "" {
				return fmt.Errorf("GPG recipient is required")
			}
		}

		c.GPGRecipient = recipient
		if err := c.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		info("Configuration saved to %s", c.Path())
		return nil
	},
}

func init() {
	initCmd.Flags().StringVarP(&initRecipient, "recipient", "r", "", "GPG recipient key ID or email")
}

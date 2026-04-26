// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package config manages the kubegonfig application configuration.
// Config is stored at XDG_CONFIG_HOME/kubegonfig/config.yaml.
package config

import (
	"fmt"
	"os"
	"strings"

	"kubegonfig/internal/storage"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	// GPGRecipient is the primary GPG key ID or email for encryption.
	GPGRecipient string `yaml:"gpg_recipient"`

	// GPGRecipients allows encrypting to multiple recipients.
	GPGRecipients []string `yaml:"gpg_recipients,omitempty"`

	// DataDir overrides the default XDG data directory.
	DataDir string `yaml:"data_dir,omitempty"`

	// ShellStyle controls output format: "posix" (default) or "fish".
	ShellStyle string `yaml:"shell_style,omitempty"`

	// path is the resolved config file path (not serialized).
	path string `yaml:"-"`
}

// Load reads the config file. Returns a default Config if the file does not exist.
func Load() (*Config, error) {
	dir, err := storage.ConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}

	path := dir + "/config.yaml"
	cfg := &Config{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.path = path
	return cfg, nil
}

// Save writes the config to disk atomically.
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return storage.AtomicWrite(c.path, data, 0600)
}

// Path returns the resolved config file path.
func (c *Config) Path() string {
	return c.path
}

// Recipients returns the list of GPG recipients for encryption.
// The primary recipient is always included first.
func (c *Config) Recipients() []string {
	var r []string
	seen := make(map[string]struct{})
	add := func(recipient string) {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			return
		}
		if _, ok := seen[recipient]; ok {
			return
		}
		seen[recipient] = struct{}{}
		r = append(r, recipient)
	}

	add(c.GPGRecipient)
	for _, extra := range c.GPGRecipients {
		add(extra)
	}
	return r
}

// ResolveDataDir returns the effective data directory.
func (c *Config) ResolveDataDir() (string, error) {
	if c.DataDir != "" {
		return c.DataDir, nil
	}
	return storage.DataDir()
}

// Validate checks that the config has minimum required fields.
func (c *Config) Validate() error {
	if c.GPGRecipient != "" && strings.TrimSpace(c.GPGRecipient) == "" {
		return fmt.Errorf("gpg_recipient must not be blank")
	}
	for _, recipient := range c.GPGRecipients {
		if strings.TrimSpace(recipient) == "" {
			return fmt.Errorf("gpg_recipients must not contain blank recipients")
		}
	}
	if len(c.Recipients()) == 0 {
		return fmt.Errorf("no GPG recipient configured; run: kubegonfig init")
	}
	return nil
}

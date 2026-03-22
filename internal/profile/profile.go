// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package profile manages kubeconfig profiles: encrypted storage, CRUD
// operations, and activation state.
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kubegonfig/internal/config"
	"kubegonfig/internal/crypto"
	"kubegonfig/internal/kubeconfig"
	"kubegonfig/internal/shell"
	"kubegonfig/internal/storage"
)

const (
	profilesDir = "profiles"
	stateDir    = "state"
	currentFile = "current"
	lockFile    = "kubegonfig.lock"
	gpgExt      = ".yaml.gpg"
)

// Manager provides profile CRUD and state operations.
type Manager struct {
	cfg     *config.Config
	dataDir string
}

// NewManager creates a Manager from the given config.
func NewManager(cfg *config.Config) (*Manager, error) {
	dataDir, err := cfg.ResolveDataDir()
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	return &Manager{cfg: cfg, dataDir: dataDir}, nil
}

func (m *Manager) profilesPath() string {
	return filepath.Join(m.dataDir, profilesDir)
}

func (m *Manager) statePath() string {
	return filepath.Join(m.dataDir, stateDir)
}

func (m *Manager) profilePath(name string) string {
	return filepath.Join(m.profilesPath(), name+gpgExt)
}

func (m *Manager) currentPath() string {
	return filepath.Join(m.statePath(), currentFile)
}

func (m *Manager) lockPath() string {
	return filepath.Join(m.dataDir, lockFile)
}

// withLock executes fn while holding an exclusive file lock.
func (m *Manager) withLock(fn func() error) error {
	lock, err := storage.NewFileLock(m.lockPath())
	if err != nil {
		return fmt.Errorf("create lock: %w", err)
	}
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Unlock()
	return fn()
}

// Create validates, encrypts, and stores a new profile.
func (m *Manager) Create(name string, data []byte) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}
	if err := kubeconfig.Validate(data); err != nil {
		return fmt.Errorf("invalid kubeconfig: %w", err)
	}
	if err := m.cfg.Validate(); err != nil {
		return err
	}

	return m.withLock(func() error {
		if m.Exists(name) {
			return fmt.Errorf("profile %q already exists", name)
		}

		encrypted, err := crypto.Encrypt(data, m.cfg.Recipients())
		if err != nil {
			return fmt.Errorf("encrypt profile: %w", err)
		}

		return storage.AtomicWrite(m.profilePath(name), encrypted, 0600)
	})
}

// Import reads a kubeconfig from a file path, validates, encrypts, and stores it.
func (m *Manager) Import(name, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file %s: %w", path, err)
	}
	return m.Create(name, data)
}

// List returns sorted profile names.
func (m *Manager) List() ([]string, error) {
	dir := m.profilesPath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), gpgExt) {
			continue
		}
		name := strings.TrimSuffix(e.Name(), gpgExt)
		names = append(names, name)
	}
	return names, nil
}

// Exists checks if a profile with the given name exists.
func (m *Manager) Exists(name string) bool {
	return storage.FileExists(m.profilePath(name))
}

// Decrypt reads and decrypts a profile, returning the plaintext kubeconfig.
func (m *Manager) Decrypt(name string) ([]byte, error) {
	if err := shell.ValidateName(name); err != nil {
		return nil, err
	}
	if !m.Exists(name) {
		return nil, fmt.Errorf("profile %q not found", name)
	}

	encrypted, err := storage.ReadFile(m.profilePath(name))
	if err != nil {
		return nil, err
	}

	data, err := crypto.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt profile %q: %w", name, err)
	}
	return data, nil
}

// Delete removes a profile. If it was active, clears the current state.
func (m *Manager) Delete(name string) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}

	return m.withLock(func() error {
		if !m.Exists(name) {
			return fmt.Errorf("profile %q not found", name)
		}

		// Clear current if this profile is active.
		current, _ := m.GetCurrent()
		if current == name {
			_ = storage.RemoveFile(m.currentPath())
		}

		return storage.RemoveFile(m.profilePath(name))
	})
}

// Rename renames a profile from oldName to newName.
func (m *Manager) Rename(oldName, newName string) error {
	if err := shell.ValidateName(oldName); err != nil {
		return fmt.Errorf("old name: %w", err)
	}
	if err := shell.ValidateName(newName); err != nil {
		return fmt.Errorf("new name: %w", err)
	}

	return m.withLock(func() error {
		if !m.Exists(oldName) {
			return fmt.Errorf("profile %q not found", oldName)
		}
		if m.Exists(newName) {
			return fmt.Errorf("profile %q already exists", newName)
		}

		oldPath := m.profilePath(oldName)
		newPath := m.profilePath(newName)

		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("rename: %w", err)
		}

		// Update current pointer if needed.
		current, _ := m.GetCurrent()
		if current == oldName {
			return m.setCurrent(newName)
		}
		return nil
	})
}

// SetCurrent records the active profile name.
func (m *Manager) SetCurrent(name string) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}
	if !m.Exists(name) {
		return fmt.Errorf("profile %q not found", name)
	}
	return m.withLock(func() error {
		return m.setCurrent(name)
	})
}

func (m *Manager) setCurrent(name string) error {
	return storage.AtomicWrite(m.currentPath(), []byte(name+"\n"), 0600)
}

// GetCurrent returns the name of the currently active profile.
// Returns empty string and nil error if no profile is active.
func (m *Manager) GetCurrent() (string, error) {
	data, err := storage.ReadFile(m.currentPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// Update re-encrypts a profile with new data (for edit workflow).
func (m *Manager) Update(name string, data []byte) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}
	if err := kubeconfig.Validate(data); err != nil {
		return fmt.Errorf("invalid kubeconfig: %w", err)
	}
	if err := m.cfg.Validate(); err != nil {
		return err
	}

	return m.withLock(func() error {
		if !m.Exists(name) {
			return fmt.Errorf("profile %q not found", name)
		}

		encrypted, err := crypto.Encrypt(data, m.cfg.Recipients())
		if err != nil {
			return fmt.Errorf("encrypt profile: %w", err)
		}

		return storage.AtomicWrite(m.profilePath(name), encrypted, 0600)
	})
}

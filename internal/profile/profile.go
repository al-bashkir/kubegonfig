// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package profile manages kubeconfig profiles: encrypted storage, CRUD
// operations, and activation state.
package profile

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func profileFileName(name string) string {
	return name + gpgExt
}

func (m *Manager) lockPath() string {
	return filepath.Join(m.dataDir, lockFile)
}

func (m *Manager) writeProfile(name string, data []byte) error {
	return storage.AtomicWriteInDir(m.profilesPath(), profileFileName(name), data, 0600)
}

func (m *Manager) readProfile(name string) ([]byte, error) {
	return storage.ReadFileInDir(m.profilesPath(), profileFileName(name))
}

func (m *Manager) writeCurrent(name string) error {
	return storage.AtomicWriteInDir(m.statePath(), currentFile, []byte(name+"\n"), 0600)
}

func (m *Manager) readCurrent() ([]byte, error) {
	return storage.ReadFileInDir(m.statePath(), currentFile)
}

// WithLock executes fn while holding an exclusive file lock.
func (m *Manager) WithLock(fn func() error) (err error) {
	lock, err := storage.NewFileLock(m.lockPath())
	if err != nil {
		return fmt.Errorf("create lock: %w", err)
	}
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			err = errors.Join(err, fmt.Errorf("release lock: %w", unlockErr))
		}
	}()

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

	return m.WithLock(func() error {
		exists, err := m.profileExists(name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("profile %q already exists", name)
		}

		encrypted, err := crypto.Encrypt(data, m.cfg.Recipients())
		if err != nil {
			return fmt.Errorf("encrypt profile: %w", err)
		}

		return m.writeProfile(name, encrypted)
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
	entries, err := storage.ReadDirInDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), gpgExt) {
			continue
		}
		name := strings.TrimSuffix(e.Name(), gpgExt)
		if err := shell.ValidateName(name); err != nil {
			continue
		}
		exists, err := storage.RegularFileInDir(dir, e.Name())
		if err != nil {
			return nil, fmt.Errorf("check profile %q: %w", name, err)
		}
		if !exists {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// Decrypt reads and decrypts a profile, returning the plaintext kubeconfig.
func (m *Manager) Decrypt(name string) ([]byte, error) {
	if err := shell.ValidateName(name); err != nil {
		return nil, err
	}
	return m.decryptProfile(name)
}

func (m *Manager) decryptProfile(name string) ([]byte, error) {
	encrypted, err := m.readProfile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("profile %q not found", name)
		}
		return nil, err
	}

	data, err := crypto.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt profile %q: %w", name, err)
	}
	return data, nil
}

// Edit decrypts a profile, passes it to edit while holding the profile lock,
// and stores the edited content if it changed.
func (m *Manager) Edit(name string, edit func([]byte) ([]byte, error)) (changed bool, err error) {
	if err := shell.ValidateName(name); err != nil {
		return false, err
	}
	if edit == nil {
		return false, fmt.Errorf("edit callback must not be nil")
	}

	err = m.WithLock(func() error {
		original, err := m.decryptProfile(name)
		if err != nil {
			return err
		}

		edited, err := edit(original)
		if err != nil {
			return err
		}
		if bytes.Equal(original, edited) {
			return nil
		}

		if err := kubeconfig.Validate(edited); err != nil {
			return fmt.Errorf("invalid kubeconfig: %w", err)
		}
		if err := m.cfg.Validate(); err != nil {
			return err
		}

		encrypted, err := crypto.Encrypt(edited, m.cfg.Recipients())
		if err != nil {
			return fmt.Errorf("encrypt profile: %w", err)
		}
		if err := m.writeProfile(name, encrypted); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

// Activate decrypts a profile, creates its activation file through create, and
// records it as current while holding the profile lock.
func (m *Manager) Activate(name string, create func([]byte) (string, error), cleanup func(string) error) (path string, err error) {
	if err := shell.ValidateName(name); err != nil {
		return "", err
	}
	if create == nil {
		return "", fmt.Errorf("activation callback must not be nil")
	}

	err = m.WithLock(func() error {
		data, err := m.decryptProfile(name)
		if err != nil {
			return err
		}

		path, err = create(data)
		if err != nil {
			return err
		}

		if err := m.setCurrent(name); err != nil {
			if cleanup != nil {
				if cleanupErr := cleanup(path); cleanupErr != nil {
					return errors.Join(err, fmt.Errorf("cleanup activation file: %w", cleanupErr))
				}
			}
			return err
		}
		return nil
	})
	return path, err
}

// Unlock decrypts a profile and writes its activation file via create,
// while holding the profile lock. Unlike Activate, it does not record
// the profile as current.
func (m *Manager) Unlock(name string, create func([]byte) (string, error)) (path string, err error) {
	if err := shell.ValidateName(name); err != nil {
		return "", err
	}
	if create == nil {
		return "", fmt.Errorf("activation callback must not be nil")
	}

	err = m.WithLock(func() error {
		data, err := m.decryptProfile(name)
		if err != nil {
			return err
		}
		path, err = create(data)
		return err
	})
	return path, err
}

// Delete removes a profile. If it was active, clears the current state.
func (m *Manager) Delete(name string) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}

	return m.WithLock(func() error {
		exists, err := m.profileExists(name)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("profile %q not found", name)
		}

		// Clear current if this profile is active.
		current, err := m.GetCurrent()
		if err != nil {
			return fmt.Errorf("read current profile: %w", err)
		}
		currentCleared := false
		if current == name {
			if err := storage.RemoveFileInDir(m.statePath(), currentFile); err != nil {
				if storage.IsPostCommitError(err) {
					if restoreErr := m.setCurrent(name); restoreErr != nil {
						return errors.Join(
							fmt.Errorf("clear current profile: %w", err),
							fmt.Errorf("restore current profile: %w", restoreErr),
						)
					}
				}
				return fmt.Errorf("clear current profile: %w", err)
			}
			currentCleared = true
		}

		if err := storage.RemoveFileInDir(m.profilesPath(), profileFileName(name)); err != nil {
			if currentCleared {
				exists, existsErr := m.profileExists(name)
				if existsErr != nil {
					return errors.Join(
						fmt.Errorf("delete profile: %w", err),
						fmt.Errorf("check profile after failed delete: %w", existsErr),
					)
				}
				if exists {
					if restoreErr := m.setCurrent(name); restoreErr != nil {
						return errors.Join(
							fmt.Errorf("delete profile: %w", err),
							fmt.Errorf("restore current profile: %w", restoreErr),
						)
					}
				} else {
					return errors.Join(
						fmt.Errorf("delete profile: %w", err),
						fmt.Errorf("profile %q may have been removed before the delete error was reported", name),
					)
				}
			}
			return fmt.Errorf("delete profile: %w", err)
		}
		return nil
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

	return m.WithLock(func() error {
		exists, err := m.profileExists(oldName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("profile %q not found", oldName)
		}
		exists, err = m.profileExists(newName)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("profile %q already exists", newName)
		}
		current, err := m.GetCurrent()
		if err != nil {
			return fmt.Errorf("read current profile: %w", err)
		}

		renameErr := storage.RenameFileInDir(m.profilesPath(), profileFileName(oldName), profileFileName(newName))
		if renameErr != nil && !storage.IsPostCommitError(renameErr) {
			return fmt.Errorf("rename: %w", renameErr)
		}

		// Update current pointer if needed.
		if current == oldName {
			if err := m.setCurrent(newName); err != nil {
				if renameErr != nil {
					return errors.Join(fmt.Errorf("rename: %w", renameErr), m.rollbackRenameAfterCurrentFailure(oldName, newName, err))
				}
				return m.rollbackRenameAfterCurrentFailure(oldName, newName, err)
			}
		}
		if renameErr != nil {
			return fmt.Errorf("rename: %w", renameErr)
		}
		return nil
	})
}

func (m *Manager) rollbackRenameAfterCurrentFailure(oldName, newName string, updateErr error) error {
	errs := []error{fmt.Errorf("update current profile: %w", updateErr)}
	rolledBack := true
	if rollbackErr := storage.RenameFileInDir(m.profilesPath(), profileFileName(newName), profileFileName(oldName)); rollbackErr != nil {
		rolledBack = false
		errs = append(errs, fmt.Errorf("rollback rename: %w", rollbackErr))
	}
	if !rolledBack {
		exists, existsErr := m.profileExists(oldName)
		if existsErr != nil {
			errs = append(errs, fmt.Errorf("check old profile after failed rollback: %w", existsErr))
		}
		rolledBack = exists
	}
	current, err := m.GetCurrent()
	if err != nil {
		errs = append(errs, fmt.Errorf("read current after failed update: %w", err))
	} else if current != oldName && rolledBack {
		if restoreErr := m.setCurrent(oldName); restoreErr != nil {
			errs = append(errs, fmt.Errorf("restore current profile: %w", restoreErr))
		}
	} else if current != oldName {
		errs = append(errs, fmt.Errorf("current profile may still point to %q because profile rollback did not restore %q", current, oldName))
	}
	return errors.Join(errs...)
}

func (m *Manager) profileExists(name string) (bool, error) {
	return storage.FileExistsInDir(m.profilesPath(), profileFileName(name))
}

// Exists reports whether a profile with the given name is stored locally.
func (m *Manager) Exists(name string) (bool, error) {
	if err := shell.ValidateName(name); err != nil {
		return false, err
	}
	return m.profileExists(name)
}

func (m *Manager) setCurrent(name string) error {
	return m.writeCurrent(name)
}

// GetCurrent returns the name of the currently active profile.
// Returns empty string and nil error if no profile is active.
func (m *Manager) GetCurrent() (string, error) {
	data, err := m.readCurrent()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", nil
	}
	if err := shell.ValidateName(name); err != nil {
		return "", fmt.Errorf("invalid current profile state: %w", err)
	}
	return name, nil
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"kubegonfig/internal/config"
	"kubegonfig/internal/crypto"
	"kubegonfig/internal/kubeconfig"
	"kubegonfig/internal/profile"

	"gopkg.in/yaml.v3"
)

// RestoreOptions configures a restore.
type RestoreOptions struct {
	In           io.Reader
	Force        bool
	SkipExisting bool
	MergeConfig  bool
	ProfilesOnly bool
	DryRun       bool
}

// RestorePlan summarizes what Restore did (or would do, with DryRun).
type RestorePlan struct {
	SchemaVersion int
	CreatedAt     time.Time
	SourceVersion string
	Encrypted     bool
	ToCreate      []string
	ToOverwrite   []string
	ToSkip        []string
	ConfigAction  string // "ignore" | "merge" | "skip-profiles-only"
}

// Restore reads a tar archive from opts.In and applies it to mgr/cfg.
func Restore(mgr *profile.Manager, cfg *config.Config, opts RestoreOptions) (RestorePlan, error) {
	plan := RestorePlan{}
	if mgr == nil {
		return plan, fmt.Errorf("Restore: nil manager")
	}
	if opts.In == nil {
		return plan, fmt.Errorf("Restore: nil input reader")
	}
	if opts.Force && opts.SkipExisting {
		return plan, fmt.Errorf("Restore: --force and --skip-existing are mutually exclusive")
	}

	tmp, err := os.CreateTemp("", "kubegonfig-restore-*")
	if err != nil {
		return plan, fmt.Errorf("create restore tempfile: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	if _, err := io.Copy(tmp, opts.In); err != nil {
		return plan, fmt.Errorf("buffer archive: %w", err)
	}

	manifest, err := readManifest(tmp)
	if err != nil {
		return plan, err
	}
	plan.SchemaVersion = manifest.SchemaVersion
	plan.CreatedAt = manifest.CreatedAt
	plan.SourceVersion = manifest.KubegonfigVersion
	plan.Encrypted = manifest.Encrypted

	wantedFiles := make(map[string]string, len(manifest.Profiles))
	for _, p := range manifest.Profiles {
		wantedFiles[p.File] = p.Name
	}

	err = mgr.WithLock(func() error {
		var collisions []string
		for _, p := range manifest.Profiles {
			exists, err := mgr.Exists(p.Name)
			if err != nil {
				return fmt.Errorf("check profile %q: %w", p.Name, err)
			}
			switch {
			case !exists:
				plan.ToCreate = append(plan.ToCreate, p.Name)
			case opts.Force:
				plan.ToOverwrite = append(plan.ToOverwrite, p.Name)
			case opts.SkipExisting:
				plan.ToSkip = append(plan.ToSkip, p.Name)
			default:
				collisions = append(collisions, p.Name)
			}
		}
		if len(collisions) > 0 {
			sort.Strings(collisions)
			return fmt.Errorf("restore: %d profile(s) already exist: %s; pass --force to overwrite or --skip-existing to keep local copies",
				len(collisions), strings.Join(collisions, ", "))
		}

		switch {
		case opts.ProfilesOnly:
			plan.ConfigAction = "skip-profiles-only"
		case opts.MergeConfig && manifest.ConfigIncluded:
			plan.ConfigAction = "merge"
		default:
			plan.ConfigAction = "ignore"
		}

		if opts.DryRun {
			return nil
		}

		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("rewind archive for apply: %w", err)
		}
		tr := tar.NewReader(tmp)
		if _, err := tr.Next(); err != nil {
			return fmt.Errorf("re-read manifest entry: %w", err)
		}

		skip := make(map[string]struct{}, len(plan.ToSkip))
		for _, n := range plan.ToSkip {
			skip[n] = struct{}{}
		}

		var writeErrs []error
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("read archive entry: %w", err)
			}
			if hdr.Typeflag == tar.TypeDir {
				continue
			}
			if hdr.Typeflag != tar.TypeReg {
				return fmt.Errorf("archive entry %q has disallowed type %d", hdr.Name, hdr.Typeflag)
			}
			if err := validateEntryPath(hdr.Name); err != nil {
				return err
			}
			switch {
			case hdr.Name == ConfigName:
				if plan.ConfigAction != "merge" {
					if _, err := io.Copy(io.Discard, tr); err != nil {
						return fmt.Errorf("skip config body: %w", err)
					}
					continue
				}
				body, err := io.ReadAll(io.LimitReader(tr, MaxProfileSize+1))
				if err != nil {
					writeErrs = append(writeErrs, fmt.Errorf("read config.yaml: %w", err))
					continue
				}
				if err := mergeArchivedConfig(cfg, body); err != nil {
					writeErrs = append(writeErrs, fmt.Errorf("merge config: %w", err))
				}
			case strings.HasPrefix(hdr.Name, ProfilesDir):
				name, ok := wantedFiles[hdr.Name]
				if !ok {
					return fmt.Errorf("archive entry %q is not listed in manifest", hdr.Name)
				}
				if _, skipIt := skip[name]; skipIt {
					if _, err := io.Copy(io.Discard, tr); err != nil {
						return fmt.Errorf("skip body for %q: %w", name, err)
					}
					continue
				}
				if hdr.Size > int64(MaxProfileSize) {
					return fmt.Errorf("archive entry %q exceeds %d byte limit", hdr.Name, MaxProfileSize)
				}
				body, err := io.ReadAll(io.LimitReader(tr, MaxProfileSize+1))
				if err != nil {
					writeErrs = append(writeErrs, fmt.Errorf("read entry %q: %w", hdr.Name, err))
					continue
				}
				if int64(len(body)) > int64(MaxProfileSize) {
					return fmt.Errorf("archive entry %q exceeds %d byte limit", hdr.Name, MaxProfileSize)
				}
				blob, err := prepareProfileBlob(cfg, manifest.Encrypted, body)
				if err != nil {
					writeErrs = append(writeErrs, fmt.Errorf("profile %q: %w", name, err))
					continue
				}
				if err := mgr.WriteEncrypted(name, blob); err != nil {
					writeErrs = append(writeErrs, fmt.Errorf("profile %q: %w", name, err))
					continue
				}
			default:
				return fmt.Errorf("archive entry %q is not allowed", hdr.Name)
			}
		}
		return errors.Join(writeErrs...)
	})
	return plan, err
}

func readManifest(rs io.ReadSeeker) (*Manifest, error) {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind archive: %w", err)
	}
	tr := tar.NewReader(rs)
	hdr, err := tr.Next()
	if err != nil {
		return nil, fmt.Errorf("read first archive entry: %w", err)
	}
	if hdr.Name != ManifestName {
		return nil, fmt.Errorf("archive does not start with %q (got %q)", ManifestName, hdr.Name)
	}
	if hdr.Size > 64*1024 {
		return nil, fmt.Errorf("manifest entry too large: %d bytes", hdr.Size)
	}
	body, err := io.ReadAll(io.LimitReader(tr, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("read manifest body: %w", err)
	}
	return ParseManifest(body)
}

func validateEntryPath(name string) error {
	if name == "" {
		return fmt.Errorf("archive entry has empty name")
	}
	if path.IsAbs(name) {
		return fmt.Errorf("archive entry %q is absolute", name)
	}
	clean := path.Clean(name)
	if clean != name || strings.Contains(name, "..") {
		return fmt.Errorf("archive entry %q contains traversal", name)
	}
	return nil
}

// prepareProfileBlob returns the bytes to write into the local store. For
// encrypted archives the body is passed through verbatim. For plaintext
// archives the body is validated as a kubeconfig and re-encrypted with the
// LOCAL recipients.
func prepareProfileBlob(cfg *config.Config, encrypted bool, body []byte) ([]byte, error) {
	if encrypted {
		return body, nil
	}
	if err := kubeconfig.Validate(body); err != nil {
		return nil, fmt.Errorf("invalid kubeconfig: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	blob, err := crypto.Encrypt(body, cfg.Recipients())
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	return blob, nil
}

// mergeArchivedConfig unions gpg_recipients from the archived config into the
// local config. gpg_recipient (primary), data_dir, and shell_style are never
// overwritten.
func mergeArchivedConfig(local *config.Config, archivedBody []byte) error {
	var archived struct {
		GPGRecipients []string `yaml:"gpg_recipients"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(archivedBody))
	dec.KnownFields(false)
	if err := dec.Decode(&archived); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("parse archived config: %w", err)
	}
	seen := make(map[string]struct{}, len(local.GPGRecipients))
	for _, r := range local.GPGRecipients {
		seen[strings.TrimSpace(r)] = struct{}{}
	}
	for _, r := range archived.GPGRecipients {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		local.GPGRecipients = append(local.GPGRecipients, r)
	}
	return local.Save()
}

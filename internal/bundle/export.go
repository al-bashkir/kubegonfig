// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"archive/tar"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"time"

	"kubegonfig/internal/config"
	"kubegonfig/internal/kubeconfig"
	"kubegonfig/internal/profile"
	"kubegonfig/internal/shell"
	"kubegonfig/internal/storage"
)

// ExportOptions configures an archive export.
type ExportOptions struct {
	// Names is the list of profile names to include. The cmd layer
	// validates, deduplicates, and (optionally) populates this from List().
	Names []string

	// Decrypt emits plaintext kubeconfig entries instead of pass-through
	// ciphertext. The cmd layer is responsible for plaintext safety guards.
	Decrypt bool

	// IncludeConfig controls whether config.yaml is added to the archive.
	IncludeConfig bool

	// KubegonfigVersion is recorded in the manifest. The bundle package
	// cannot import main, so the cmd layer injects this.
	KubegonfigVersion string

	// Out is where the tar stream is written.
	Out io.Writer
}

// Export writes a tar archive containing the requested profiles (and
// optionally config.yaml) to opts.Out. Holds the profile lock for the full
// snapshot duration.
func Export(mgr *profile.Manager, cfg *config.Config, opts ExportOptions) error {
	if mgr == nil {
		return fmt.Errorf("Export: nil manager")
	}
	if opts.Out == nil {
		return fmt.Errorf("Export: nil output writer")
	}
	if len(opts.Names) == 0 {
		return fmt.Errorf("Export: no profiles selected")
	}

	names := dedupeAndSort(opts.Names)
	for _, name := range names {
		if err := shell.ValidateName(name); err != nil {
			return fmt.Errorf("export %q: %w", name, err)
		}
	}

	return mgr.WithLock(func() error {
		now := time.Now().UTC().Truncate(time.Second)
		ext := EncryptedExt
		if opts.Decrypt {
			ext = PlaintextExt
		}

		manifest := &Manifest{
			SchemaVersion:     SchemaVersion,
			CreatedAt:         now,
			KubegonfigVersion: opts.KubegonfigVersion,
			Encrypted:         !opts.Decrypt,
			ProfileCount:      len(names),
			Profiles:          make([]ManifestProfile, 0, len(names)),
			ConfigIncluded:    opts.IncludeConfig,
		}

		bodies := make(map[string][]byte, len(names))
		for _, name := range names {
			data, err := readProfileForExport(mgr, name, opts.Decrypt)
			if err != nil {
				return err
			}
			bodies[name] = data
			manifest.Profiles = append(manifest.Profiles, ManifestProfile{
				Name: name,
				File: ProfilesDir + name + ext,
			})
		}

		manifestBody, err := MarshalManifest(manifest)
		if err != nil {
			return fmt.Errorf("marshal manifest: %w", err)
		}

		tw := tar.NewWriter(opts.Out)
		if err := writeTarEntry(tw, ManifestName, manifestBody, now); err != nil {
			return err
		}
		for _, name := range names {
			if err := writeTarEntry(tw, ProfilesDir+name+ext, bodies[name], now); err != nil {
				return err
			}
		}
		if opts.IncludeConfig {
			configBody, err := readConfigBody(cfg)
			if err != nil {
				return fmt.Errorf("read config.yaml: %w", err)
			}
			if err := writeTarEntry(tw, ConfigName, configBody, now); err != nil {
				return err
			}
		}
		return tw.Close()
	})
}

func readProfileForExport(mgr *profile.Manager, name string, decrypt bool) ([]byte, error) {
	if !decrypt {
		blob, err := mgr.ReadEncrypted(name)
		if err != nil {
			return nil, fmt.Errorf("export %q: %w", name, err)
		}
		return blob, nil
	}
	plain, err := mgr.Decrypt(name)
	if err != nil {
		return nil, fmt.Errorf("export %q: %w", name, err)
	}
	if err := kubeconfig.Validate(plain); err != nil {
		return nil, fmt.Errorf("export %q: invalid plaintext kubeconfig: %w", name, err)
	}
	return plain, nil
}

func writeTarEntry(tw *tar.Writer, name string, body []byte, mtime time.Time) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0600,
		Size:    int64(len(body)),
		ModTime: mtime,
		Uid:     0,
		Gid:     0,
		Uname:   "",
		Gname:   "",
		Format:  tar.FormatUSTAR,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header for %q: %w", name, err)
	}
	if _, err := tw.Write(body); err != nil {
		return fmt.Errorf("write tar body for %q: %w", name, err)
	}
	return nil
}

func dedupeAndSort(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, n := range in {
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func readConfigBody(cfg *config.Config) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	path := cfg.Path()
	if path == "" {
		return nil, fmt.Errorf("config path is not set; load config before export")
	}
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	return storage.ReadFileInDir(dir, name)
}

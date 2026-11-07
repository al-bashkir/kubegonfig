// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"kubegonfig/internal/config"
	"kubegonfig/internal/crypto"
	"kubegonfig/internal/kubeconfig"
	"kubegonfig/internal/profile"
	"kubegonfig/internal/storage"

	"gopkg.in/yaml.v3"
)

// RestoreOptions configures a restore.
type RestoreOptions struct {
	In           io.Reader
	Force        bool
	SkipExisting bool
	MergeConfig  bool
	DryRun       bool
}

// RestorePlan summarizes what Restore did (or would do, with DryRun).
type RestorePlan struct {
	ToCreate     []string
	ToOverwrite  []string
	ToSkip       []string
	ConfigAction string // "ignore" | "merge"
}

// Restore reads a tar archive from opts.In and applies it to mgr/cfg.
func Restore(mgr *profile.Manager, cfg *config.Config, opts RestoreOptions) (RestorePlan, error) {
	plan := RestorePlan{}
	if opts.Force && opts.SkipExisting {
		return plan, fmt.Errorf("--force and --skip-existing are mutually exclusive")
	}

	runtimeDir := storage.RuntimeDir()
	if err := storage.EnsureDir(runtimeDir, 0700); err != nil {
		return plan, fmt.Errorf("create runtime dir: %w", err)
	}
	tmp, err := os.CreateTemp(runtimeDir, "kubegonfig-restore-*")
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

	err = mgr.WithLock(func() error {
		var err error
		plan, err = planRestore(mgr, manifest, opts)
		if err != nil || opts.DryRun {
			return err
		}
		return applyRestore(mgr, cfg, tmp, manifest, plan)
	})
	return plan, err
}

// planRestore classifies every manifest profile against the local store and
// decides the config action. Must run under the profile lock.
func planRestore(mgr *profile.Manager, manifest *Manifest, opts RestoreOptions) (RestorePlan, error) {
	plan := RestorePlan{}
	var collisions []string
	for _, p := range manifest.Profiles {
		exists, err := mgr.Exists(p.Name)
		if err != nil {
			return plan, fmt.Errorf("check profile %q: %w", p.Name, err)
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
		return plan, fmt.Errorf("restore: %d profile(s) already exist: %s; pass --force to overwrite or --skip-existing to keep local copies",
			len(collisions), strings.Join(collisions, ", "))
	}

	plan.ConfigAction = "ignore"
	if wantsConfigMerge(opts, manifest) {
		plan.ConfigAction = "merge"
	}
	return plan, nil
}

func wantsConfigMerge(opts RestoreOptions, manifest *Manifest) bool {
	return opts.MergeConfig && manifest.ConfigIncluded
}

// restorer holds the state of one apply pass over the archive.
type restorer struct {
	mgr      *profile.Manager
	cfg      *config.Config
	manifest *Manifest
	plan     RestorePlan
	wanted   map[string]string   // archive file -> profile name
	skip     map[string]struct{} // profile names to keep local
	// writeErrs collects per-entry failures so one bad entry does not stop the rest.
	writeErrs []error
}

// applyRestore replays the buffered archive (past the manifest) into mgr/cfg.
func applyRestore(mgr *profile.Manager, cfg *config.Config, tmp io.ReadSeeker, manifest *Manifest, plan RestorePlan) error {
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind archive for apply: %w", err)
	}
	tr := tar.NewReader(tmp)
	if _, err := tr.Next(); err != nil {
		return fmt.Errorf("re-read manifest entry: %w", err)
	}

	r := newRestorer(mgr, cfg, manifest, plan)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive entry: %w", err)
		}
		if err := r.entry(tr, hdr); err != nil {
			return err
		}
	}
	return errors.Join(r.writeErrs...)
}

func newRestorer(mgr *profile.Manager, cfg *config.Config, manifest *Manifest, plan RestorePlan) *restorer {
	r := &restorer{
		mgr: mgr, cfg: cfg, manifest: manifest, plan: plan,
		wanted: make(map[string]string, len(manifest.Profiles)),
		skip:   make(map[string]struct{}, len(plan.ToSkip)),
	}
	for _, p := range manifest.Profiles {
		r.wanted[p.File] = p.Name
	}
	for _, n := range plan.ToSkip {
		r.skip[n] = struct{}{}
	}
	return r
}

// entry dispatches one archive entry. A non-nil return aborts the restore.
func (r *restorer) entry(tr *tar.Reader, hdr *tar.Header) error {
	if hdr.Typeflag != tar.TypeReg {
		return fmt.Errorf("archive entry %q has disallowed type %d", hdr.Name, hdr.Typeflag)
	}
	// Entry names must match ConfigName or a manifest-listed file exactly;
	// that allowlist rejects absolute and traversal paths.
	switch {
	case hdr.Name == ConfigName:
		return r.configEntry(tr, hdr)
	case strings.HasPrefix(hdr.Name, ProfilesDir):
		return r.profileEntry(tr, hdr)
	default:
		return fmt.Errorf("archive entry %q is not allowed", hdr.Name)
	}
}

// configEntry merges the archived config when the plan asks for it. A non-nil
// return aborts the restore; recoverable failures go to r.writeErrs.
func (r *restorer) configEntry(tr *tar.Reader, hdr *tar.Header) error {
	if r.plan.ConfigAction != "merge" {
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return fmt.Errorf("skip config body: %w", err)
		}
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(tr, MaxProfileSize+1))
	if err != nil {
		r.writeErrs = append(r.writeErrs, fmt.Errorf("read config.yaml: %w", err))
		return nil
	}
	if int64(len(body)) > int64(MaxProfileSize) {
		return fmt.Errorf("archive entry %q exceeds %d byte limit", hdr.Name, MaxProfileSize)
	}
	if err := mergeArchivedConfig(r.cfg, body); err != nil {
		r.writeErrs = append(r.writeErrs, fmt.Errorf("merge config: %w", err))
	}
	return nil
}

// profileEntry stores one archived profile unless the plan skips it.
func (r *restorer) profileEntry(tr *tar.Reader, hdr *tar.Header) error {
	name, ok := r.wanted[hdr.Name]
	if !ok {
		return fmt.Errorf("archive entry %q is not listed in manifest", hdr.Name)
	}
	if _, skipIt := r.skip[name]; skipIt {
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return fmt.Errorf("skip body for %q: %w", name, err)
		}
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(tr, MaxProfileSize+1))
	if err != nil {
		r.writeErrs = append(r.writeErrs, fmt.Errorf("read entry %q: %w", hdr.Name, err))
		return nil
	}
	if int64(len(body)) > int64(MaxProfileSize) {
		return fmt.Errorf("archive entry %q exceeds %d byte limit", hdr.Name, MaxProfileSize)
	}
	blob, err := prepareProfileBlob(r.cfg, r.manifest.Encrypted, body)
	if err == nil {
		err = r.mgr.WriteEncrypted(name, blob)
	}
	if err != nil {
		r.writeErrs = append(r.writeErrs, fmt.Errorf("profile %q: %w", name, err))
	}
	return nil
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
	if err := yaml.Unmarshal(archivedBody, &archived); err != nil {
		return fmt.Errorf("parse archived config: %w", err)
	}
	for _, r := range archived.GPGRecipients {
		if r = strings.TrimSpace(r); r != "" && !slices.Contains(local.GPGRecipients, r) {
			local.GPGRecipients = append(local.GPGRecipients, r)
		}
	}
	return local.Save()
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"

	"gopkg.in/yaml.v3"
)

// newSeededManager builds a profile.Manager whose store contains the given
// ciphertext blobs keyed by name.
func newSeededManager(t *testing.T, blobs map[string][]byte) *profile.Manager {
	t.Helper()
	dataDir := t.TempDir()
	cfg := &config.Config{DataDir: dataDir, GPGRecipient: "test@example.com"}
	mgr, err := profile.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}
	for name, blob := range blobs {
		if err := mgr.WriteEncrypted(name, blob); err != nil {
			t.Fatalf("WriteEncrypted(%q) error: %v", name, err)
		}
	}
	return mgr
}

func readTarEntries(t *testing.T, archive []byte) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read tar body for %q: %v", hdr.Name, err)
		}
		out[hdr.Name] = body
	}
	return out
}

func TestManifest_RoundTrip(t *testing.T) {
	want := Manifest{
		SchemaVersion:     1,
		CreatedAt:         time.Date(2026, 5, 5, 14, 23, 1, 0, time.UTC),
		KubegonfigVersion: "0.2.0",
		Encrypted:         true,
		ProfileCount:      2,
		Profiles: []ManifestProfile{
			{Name: "production", File: "profiles/production.yaml.gpg"},
			{Name: "staging", File: "profiles/staging.yaml.gpg"},
		},
		ConfigIncluded: true,
	}

	encoded, err := yaml.Marshal(&want)
	if err != nil {
		t.Fatalf("yaml.Marshal() error: %v", err)
	}

	got, err := ParseManifest(encoded)
	if err != nil {
		t.Fatalf("ParseManifest() error: %v", err)
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", *got, want)
	}
}

func TestParseManifest_RejectsFutureSchemaVersion(t *testing.T) {
	src := []byte("schema_version: 999\ncreated_at: 2026-05-05T14:23:01Z\nkubegonfig_version: 0.2.0\nencrypted: true\nprofile_count: 0\nprofiles: []\nconfig_included: false\n")
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want unsupported schema error")
	}
}

func TestParseManifest_RejectsZeroSchemaVersion(t *testing.T) {
	src := []byte("schema_version: 0\ncreated_at: 2026-05-05T14:23:01Z\nkubegonfig_version: 0.2.0\nencrypted: true\nprofile_count: 0\nprofiles: []\nconfig_included: false\n")
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want invalid schema version error")
	}
}

func TestParseManifest_RejectsCountMismatch(t *testing.T) {
	src := []byte(`schema_version: 1
created_at: 2026-05-05T14:23:01Z
kubegonfig_version: 0.2.0
encrypted: true
profile_count: 5
profiles:
  - name: a
    file: profiles/a.yaml.gpg
config_included: false
`)
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want count mismatch error")
	}
}

func TestParseManifest_RejectsInvalidProfileName(t *testing.T) {
	src := []byte(`schema_version: 1
created_at: 2026-05-05T14:23:01Z
kubegonfig_version: 0.2.0
encrypted: true
profile_count: 1
profiles:
  - name: "../escape"
    file: profiles/escape.yaml.gpg
config_included: false
`)
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want invalid profile name error")
	}
}

func TestParseManifest_RejectsExtensionMismatch(t *testing.T) {
	src := []byte(`schema_version: 1
created_at: 2026-05-05T14:23:01Z
kubegonfig_version: 0.2.0
encrypted: true
profile_count: 1
profiles:
  - name: a
    file: profiles/a.yaml
config_included: false
`)
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want extension mismatch error")
	}
}

func TestParseManifest_RejectsUnknownField(t *testing.T) {
	src := []byte(`schema_version: 1
created_at: 2026-05-05T14:23:01Z
kubegonfig_version: 0.2.0
encrypted: true
profile_count: 0
profiles: []
config_included: false
extra_field: oops
`)
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want unknown field error")
	}
}

func TestExport_EncryptedPassThrough(t *testing.T) {
	mgr := newSeededManager(t, map[string][]byte{
		"alpha": []byte("alpha-cipher"),
		"beta":  []byte("beta-cipher"),
	})
	cfg := &config.Config{GPGRecipient: "test@example.com"}

	var buf bytes.Buffer
	err := Export(mgr, cfg, ExportOptions{
		Names:             []string{"alpha", "beta"},
		Decrypt:           false,
		IncludeConfig:     false,
		KubegonfigVersion: "0.2.0",
		Out:               &buf,
	})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	entries := readTarEntries(t, buf.Bytes())

	manifestBody, ok := entries[ManifestName]
	if !ok {
		t.Fatal("manifest entry missing from archive")
	}
	manifest, err := ParseManifest(manifestBody)
	if err != nil {
		t.Fatalf("ParseManifest(): %v", err)
	}
	if !manifest.Encrypted {
		t.Error("manifest.Encrypted = false, want true")
	}
	if manifest.ConfigIncluded {
		t.Error("manifest.ConfigIncluded = true, want false")
	}

	if got, want := entries[filepath.ToSlash(ProfilesDir+"alpha"+EncryptedExt)], []byte("alpha-cipher"); !bytes.Equal(got, want) {
		t.Errorf("alpha entry = %q, want %q", got, want)
	}
	if got, want := entries[filepath.ToSlash(ProfilesDir+"beta"+EncryptedExt)], []byte("beta-cipher"); !bytes.Equal(got, want) {
		t.Errorf("beta entry = %q, want %q", got, want)
	}
	if _, included := entries[ConfigName]; included {
		t.Error("config.yaml present in archive when IncludeConfig=false")
	}
}

func TestExport_ManifestIsFirstEntry(t *testing.T) {
	mgr := newSeededManager(t, map[string][]byte{"alpha": []byte("alpha-cipher")})
	cfg := &config.Config{GPGRecipient: "test@example.com"}

	var buf bytes.Buffer
	if err := Export(mgr, cfg, ExportOptions{
		Names: []string{"alpha"}, KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("tar.Next: %v", err)
	}
	if hdr.Name != ManifestName {
		t.Errorf("first entry = %q, want %q", hdr.Name, ManifestName)
	}
}

func TestExport_EntryMetadataReproducible(t *testing.T) {
	mgr := newSeededManager(t, map[string][]byte{"alpha": []byte("a")})
	cfg := &config.Config{GPGRecipient: "test@example.com"}

	var buf bytes.Buffer
	if err := Export(mgr, cfg, ExportOptions{
		Names: []string{"alpha"}, KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if hdr.Mode != 0600 {
			t.Errorf("entry %q mode = %o, want 0600", hdr.Name, hdr.Mode)
		}
		if hdr.Uid != 0 || hdr.Gid != 0 {
			t.Errorf("entry %q uid/gid = %d/%d, want 0/0", hdr.Name, hdr.Uid, hdr.Gid)
		}
		if hdr.Uname != "" || hdr.Gname != "" {
			t.Errorf("entry %q uname/gname = %q/%q, want empty", hdr.Name, hdr.Uname, hdr.Gname)
		}
	}
}

func TestExport_EntriesSortedByName(t *testing.T) {
	mgr := newSeededManager(t, map[string][]byte{
		"zulu":  []byte("z"),
		"alpha": []byte("a"),
		"mike":  []byte("m"),
	})
	cfg := &config.Config{GPGRecipient: "test@example.com"}

	var buf bytes.Buffer
	if err := Export(mgr, cfg, ExportOptions{
		Names: []string{"zulu", "alpha", "mike"}, KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if hdr.Name == ManifestName {
			continue
		}
		names = append(names, hdr.Name)
	}
	want := []string{
		ProfilesDir + "alpha" + EncryptedExt,
		ProfilesDir + "mike" + EncryptedExt,
		ProfilesDir + "zulu" + EncryptedExt,
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("entry order = %v, not sorted", names)
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("entries = %v, want %v", names, want)
	}
}

func TestExport_MissingProfile(t *testing.T) {
	mgr := newSeededManager(t, map[string][]byte{"alpha": []byte("a")})
	cfg := &config.Config{GPGRecipient: "test@example.com"}

	var buf bytes.Buffer
	err := Export(mgr, cfg, ExportOptions{
		Names: []string{"alpha", "ghost"}, KubegonfigVersion: "0.2.0", Out: &buf,
	})
	if err == nil {
		t.Fatal("Export() error = nil, want missing profile error")
	}
}

func TestExport_DecryptEmitsPlaintextValidatedKubeconfig(t *testing.T) {
	installFakeGPGForBundle(t)

	dataDir := t.TempDir()
	cfg := &config.Config{DataDir: dataDir, GPGRecipient: "test@example.com"}
	mgr, err := profile.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}
	if err := mgr.WriteEncrypted("alpha", validKubeconfigForBundle()); err != nil {
		t.Fatalf("WriteEncrypted(): %v", err)
	}

	var buf bytes.Buffer
	if err := Export(mgr, cfg, ExportOptions{
		Names: []string{"alpha"}, Decrypt: true,
		KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	entries := readTarEntries(t, buf.Bytes())
	manifest, err := ParseManifest(entries[ManifestName])
	if err != nil {
		t.Fatalf("ParseManifest(): %v", err)
	}
	if manifest.Encrypted {
		t.Error("manifest.Encrypted = true on --decrypt export, want false")
	}
	if _, ok := entries[ProfilesDir+"alpha"+PlaintextExt]; !ok {
		t.Errorf("plaintext entry missing; entries: %v", keysOf(entries))
	}
}

func TestExport_DecryptRejectsInvalidPlaintext(t *testing.T) {
	installFakeGPGForBundle(t)

	dataDir := t.TempDir()
	cfg := &config.Config{DataDir: dataDir, GPGRecipient: "test@example.com"}
	mgr, err := profile.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}
	if err := mgr.WriteEncrypted("alpha", []byte("garbage-not-yaml")); err != nil {
		t.Fatalf("WriteEncrypted(): %v", err)
	}

	var buf bytes.Buffer
	err = Export(mgr, cfg, ExportOptions{
		Names: []string{"alpha"}, Decrypt: true,
		KubegonfigVersion: "0.2.0", Out: &buf,
	})
	if err == nil {
		t.Fatal("Export() error = nil, want invalid plaintext kubeconfig error")
	}
}

func TestExport_IncludeConfig(t *testing.T) {
	dataDir := t.TempDir()
	configDir := t.TempDir()
	cfgBody := []byte("gpg_recipient: test@example.com\n")
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), cfgBody, 0600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	cfg := &config.Config{DataDir: dataDir, GPGRecipient: "test@example.com"}
	cfg.Path = filepath.Join(configDir, "config.yaml")

	mgr, err := profile.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}
	if err := mgr.WriteEncrypted("alpha", []byte("a")); err != nil {
		t.Fatalf("WriteEncrypted(): %v", err)
	}

	var buf bytes.Buffer
	if err := Export(mgr, cfg, ExportOptions{
		Names: []string{"alpha"}, IncludeConfig: true,
		KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	entries := readTarEntries(t, buf.Bytes())
	manifest, err := ParseManifest(entries[ManifestName])
	if err != nil {
		t.Fatalf("ParseManifest(): %v", err)
	}
	if !manifest.ConfigIncluded {
		t.Error("manifest.ConfigIncluded = false, want true")
	}
	if got, ok := entries[ConfigName]; !ok || !bytes.Equal(got, cfgBody) {
		t.Errorf("config.yaml entry = %q, want %q", got, cfgBody)
	}
}

func validKubeconfigForBundle() []byte {
	return []byte(`apiVersion: v1
kind: Config
clusters:
  - name: cluster
    cluster:
      server: https://example.com
users:
  - name: user
    user: {}
contexts:
  - name: context
    context:
      cluster: cluster
      user: user
current-context: context
`)
}

// installFakeGPGForBundle installs stub gpg/gpg2 binaries whose --decrypt and
// --encrypt are both `cat`. The on-disk "ciphertext" therefore equals the
// plaintext for testing purposes.
func installFakeGPGForBundle(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"gpg2", "gpg"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\ncat\n"), 0700); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestRestore_RoundTripEncrypted(t *testing.T) {
	src := newSeededManager(t, map[string][]byte{
		"alpha": []byte("alpha-cipher"),
		"beta":  []byte("beta-cipher"),
	})
	srcCfg := &config.Config{GPGRecipient: "test@example.com"}

	var buf bytes.Buffer
	if err := Export(src, srcCfg, ExportOptions{
		Names:             []string{"alpha", "beta"},
		KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dst := newSeededManager(t, nil)
	dstCfg := &config.Config{GPGRecipient: "test@example.com"}

	plan, err := Restore(dst, dstCfg, RestoreOptions{In: bytes.NewReader(buf.Bytes())})
	if err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if got, want := len(plan.ToCreate), 2; got != want {
		t.Errorf("plan.ToCreate len = %d, want %d", got, want)
	}

	for _, name := range []string{"alpha", "beta"} {
		got, err := dst.ReadEncrypted(name)
		if err != nil {
			t.Fatalf("ReadEncrypted(%q): %v", name, err)
		}
		if string(got) != name+"-cipher" {
			t.Errorf("restored %q = %q, want %q", name, got, name+"-cipher")
		}
	}
}

func TestRestore_DryRunDoesNotWrite(t *testing.T) {
	src := newSeededManager(t, map[string][]byte{"alpha": []byte("a")})
	srcCfg := &config.Config{GPGRecipient: "test@example.com"}
	var buf bytes.Buffer
	if err := Export(src, srcCfg, ExportOptions{
		Names: []string{"alpha"}, KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dst := newSeededManager(t, nil)
	plan, err := Restore(dst, &config.Config{GPGRecipient: "test@example.com"}, RestoreOptions{
		In: bytes.NewReader(buf.Bytes()), DryRun: true,
	})
	if err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if len(plan.ToCreate) != 1 || plan.ToCreate[0] != "alpha" {
		t.Errorf("plan.ToCreate = %v, want [alpha]", plan.ToCreate)
	}

	exists, err := dst.Exists("alpha")
	if err != nil {
		t.Fatalf("Exists(): %v", err)
	}
	if exists {
		t.Error("alpha was written despite DryRun=true")
	}
}

func TestRestore_CollisionWithoutFlagsFails(t *testing.T) {
	src := newSeededManager(t, map[string][]byte{
		"alpha": []byte("a-new"),
		"beta":  []byte("b-new"),
	})
	srcCfg := &config.Config{GPGRecipient: "test@example.com"}
	var buf bytes.Buffer
	if err := Export(src, srcCfg, ExportOptions{
		Names: []string{"alpha", "beta"}, KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dst := newSeededManager(t, map[string][]byte{
		"alpha": []byte("a-old"),
		"beta":  []byte("b-old"),
	})

	_, err := Restore(dst, &config.Config{GPGRecipient: "test@example.com"}, RestoreOptions{
		In: bytes.NewReader(buf.Bytes()),
	})
	if err == nil {
		t.Fatal("Restore() error = nil, want collision error")
	}
	if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Errorf("error %q must list ALL colliding names", err)
	}

	got, _ := dst.ReadEncrypted("alpha")
	if string(got) != "a-old" {
		t.Errorf("alpha overwritten despite collision error: got %q", got)
	}
}

func TestRestore_ForceOverwrites(t *testing.T) {
	src := newSeededManager(t, map[string][]byte{"alpha": []byte("a-new")})
	srcCfg := &config.Config{GPGRecipient: "test@example.com"}
	var buf bytes.Buffer
	if err := Export(src, srcCfg, ExportOptions{
		Names: []string{"alpha"}, KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dst := newSeededManager(t, map[string][]byte{"alpha": []byte("a-old")})
	plan, err := Restore(dst, &config.Config{GPGRecipient: "test@example.com"}, RestoreOptions{
		In: bytes.NewReader(buf.Bytes()), Force: true,
	})
	if err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if len(plan.ToOverwrite) != 1 || plan.ToOverwrite[0] != "alpha" {
		t.Errorf("plan.ToOverwrite = %v, want [alpha]", plan.ToOverwrite)
	}

	got, _ := dst.ReadEncrypted("alpha")
	if string(got) != "a-new" {
		t.Errorf("after Force restore, alpha = %q, want %q", got, "a-new")
	}
}

func TestRestore_SkipExistingKeepsLocal(t *testing.T) {
	src := newSeededManager(t, map[string][]byte{
		"alpha": []byte("a-new"), "beta": []byte("b-new"),
	})
	srcCfg := &config.Config{GPGRecipient: "test@example.com"}
	var buf bytes.Buffer
	if err := Export(src, srcCfg, ExportOptions{
		Names: []string{"alpha", "beta"}, KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dst := newSeededManager(t, map[string][]byte{"alpha": []byte("a-old")})
	plan, err := Restore(dst, &config.Config{GPGRecipient: "test@example.com"}, RestoreOptions{
		In: bytes.NewReader(buf.Bytes()), SkipExisting: true,
	})
	if err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if !reflect.DeepEqual(plan.ToSkip, []string{"alpha"}) {
		t.Errorf("plan.ToSkip = %v, want [alpha]", plan.ToSkip)
	}
	if !reflect.DeepEqual(plan.ToCreate, []string{"beta"}) {
		t.Errorf("plan.ToCreate = %v, want [beta]", plan.ToCreate)
	}

	got, _ := dst.ReadEncrypted("alpha")
	if string(got) != "a-old" {
		t.Errorf("alpha kept-local check failed; got %q", got)
	}
	got, _ = dst.ReadEncrypted("beta")
	if string(got) != "b-new" {
		t.Errorf("beta restore check failed; got %q", got)
	}
}

func TestRestore_ForceAndSkipExistingMutuallyExclusive(t *testing.T) {
	dst := newSeededManager(t, nil)
	_, err := Restore(dst, &config.Config{GPGRecipient: "test@example.com"}, RestoreOptions{
		In: bytes.NewReader(nil), Force: true, SkipExisting: true,
	})
	if err == nil {
		t.Fatal("Restore() error = nil, want mutual-exclusion error")
	}
}

func TestRestore_RejectsManifestNotFirst(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	body := []byte("a")
	_ = tw.WriteHeader(&tar.Header{Name: ProfilesDir + "alpha" + EncryptedExt, Mode: 0600, Size: int64(len(body))})
	_, _ = tw.Write(body)
	manifestBody, _ := yaml.Marshal(&Manifest{
		SchemaVersion: 1, CreatedAt: time.Now().UTC(), KubegonfigVersion: "0.2.0",
		Encrypted: true, ProfileCount: 1,
		Profiles: []ManifestProfile{{Name: "alpha", File: ProfilesDir + "alpha" + EncryptedExt}},
	})
	_ = tw.WriteHeader(&tar.Header{Name: ManifestName, Mode: 0600, Size: int64(len(manifestBody))})
	_, _ = tw.Write(manifestBody)
	_ = tw.Close()

	dst := newSeededManager(t, nil)
	_, err := Restore(dst, &config.Config{GPGRecipient: "test@example.com"}, RestoreOptions{In: bytes.NewReader(buf.Bytes())})
	if err == nil {
		t.Fatal("Restore() error = nil, want manifest-not-first error")
	}
}

func TestRestore_RejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	manifestBody, _ := yaml.Marshal(&Manifest{
		SchemaVersion: 1, CreatedAt: time.Now().UTC(), KubegonfigVersion: "0.2.0",
		Encrypted: true, ProfileCount: 0, Profiles: nil,
	})
	_ = tw.WriteHeader(&tar.Header{Name: ManifestName, Mode: 0600, Size: int64(len(manifestBody))})
	_, _ = tw.Write(manifestBody)
	body := []byte("evil")
	_ = tw.WriteHeader(&tar.Header{Name: "../etc/passwd", Mode: 0600, Size: int64(len(body))})
	_, _ = tw.Write(body)
	_ = tw.Close()

	dst := newSeededManager(t, nil)
	_, err := Restore(dst, &config.Config{GPGRecipient: "test@example.com"}, RestoreOptions{In: bytes.NewReader(buf.Bytes())})
	if err == nil {
		t.Fatal("Restore() error = nil, want path-traversal error")
	}
}

func TestExport_LockSerializesWithDelete(t *testing.T) {
	mgr := newSeededManager(t, map[string][]byte{
		"alpha": []byte("a"),
		"beta":  []byte("b"),
	})
	cfg := &config.Config{GPGRecipient: "test@example.com"}

	exportDone := make(chan error, 1)
	deleteDone := make(chan error, 1)

	var buf bytes.Buffer
	go func() {
		exportDone <- Export(mgr, cfg, ExportOptions{
			Names: []string{"alpha", "beta"}, KubegonfigVersion: "0.2.0", Out: &buf,
		})
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		deleteDone <- mgr.Delete("alpha")
	}()

	if err := <-exportDone; err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if err := <-deleteDone; err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	entries := readTarEntries(t, buf.Bytes())
	manifest, err := ParseManifest(entries[ManifestName])
	if err != nil {
		t.Fatalf("ParseManifest(): %v", err)
	}
	for _, p := range manifest.Profiles {
		if _, ok := entries[p.File]; !ok {
			t.Errorf("manifest lists %q but tar has no entry; entries: %v", p.File, keysOf(entries))
		}
	}
}

func TestRestore_MergeConfigUnionsRecipients(t *testing.T) {
	srcCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "src@example.com"}
	srcCfg.GPGRecipients = []string{"shared@example.com"}
	srcCfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	if err := srcCfg.Save(); err != nil {
		t.Fatalf("save src config: %v", err)
	}
	srcMgr, _ := profile.NewManager(srcCfg)
	if err := srcMgr.WriteEncrypted("alpha", []byte("a")); err != nil {
		t.Fatalf("WriteEncrypted(): %v", err)
	}
	var buf bytes.Buffer
	if err := Export(srcMgr, srcCfg, ExportOptions{
		Names: []string{"alpha"}, IncludeConfig: true,
		KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dstCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "dst@example.com"}
	dstCfg.GPGRecipients = []string{"local-only@example.com"}
	dstCfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	if err := dstCfg.Save(); err != nil {
		t.Fatalf("save dst config: %v", err)
	}
	dstMgr, _ := profile.NewManager(dstCfg)

	if _, err := Restore(dstMgr, dstCfg, RestoreOptions{
		In: bytes.NewReader(buf.Bytes()), MergeConfig: true,
	}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}

	if dstCfg.GPGRecipient != "dst@example.com" {
		t.Errorf("primary recipient overwritten: got %q", dstCfg.GPGRecipient)
	}
	got := append([]string(nil), dstCfg.GPGRecipients...)
	sort.Strings(got)
	want := []string{"local-only@example.com", "shared@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GPGRecipients = %v, want %v", got, want)
	}
}

func TestRestore_ProfilesOnlySkipsConfigEvenWithMergeConfig(t *testing.T) {
	srcCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "src@example.com"}
	srcCfg.GPGRecipients = []string{"shared@example.com"}
	srcCfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	_ = srcCfg.Save()
	srcMgr, _ := profile.NewManager(srcCfg)
	if err := srcMgr.WriteEncrypted("alpha", []byte("a")); err != nil {
		t.Fatalf("WriteEncrypted(): %v", err)
	}
	var buf bytes.Buffer
	_ = Export(srcMgr, srcCfg, ExportOptions{
		Names: []string{"alpha"}, IncludeConfig: true,
		KubegonfigVersion: "0.2.0", Out: &buf,
	})

	dstCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "dst@example.com"}
	dstCfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	_ = dstCfg.Save()
	dstMgr, _ := profile.NewManager(dstCfg)

	plan, err := Restore(dstMgr, dstCfg, RestoreOptions{
		In: bytes.NewReader(buf.Bytes()), MergeConfig: true, ProfilesOnly: true,
	})
	if err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if plan.ConfigAction != "skip-profiles-only" {
		t.Errorf("plan.ConfigAction = %q, want skip-profiles-only", plan.ConfigAction)
	}
	if len(dstCfg.GPGRecipients) != 0 {
		t.Errorf("GPGRecipients merged despite ProfilesOnly: %v", dstCfg.GPGRecipients)
	}
}

func TestRestore_PlaintextArchiveReEncryptsWithLocalRecipients(t *testing.T) {
	installFakeGPGForBundle(t)

	srcCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "old@example.com"}
	srcMgr, err := profile.NewManager(srcCfg)
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}
	if err := srcMgr.WriteEncrypted("alpha", validKubeconfigForBundle()); err != nil {
		t.Fatalf("WriteEncrypted(): %v", err)
	}

	var buf bytes.Buffer
	if err := Export(srcMgr, srcCfg, ExportOptions{
		Names: []string{"alpha"}, Decrypt: true,
		KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dstCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "new@example.com"}
	dstMgr, err := profile.NewManager(dstCfg)
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}

	if _, err := Restore(dstMgr, dstCfg, RestoreOptions{In: bytes.NewReader(buf.Bytes())}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}

	got, err := dstMgr.ReadEncrypted("alpha")
	if err != nil {
		t.Fatalf("ReadEncrypted(): %v", err)
	}
	if !bytes.Equal(got, validKubeconfigForBundle()) {
		t.Errorf("re-encrypted body mismatch")
	}
}

func TestRestore_PlaintextArchiveRejectsInvalidKubeconfig(t *testing.T) {
	installFakeGPGForBundle(t)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	now := time.Now().UTC().Truncate(time.Second)
	manifestBody, _ := yaml.Marshal(&Manifest{
		SchemaVersion: 1, CreatedAt: now, KubegonfigVersion: "0.2.0",
		Encrypted: false, ProfileCount: 1,
		Profiles: []ManifestProfile{{Name: "alpha", File: ProfilesDir + "alpha" + PlaintextExt}},
	})
	_ = tw.WriteHeader(&tar.Header{Name: ManifestName, Mode: 0600, Size: int64(len(manifestBody)), ModTime: now})
	_, _ = tw.Write(manifestBody)
	body := []byte("not-a-kubeconfig")
	_ = tw.WriteHeader(&tar.Header{Name: ProfilesDir + "alpha" + PlaintextExt, Mode: 0600, Size: int64(len(body)), ModTime: now})
	_, _ = tw.Write(body)
	_ = tw.Close()

	dstCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "new@example.com"}
	dstMgr, _ := profile.NewManager(dstCfg)

	_, err := Restore(dstMgr, dstCfg, RestoreOptions{In: bytes.NewReader(buf.Bytes())})
	if err == nil {
		t.Fatal("Restore() error = nil, want invalid kubeconfig error")
	}
}

func TestRestore_RejectsOversizedProfile(t *testing.T) {
	huge := bytes.Repeat([]byte("x"), MaxProfileSize+1)
	src := newSeededManager(t, map[string][]byte{"alpha": huge})
	srcCfg := &config.Config{GPGRecipient: "test@example.com"}
	var buf bytes.Buffer
	if err := Export(src, srcCfg, ExportOptions{
		Names: []string{"alpha"}, KubegonfigVersion: "0.2.0", Out: &buf,
	}); err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	dst := newSeededManager(t, nil)
	_, err := Restore(dst, &config.Config{GPGRecipient: "test@example.com"}, RestoreOptions{In: bytes.NewReader(buf.Bytes())})
	if err == nil {
		t.Fatal("Restore() error = nil, want size cap error")
	}
}

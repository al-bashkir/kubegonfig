// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"archive/tar"
	"bytes"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"
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

	encoded, err := MarshalManifest(&want)
	if err != nil {
		t.Fatalf("MarshalManifest() error: %v", err)
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

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"archive/tar"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"

	"gopkg.in/yaml.v3"
)

// buildEncryptedArchive writes an archive with a manifest, the given
// encrypted profiles, and an optional config.yaml body (nil = no config).
func buildEncryptedArchive(t *testing.T, profiles map[string][]byte, configBody []byte) []byte {
	t.Helper()
	names := keysOf(profiles)
	m := Manifest{
		SchemaVersion: SchemaVersion, CreatedAt: time.Now().UTC().Truncate(time.Second),
		Encrypted: true, ProfileCount: len(names), ConfigIncluded: configBody != nil,
	}
	for _, n := range names {
		m.Profiles = append(m.Profiles, ManifestProfile{Name: n, File: ProfilesDir + n + EncryptedExt})
	}
	mb, err := yaml.Marshal(&m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := writeTarEntry(tw, ManifestName, mb, m.CreatedAt); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := writeTarEntry(tw, ProfilesDir+n+EncryptedExt, profiles[n], m.CreatedAt); err != nil {
			t.Fatal(err)
		}
	}
	if configBody != nil {
		if err := writeTarEntry(tw, ConfigName, configBody, m.CreatedAt); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newConfigOnDisk(t *testing.T, recipients ...string) *config.Config {
	t.Helper()
	cfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "dst@example.com", GPGRecipients: recipients}
	cfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return cfg
}

func TestRestore_SingleCollisionFails(t *testing.T) {
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("new"), "beta": []byte("b")}, nil)
	dst := newSeededManager(t, map[string][]byte{"alpha": []byte("old")})

	_, err := Restore(dst, &config.Config{GPGRecipient: "t@example.com"}, RestoreOptions{In: bytes.NewReader(archive)})
	if err == nil || !strings.Contains(err.Error(), "alpha") {
		t.Fatalf("Restore() error = %v, want collision error naming alpha", err)
	}
	if got, _ := dst.ReadEncrypted("alpha"); string(got) != "old" {
		t.Errorf("alpha = %q, want unchanged %q", got, "old")
	}
	if ok, _ := dst.Exists("beta"); ok {
		t.Error("beta written despite collision error")
	}
}

func TestRestore_ConfigActionRequiresFlagAndArchiveConfig(t *testing.T) {
	installFakeGPGForBundle(t)
	withCfg := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")}, []byte("gpg_recipients: [extra@example.com]\n"))
	noCfg := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")}, nil)
	tests := []struct {
		name    string
		archive []byte
		merge   bool
		action  string
		want    []string
	}{
		{"config but no flag", withCfg, false, "ignore", nil},
		{"flag but no config", noCfg, true, "ignore", nil},
		{"both", withCfg, true, "merge", []string{"extra@example.com"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newConfigOnDisk(t)
			mgr, _ := profile.NewManager(cfg)
			plan, err := Restore(mgr, cfg, RestoreOptions{In: bytes.NewReader(tc.archive), MergeConfig: tc.merge})
			if err != nil {
				t.Fatalf("Restore() error: %v", err)
			}
			if plan.ConfigAction != tc.action {
				t.Errorf("ConfigAction = %q, want %q", plan.ConfigAction, tc.action)
			}
			if !reflect.DeepEqual(cfg.GPGRecipients, tc.want) {
				t.Errorf("GPGRecipients = %v, want %v", cfg.GPGRecipients, tc.want)
			}
		})
	}
}

func TestRestore_MergeSkipsBlankAndDuplicateRecipients(t *testing.T) {
	installFakeGPGForBundle(t)
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")},
		[]byte("gpg_recipients: ['', '  ', local@example.com, ' new@example.com ']\n"))
	cfg := newConfigOnDisk(t, "local@example.com")
	mgr, _ := profile.NewManager(cfg)

	if _, err := Restore(mgr, cfg, RestoreOptions{In: bytes.NewReader(archive), MergeConfig: true}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	want := []string{"local@example.com", "new@example.com"}
	if !reflect.DeepEqual(cfg.GPGRecipients, want) {
		t.Errorf("GPGRecipients = %v, want %v", cfg.GPGRecipients, want)
	}
}

func TestRestore_AcceptsProfileOfExactlyMaxSize(t *testing.T) {
	body := bytes.Repeat([]byte("x"), MaxProfileSize)
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": body}, nil)
	dst := newSeededManager(t, nil)
	if _, err := Restore(dst, &config.Config{GPGRecipient: "t@example.com"}, RestoreOptions{In: bytes.NewReader(archive)}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if got, _ := dst.ReadEncrypted("alpha"); len(got) != MaxProfileSize {
		t.Errorf("stored %d bytes, want %d", len(got), MaxProfileSize)
	}
}

func TestRestore_RejectsOversizedConfig(t *testing.T) {
	big := "gpg_recipients: [a@example.com]\n#" + strings.Repeat("x", MaxProfileSize)
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")}, []byte(big))
	cfg := newConfigOnDisk(t)
	mgr, _ := profile.NewManager(cfg)

	_, err := Restore(mgr, cfg, RestoreOptions{In: bytes.NewReader(archive), MergeConfig: true})
	want := fmt.Sprintf("archive entry %q exceeds %d byte limit", ConfigName, MaxProfileSize)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Restore() error = %v, want %q", err, want)
	}
	if len(cfg.GPGRecipients) != 0 {
		t.Errorf("truncated config merged: %v", cfg.GPGRecipients)
	}
}

func TestRestore_AcceptsConfigOfExactlyMaxSize(t *testing.T) {
	installFakeGPGForBundle(t)
	head := "gpg_recipients: [a@example.com]\n#"
	body := head + strings.Repeat("x", MaxProfileSize-len(head))
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")}, []byte(body))
	cfg := newConfigOnDisk(t)
	mgr, _ := profile.NewManager(cfg)

	if _, err := Restore(mgr, cfg, RestoreOptions{In: bytes.NewReader(archive), MergeConfig: true}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if !reflect.DeepEqual(cfg.GPGRecipients, []string{"a@example.com"}) {
		t.Errorf("GPGRecipients = %v", cfg.GPGRecipients)
	}
}

func TestRestore_AcceptsManifestOfExactly64KiB(t *testing.T) {
	m := Manifest{SchemaVersion: SchemaVersion, CreatedAt: time.Now().UTC().Truncate(time.Second), Encrypted: true}
	mb, _ := yaml.Marshal(&m)
	pad := 64*1024 - len(mb)
	mb = append(mb, []byte("#"+strings.Repeat("p", pad-2)+"\n")...)
	if len(mb) != 64*1024 {
		t.Fatalf("manifest len = %d", len(mb))
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := writeTarEntry(tw, ManifestName, mb, m.CreatedAt); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	dst := newSeededManager(t, nil)
	if _, err := Restore(dst, &config.Config{GPGRecipient: "t@example.com"}, RestoreOptions{In: &buf}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
}

// dirSpy lists dir on the first Read, so it sees what exists on disk while
// Restore buffers the archive.
type dirSpy struct {
	r      *bytes.Reader
	dir    string
	t      *testing.T
	listed []string
}

func (s *dirSpy) Read(p []byte) (int, error) {
	if s.listed == nil {
		entries, err := os.ReadDir(s.dir)
		if err != nil {
			s.t.Fatalf("read runtime dir: %v", err)
		}
		s.listed = []string{}
		for _, e := range entries {
			s.listed = append(s.listed, e.Name())
		}
	}
	return s.r.Read(p)
}

func TestRestore_BufferLeavesNoNamedFileInRuntimeDir(t *testing.T) {
	runtime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")}, nil)
	dst := newSeededManager(t, nil)

	spy := &dirSpy{r: bytes.NewReader(archive), dir: filepath.Join(runtime, "kubegonfig"), t: t}
	if _, err := Restore(dst, &config.Config{GPGRecipient: "t@example.com"}, RestoreOptions{In: spy}); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if len(spy.listed) != 0 {
		t.Errorf("runtime dir held %v while buffering; a killed process would leave plaintext behind", spy.listed)
	}
}

// installLoggingFakeGPG installs a gpg stub that logs its arguments, fails
// --list-keys for any recipient containing "unknown@", and otherwise acts as cat.
func installLoggingFakeGPG(t *testing.T) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "gpg.log")
	script := "#!/bin/sh\necho \"$@\" >> " + logPath + "\n" +
		"case \"$*\" in *--list-keys*unknown@*) echo 'gpg: error reading key: No public key' >&2; exit 2;; esac\ncat\n"
	for _, name := range []string{"gpg2", "gpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// buildPlaintextArchiveConfigFirst writes manifest, config.yaml, then the
// plaintext profile, the order an attacker-built archive may use.
func buildPlaintextArchiveConfigFirst(t *testing.T, name string, kubeconfig, configBody []byte) []byte {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	m := Manifest{SchemaVersion: SchemaVersion, CreatedAt: now, ProfileCount: 1, ConfigIncluded: true,
		Profiles: []ManifestProfile{{Name: name, File: ProfilesDir + name + PlaintextExt}}}
	mb, err := yaml.Marshal(&m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range []struct {
		name string
		body []byte
	}{{ManifestName, mb}, {ConfigName, configBody}, {ProfilesDir + name + PlaintextExt, kubeconfig}} {
		if err := writeTarEntry(tw, e.name, e.body, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRestore_MergedRecipientsDoNotAffectSameRunEncryption(t *testing.T) {
	logPath := installLoggingFakeGPG(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	archive := buildPlaintextArchiveConfigFirst(t, "alpha", validKubeconfigForBundle(), []byte("gpg_recipients: [extra@example.com]\n"))
	cfg := newConfigOnDisk(t)
	mgr, _ := profile.NewManager(cfg)

	if _, err := Restore(mgr, cfg, RestoreOptions{In: bytes.NewReader(archive), MergeConfig: true}); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	logged, _ := os.ReadFile(logPath)
	for _, line := range strings.Split(string(logged), "\n") {
		if strings.Contains(line, "--encrypt") && strings.Contains(line, "extra@example.com") {
			t.Errorf("profile was encrypted to an archive-supplied recipient: %q", line)
		}
	}
	if !reflect.DeepEqual(cfg.GPGRecipients, []string{"extra@example.com"}) {
		t.Errorf("GPGRecipients = %v, want the merged recipient saved", cfg.GPGRecipients)
	}
}

func TestRestore_MergeRejectsRecipientWithoutLocalKey(t *testing.T) {
	installLoggingFakeGPG(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")},
		[]byte("gpg_recipients: [unknown@example.com, good@example.com]\n"))
	cfg := newConfigOnDisk(t)
	mgr, _ := profile.NewManager(cfg)

	plan, err := Restore(mgr, cfg, RestoreOptions{In: bytes.NewReader(archive), MergeConfig: true})
	if err == nil || !strings.Contains(err.Error(), "unknown@example.com") {
		t.Fatalf("Restore() error = %v, want error naming unknown@example.com", err)
	}
	if !reflect.DeepEqual(cfg.GPGRecipients, []string{"good@example.com"}) {
		t.Errorf("GPGRecipients = %v, want only the recipient with a local key", cfg.GPGRecipients)
	}
	if !reflect.DeepEqual(plan.AddedRecipients, []string{"good@example.com"}) {
		t.Errorf("AddedRecipients = %v, want [good@example.com]", plan.AddedRecipients)
	}
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"kubegonfig/internal/config"
)

// budgetWriter accepts n bytes, then fails every write.
type budgetWriter struct{ n int }

func (w *budgetWriter) Write(p []byte) (int, error) {
	if len(p) > w.n {
		return 0, errors.New("disk full")
	}
	w.n -= len(p)
	return len(p), nil
}

func TestExport_IncludeConfigMissingFileFails(t *testing.T) {
	mgr := newSeededManager(t, map[string][]byte{"alpha": []byte("a")})
	cfg := &config.Config{GPGRecipient: "t@example.com", Path: t.TempDir() + "/config.yaml"}

	err := Export(mgr, cfg, ExportOptions{
		Names: []string{"alpha"}, IncludeConfig: true, KubegonfigVersion: "0.2.0", Out: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "read config.yaml") {
		t.Fatalf("Export() error = %v, want read config.yaml error", err)
	}
}

func TestExport_WriteFailureAtAnyPointIsReported(t *testing.T) {
	mgr := newSeededManager(t, map[string][]byte{"alpha": []byte("a")})
	cfg := newConfigOnDisk(t)
	opts := ExportOptions{Names: []string{"alpha"}, IncludeConfig: true, KubegonfigVersion: "0.2.0"}

	var full bytes.Buffer
	opts.Out = &full
	if err := Export(mgr, cfg, opts); err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if !bytes.HasSuffix(full.Bytes(), make([]byte, 1024)) {
		t.Error("archive lacks tar end-of-archive trailer")
	}
	// Every 512-byte tar block boundary: header, body, trailer.
	for budget := 0; budget < full.Len(); budget += 512 {
		opts.Out = &budgetWriter{n: budget}
		if err := Export(mgr, cfg, opts); err == nil {
			t.Errorf("budget %d of %d: Export() error = nil, want write error", budget, full.Len())
		}
	}
}

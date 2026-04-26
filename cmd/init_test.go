// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import "testing"

func TestInitRecipientFromFlag(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		changed   bool
		want      string
		wantOK    bool
		wantError bool
	}{
		{name: "not set", value: "", changed: false, want: "", wantOK: false},
		{name: "trimmed", value: " user@example.com ", changed: true, want: "user@example.com", wantOK: true},
		{name: "blank", value: " \t ", changed: true, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := initRecipientFromFlag(tt.value, tt.changed)
			if (err != nil) != tt.wantError {
				t.Fatalf("initRecipientFromFlag() error = %v, wantError %v", err, tt.wantError)
			}
			if got != tt.want {
				t.Fatalf("initRecipientFromFlag() recipient = %q, want %q", got, tt.want)
			}
			if ok != tt.wantOK {
				t.Fatalf("initRecipientFromFlag() ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

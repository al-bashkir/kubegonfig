// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package shell provides profile name validation, shell-safe escaping,
// and environment variable export formatting.
package shell

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	StylePosix = "posix"
	StyleFish  = "fish"
)

// validNameRe matches safe profile names: starts with alnum, then alnum/dot/dash/underscore.
// Max length 64 to prevent filesystem issues.
var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// ValidateName checks that a profile name is safe for use as a filename
// and in shell commands. Rejects path traversal, shell metacharacters,
// and empty/blank names.
func ValidateName(name string) error {
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("profile name %q is invalid: must start with alphanumeric, "+
			"contain only [a-zA-Z0-9._-], and be 1-64 characters", name)
	}
	return nil
}

// EscapePosix returns a POSIX shell-safe single-quoted string.
// Single quotes within the value are escaped as '\” (end quote, escaped quote, start quote).
func EscapePosix(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// NormalizeStyle validates a shell output style and applies the default.
func NormalizeStyle(style string) (string, error) {
	switch style {
	case "":
		return StylePosix, nil
	case StylePosix, StyleFish:
		return style, nil
	default:
		return "", fmt.Errorf("unsupported shell style %q (expected %q or %q)", style, StylePosix, StyleFish)
	}
}

// FormatSet returns a shell command that exports key=value. style must come
// from NormalizeStyle.
func FormatSet(style, key, value string) string {
	if style == StyleFish {
		return fmt.Sprintf("set -gx %s %s", key, EscapePosix(value))
	}
	return fmt.Sprintf("export %s=%s", key, EscapePosix(value))
}

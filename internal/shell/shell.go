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

var validEnvKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateName checks that a profile name is safe for use as a filename
// and in shell commands. Rejects path traversal, shell metacharacters,
// and empty/blank names.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name must not be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("profile name %q is not allowed", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("profile name %q contains path separator", name)
	}
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

func validateEnvKey(key string) error {
	if !validEnvKeyRe.MatchString(key) {
		return fmt.Errorf("invalid environment variable name %q", key)
	}
	return nil
}

func formatExport(key, value string) string {
	return fmt.Sprintf("export %s=%s", key, EscapePosix(value))
}

func formatFishSet(key, value string) string {
	return fmt.Sprintf("set -gx %s %s", key, EscapePosix(value))
}

// NormalizeStyle validates a shell output style and applies the default.
func NormalizeStyle(style string) (string, error) {
	if style == "" {
		return StylePosix, nil
	}
	switch style {
	case StylePosix, StyleFish:
		return style, nil
	default:
		return "", fmt.Errorf("unsupported shell style %q (expected %q or %q)", style, StylePosix, StyleFish)
	}
}

// FormatSet returns a shell command that exports key=value for the given style.
func FormatSet(style, key, value string) (string, error) {
	style, err := NormalizeStyle(style)
	if err != nil {
		return "", err
	}
	if err := validateEnvKey(key); err != nil {
		return "", err
	}
	switch style {
	case StyleFish:
		return formatFishSet(key, value), nil
	default:
		return formatExport(key, value), nil
	}
}

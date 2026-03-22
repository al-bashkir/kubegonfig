// Package shell provides profile name validation, shell-safe escaping,
// and environment variable export formatting.
package shell

import (
	"fmt"
	"regexp"
	"strings"
)

// validNameRe matches safe profile names: starts with alnum, then alnum/dot/dash/underscore.
// Max length 64 to prevent filesystem issues.
var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

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

// FormatExport returns a POSIX-compatible export statement.
// Output: export KEY='value'
func FormatExport(key, value string) string {
	return fmt.Sprintf("export %s=%s", key, EscapePosix(value))
}

// FormatUnset returns a POSIX-compatible unset statement.
func FormatUnset(key string) string {
	return fmt.Sprintf("unset %s", key)
}

// FormatFishSet returns a fish shell set statement.
// Output: set -gx KEY 'value'
func FormatFishSet(key, value string) string {
	return fmt.Sprintf("set -gx %s %s", key, EscapePosix(value))
}

// FormatFishErase returns a fish shell erase statement.
func FormatFishErase(key string) string {
	return fmt.Sprintf("set -e %s", key)
}

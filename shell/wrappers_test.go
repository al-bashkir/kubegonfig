// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package shell_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBashWrapperHelpPassesThroughWithoutEval(t *testing.T) {
	testWrapperHelpPassesThroughWithoutEval(t, "bash", "kubegonfig.bash")
}

func TestZshWrapperHelpPassesThroughWithoutEval(t *testing.T) {
	testWrapperHelpPassesThroughWithoutEval(t, "zsh", "kubegonfig.zsh")
}

func TestBashWrapperAddsDefaultPosixShellStyle(t *testing.T) {
	testWrapperAddsDefaultPosixShellStyle(t, "bash", "kubegonfig.bash")
}

func TestZshWrapperAddsDefaultPosixShellStyle(t *testing.T) {
	testWrapperAddsDefaultPosixShellStyle(t, "zsh", "kubegonfig.zsh")
}

func TestFishWrapperHelpPassesThroughWithoutEval(t *testing.T) {
	testFishWrapperHelpPassesThroughWithoutEval(t)
}

func TestFishWrapperAddsDefaultFishShellStyle(t *testing.T) {
	testFishWrapperAddsDefaultFishShellStyle(t)
}

func testWrapperHelpPassesThroughWithoutEval(t *testing.T, shellName, wrapperFile string) {
	t.Helper()

	shellPath, err := exec.LookPath(shellName)
	if err != nil {
		t.Skipf("%s not installed: %v", shellName, err)
	}

	wrapperPath, err := filepath.Abs(wrapperFile)
	if err != nil {
		t.Fatalf("Abs(%q) error: %v", wrapperFile, err)
	}

	stubDir := writeStubKubegonfig(t)

	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "env long help", args: []string{"env", "--help"}},
		{name: "use short help", args: []string{"use", "-h"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			argsPath := filepath.Join(t.TempDir(), "args")
			script := fmt.Sprintf(`source %s
KUBECONFIG=before
kubegonfig %s
printf 'after:%%s\n' "${KUBECONFIG:-unset}"
`, shellQuote(wrapperPath), joinShellArgs(tt.args))

			cmd := exec.Command(shellPath, "-c", script)
			cmd.Env = append(os.Environ(),
				"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"KUBEGONFIG_STUB_ARGS="+argsPath,
			)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s wrapper command error: %v\noutput:\n%s", shellName, err, out)
			}

			wantOut := "KUBECONFIG=evalled\nexport KUBECONFIG\nafter:before\n"
			if got := string(out); got != wantOut {
				t.Fatalf("%s wrapper output = %q, want %q", shellName, got, wantOut)
			}

			argsData, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatalf("ReadFile(%q) error: %v", argsPath, err)
			}
			wantArgs := strings.Join(tt.args, " ") + "\n"
			if got := string(argsData); got != wantArgs {
				t.Fatalf("stub args = %q, want %q", got, wantArgs)
			}
		})
	}
}

func testWrapperAddsDefaultPosixShellStyle(t *testing.T, shellName, wrapperFile string) {
	t.Helper()

	shellPath, err := exec.LookPath(shellName)
	if err != nil {
		t.Skipf("%s not installed: %v", shellName, err)
	}

	wrapperPath, err := filepath.Abs(wrapperFile)
	if err != nil {
		t.Fatalf("Abs(%q) error: %v", wrapperFile, err)
	}

	stubDir := writeStubKubegonfig(t)

	for _, tt := range []struct {
		name     string
		args     []string
		wantArgs string
	}{
		{
			name:     "env default shell",
			args:     []string{"env", "prod"},
			wantArgs: "env prod --shell posix\n",
		},
		{
			name:     "env with global quiet before subcommand",
			args:     []string{"--quiet", "env", "prod"},
			wantArgs: "--quiet env prod --shell posix\n",
		},
		{
			name:     "use with short global quiet before subcommand",
			args:     []string{"-q", "use", "prod"},
			wantArgs: "-q use prod --shell posix\n",
		},
		{
			name:     "use explicit shell flag",
			args:     []string{"use", "prod", "--shell", "fish"},
			wantArgs: "use prod --shell fish\n",
		},
		{
			name:     "env explicit shell equals",
			args:     []string{"env", "prod", "--shell=fish"},
			wantArgs: "env prod --shell=fish\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			argsPath := filepath.Join(t.TempDir(), "args")
			script := fmt.Sprintf(`source %s
KUBECONFIG=before
kubegonfig %s
printf 'after:%%s\n' "${KUBECONFIG:-unset}"
`, shellQuote(wrapperPath), joinShellArgs(tt.args))

			cmd := exec.Command(shellPath, "-c", script)
			cmd.Env = append(os.Environ(),
				"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"KUBEGONFIG_STUB_ARGS="+argsPath,
			)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s wrapper command error: %v\noutput:\n%s", shellName, err, out)
			}
			if got, want := string(out), "after:evalled\n"; got != want {
				t.Fatalf("%s wrapper output = %q, want %q", shellName, got, want)
			}

			argsData, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatalf("ReadFile(%q) error: %v", argsPath, err)
			}
			if got := string(argsData); got != tt.wantArgs {
				t.Fatalf("stub args = %q, want %q", got, tt.wantArgs)
			}
		})
	}
}

func testFishWrapperHelpPassesThroughWithoutEval(t *testing.T) {
	t.Helper()

	shellPath, err := exec.LookPath("fish")
	if err != nil {
		t.Skipf("fish not installed: %v", err)
	}
	wrapperPath, err := filepath.Abs("kubegonfig.fish")
	if err != nil {
		t.Fatalf("Abs(kubegonfig.fish) error: %v", err)
	}
	stubDir := writeStubKubegonfig(t)

	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "env long help", args: []string{"env", "--help"}},
		{name: "use short help", args: []string{"use", "-h"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			argsPath := filepath.Join(t.TempDir(), "args")
			script := fmt.Sprintf(`source %s
set -gx KUBECONFIG before
kubegonfig %s
printf 'after:%%s\n' "$KUBECONFIG"
`, shellQuote(wrapperPath), joinShellArgs(tt.args))

			cmd := exec.Command(shellPath, "-c", script)
			cmd.Env = append(os.Environ(),
				"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"KUBEGONFIG_STUB_ARGS="+argsPath,
				"KUBEGONFIG_STUB_OUTPUT=fish",
			)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("fish wrapper command error: %v\noutput:\n%s", err, out)
			}
			wantOut := "set -gx KUBECONFIG evalled\nafter:before\n"
			if got := string(out); got != wantOut {
				t.Fatalf("fish wrapper output = %q, want %q", got, wantOut)
			}

			argsData, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatalf("ReadFile(%q) error: %v", argsPath, err)
			}
			wantArgs := strings.Join(tt.args, " ") + "\n"
			if got := string(argsData); got != wantArgs {
				t.Fatalf("stub args = %q, want %q", got, wantArgs)
			}
		})
	}
}

func testFishWrapperAddsDefaultFishShellStyle(t *testing.T) {
	t.Helper()

	shellPath, err := exec.LookPath("fish")
	if err != nil {
		t.Skipf("fish not installed: %v", err)
	}
	wrapperPath, err := filepath.Abs("kubegonfig.fish")
	if err != nil {
		t.Fatalf("Abs(kubegonfig.fish) error: %v", err)
	}
	stubDir := writeStubKubegonfig(t)

	for _, tt := range []struct {
		name     string
		args     []string
		wantArgs string
	}{
		{
			name:     "env default shell",
			args:     []string{"env", "prod"},
			wantArgs: "env prod --shell fish\n",
		},
		{
			name:     "env with global quiet before subcommand",
			args:     []string{"--quiet", "env", "prod"},
			wantArgs: "--quiet env prod --shell fish\n",
		},
		{
			name:     "use with short global quiet before subcommand",
			args:     []string{"-q", "use", "prod"},
			wantArgs: "-q use prod --shell fish\n",
		},
		{
			name:     "use explicit shell flag",
			args:     []string{"use", "prod", "--shell", "posix"},
			wantArgs: "use prod --shell posix\n",
		},
		{
			name:     "env explicit shell equals",
			args:     []string{"env", "prod", "--shell=posix"},
			wantArgs: "env prod --shell=posix\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			argsPath := filepath.Join(t.TempDir(), "args")
			script := fmt.Sprintf(`source %s
set -gx KUBECONFIG before
kubegonfig %s
printf 'after:%%s\n' "$KUBECONFIG"
`, shellQuote(wrapperPath), joinShellArgs(tt.args))

			cmd := exec.Command(shellPath, "-c", script)
			cmd.Env = append(os.Environ(),
				"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"KUBEGONFIG_STUB_ARGS="+argsPath,
				"KUBEGONFIG_STUB_OUTPUT=fish",
			)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("fish wrapper command error: %v\noutput:\n%s", err, out)
			}
			if got, want := string(out), "after:evalled\n"; got != want {
				t.Fatalf("fish wrapper output = %q, want %q", got, want)
			}

			argsData, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatalf("ReadFile(%q) error: %v", argsPath, err)
			}
			if got := string(argsData); got != tt.wantArgs {
				t.Fatalf("stub args = %q, want %q", got, tt.wantArgs)
			}
		})
	}
}

func writeStubKubegonfig(t *testing.T) string {
	t.Helper()

	stubDir := t.TempDir()
	stubPath := filepath.Join(stubDir, "kubegonfig")
	stub := `#!/bin/sh
printf '%s\n' "$*" >> "$KUBEGONFIG_STUB_ARGS"
if [ "${KUBEGONFIG_STUB_OUTPUT:-posix}" = fish ]; then
    printf '%s\n' 'set -gx KUBECONFIG evalled'
else
    printf '%s\n' 'KUBECONFIG=evalled' 'export KUBECONFIG'
fi
`
	if err := os.WriteFile(stubPath, []byte(stub), 0700); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", stubPath, err)
	}
	return stubDir
}

func joinShellArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

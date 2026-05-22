// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"kubegonfig/internal/shell"
	"kubegonfig/internal/tmpfile"
)

func activeShellStyle(flagValue string) (string, error) {
	style := flagValue
	if style == "" {
		style = cfg.ShellStyle
	}
	return shell.NormalizeStyle(style)
}

func activateProfile(name, shellFlag string) (string, error) {
	style, err := activeShellStyle(shellFlag)
	if err != nil {
		return "", err
	}

	path, err := mgr.Activate(
		name,
		tmpfile.ProbeCached,
		func(data []byte) (string, error) {
			return tmpfile.Create(name, data)
		},
		tmpfile.Remove,
	)
	if err != nil {
		return "", err
	}

	return shell.FormatSet(style, "KUBECONFIG", path)
}

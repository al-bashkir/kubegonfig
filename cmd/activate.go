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

	data, err := mgr.Decrypt(name)
	if err != nil {
		return "", err
	}

	path, err := tmpfile.Create(name, data)
	if err != nil {
		return "", err
	}

	if err := mgr.SetCurrent(name); err != nil {
		_ = tmpfile.Remove(path)
		return "", err
	}

	return shell.FormatSet(style, "KUBECONFIG", path)
}

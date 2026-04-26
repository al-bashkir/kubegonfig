// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// kubegonfig is a secure kubeconfig profile manager.
// It stores profiles encrypted with GPG and provides safe shell
// integration for Kubernetes context switching.
package main

import "kubegonfig/cmd"

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}

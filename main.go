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

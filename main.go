// kubegonfig is a secure kubeconfig profile manager.
// It stores profiles encrypted with GPG and provides safe shell
// integration for Kubernetes context switching.
package main

import "kubegonfig/cmd"

func main() {
	cmd.Execute()
}

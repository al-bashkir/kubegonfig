// Package kubeconfig provides validation of kubeconfig YAML content.
// It checks structural correctness without interpreting or logging secrets.
package kubeconfig

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// kubeConfig is a minimal representation of a kubeconfig file.
// Only structural fields are checked; secret values are never inspected.
type kubeConfig struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Clusters   []clusterEntry `yaml:"clusters"`
	Contexts   []contextEntry `yaml:"contexts"`
	Users      []userEntry    `yaml:"users"`
}

type clusterEntry struct {
	Name    string      `yaml:"name"`
	Cluster clusterData `yaml:"cluster"`
}

type clusterData struct {
	Server string `yaml:"server"`
}

type contextEntry struct {
	Name    string      `yaml:"name"`
	Context contextData `yaml:"context"`
}

type contextData struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type userEntry struct {
	Name string `yaml:"name"`
}

// Validate checks that data is a valid kubeconfig YAML with required fields.
func Validate(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("kubeconfig is empty")
	}

	var kc kubeConfig
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	if kc.APIVersion == "" {
		return fmt.Errorf("missing required field: apiVersion")
	}
	if kc.APIVersion != "v1" {
		return fmt.Errorf("unsupported apiVersion: %q (expected \"v1\")", kc.APIVersion)
	}
	if kc.Kind != "Config" {
		return fmt.Errorf("invalid kind: %q (expected \"Config\")", kc.Kind)
	}
	if len(kc.Clusters) == 0 {
		return fmt.Errorf("kubeconfig has no clusters defined")
	}
	if len(kc.Contexts) == 0 {
		return fmt.Errorf("kubeconfig has no contexts defined")
	}
	if len(kc.Users) == 0 {
		return fmt.Errorf("kubeconfig has no users defined")
	}

	// Validate cluster entries have minimum required fields.
	for i, c := range kc.Clusters {
		if c.Name == "" {
			return fmt.Errorf("cluster[%d]: missing name", i)
		}
		if c.Cluster.Server == "" {
			return fmt.Errorf("cluster[%d] %q: missing server", i, c.Name)
		}
	}

	// Validate context entries reference cluster and user.
	for i, ctx := range kc.Contexts {
		if ctx.Name == "" {
			return fmt.Errorf("context[%d]: missing name", i)
		}
		if ctx.Context.Cluster == "" {
			return fmt.Errorf("context[%d] %q: missing cluster reference", i, ctx.Name)
		}
		if ctx.Context.User == "" {
			return fmt.Errorf("context[%d] %q: missing user reference", i, ctx.Name)
		}
	}

	// Validate user entries have names.
	for i, u := range kc.Users {
		if u.Name == "" {
			return fmt.Errorf("user[%d]: missing name", i)
		}
	}

	return nil
}

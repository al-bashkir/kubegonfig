// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

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
	APIVersion     string         `yaml:"apiVersion"`
	Kind           string         `yaml:"kind"`
	Clusters       []clusterEntry `yaml:"clusters"`
	Contexts       []contextEntry `yaml:"contexts"`
	CurrentContext string         `yaml:"current-context"`
	Users          []userEntry    `yaml:"users"`
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
	clusterNames := make(map[string]struct{}, len(kc.Clusters))
	for i, c := range kc.Clusters {
		if c.Name == "" {
			return fmt.Errorf("cluster[%d]: missing name", i)
		}
		if _, ok := clusterNames[c.Name]; ok {
			return fmt.Errorf("cluster[%d] %q: duplicate name", i, c.Name)
		}
		if c.Cluster.Server == "" {
			return fmt.Errorf("cluster[%d] %q: missing server", i, c.Name)
		}
		clusterNames[c.Name] = struct{}{}
	}

	// Validate user entries have names before contexts reference them.
	userNames := make(map[string]struct{}, len(kc.Users))
	for i, u := range kc.Users {
		if u.Name == "" {
			return fmt.Errorf("user[%d]: missing name", i)
		}
		if _, ok := userNames[u.Name]; ok {
			return fmt.Errorf("user[%d] %q: duplicate name", i, u.Name)
		}
		userNames[u.Name] = struct{}{}
	}

	// Validate context entries reference defined clusters and users.
	contextNames := make(map[string]struct{}, len(kc.Contexts))
	for i, ctx := range kc.Contexts {
		if ctx.Name == "" {
			return fmt.Errorf("context[%d]: missing name", i)
		}
		if _, ok := contextNames[ctx.Name]; ok {
			return fmt.Errorf("context[%d] %q: duplicate name", i, ctx.Name)
		}
		contextNames[ctx.Name] = struct{}{}
		if ctx.Context.Cluster == "" {
			return fmt.Errorf("context[%d] %q: missing cluster reference", i, ctx.Name)
		}
		if _, ok := clusterNames[ctx.Context.Cluster]; !ok {
			return fmt.Errorf("context[%d] %q: unknown cluster reference %q", i, ctx.Name, ctx.Context.Cluster)
		}
		if ctx.Context.User == "" {
			return fmt.Errorf("context[%d] %q: missing user reference", i, ctx.Name)
		}
		if _, ok := userNames[ctx.Context.User]; !ok {
			return fmt.Errorf("context[%d] %q: unknown user reference %q", i, ctx.Name, ctx.Context.User)
		}
	}
	if kc.CurrentContext != "" {
		if _, ok := contextNames[kc.CurrentContext]; !ok {
			return fmt.Errorf("current-context %q: unknown context", kc.CurrentContext)
		}
	}

	return nil
}

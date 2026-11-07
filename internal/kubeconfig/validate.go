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

	if err := checkRequired(&kc); err != nil {
		return err
	}
	clusterNames, err := checkClusters(kc.Clusters)
	if err != nil {
		return err
	}
	// Users are checked before contexts reference them.
	userNames, err := checkUsers(kc.Users)
	if err != nil {
		return err
	}
	contextNames, err := checkContexts(kc.Contexts, clusterNames, userNames)
	if err != nil {
		return err
	}
	if kc.CurrentContext != "" {
		if _, ok := contextNames[kc.CurrentContext]; !ok {
			return fmt.Errorf("current-context %q: unknown context", kc.CurrentContext)
		}
	}
	return nil
}

// checkRequired verifies the header fields and that each section is non-empty.
func checkRequired(kc *kubeConfig) error {
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
	return nil
}

// checkClusters verifies cluster entries and returns the set of their names.
func checkClusters(clusters []clusterEntry) (map[string]struct{}, error) {
	names := make(map[string]struct{}, len(clusters))
	for i, c := range clusters {
		if err := claimName(names, "cluster", i, c.Name); err != nil {
			return nil, err
		}
		if c.Cluster.Server == "" {
			return nil, fmt.Errorf("cluster[%d] %q: missing server", i, c.Name)
		}
	}
	return names, nil
}

// claimName adds name to names, rejecting blank and duplicate names.
func claimName(names map[string]struct{}, kind string, i int, name string) error {
	if name == "" {
		return fmt.Errorf("%s[%d]: missing name", kind, i)
	}
	if _, ok := names[name]; ok {
		return fmt.Errorf("%s[%d] %q: duplicate name", kind, i, name)
	}
	names[name] = struct{}{}
	return nil
}

// checkUsers verifies user entries and returns the set of their names.
func checkUsers(users []userEntry) (map[string]struct{}, error) {
	names := make(map[string]struct{}, len(users))
	for i, u := range users {
		if err := claimName(names, "user", i, u.Name); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// checkContexts verifies context entries reference defined clusters and users
// and returns the set of their names.
func checkContexts(contexts []contextEntry, clusterNames, userNames map[string]struct{}) (map[string]struct{}, error) {
	names := make(map[string]struct{}, len(contexts))
	for i, ctx := range contexts {
		if err := claimName(names, "context", i, ctx.Name); err != nil {
			return nil, err
		}
		if err := checkContextRefs(i, ctx, clusterNames, userNames); err != nil {
			return nil, err
		}
	}
	return names, nil
}

func checkContextRefs(i int, ctx contextEntry, clusterNames, userNames map[string]struct{}) error {
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
	return nil
}

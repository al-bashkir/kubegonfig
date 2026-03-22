package kubeconfig

import (
	"strings"
	"testing"
)

// validKubeconfig is a minimal valid kubeconfig for tests.
const validKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
users:
- name: test-user
`

func TestValidate_ValidConfig(t *testing.T) {
	err := Validate([]byte(validKubeconfig))
	if err != nil {
		t.Errorf("Validate(valid) returned error: %v", err)
	}
}

func TestValidate_Empty(t *testing.T) {
	err := Validate([]byte{})
	if err == nil {
		t.Error("Validate(empty) should return error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}

func TestValidate_InvalidYAML(t *testing.T) {
	err := Validate([]byte("not: [yaml: bad"))
	if err == nil {
		t.Error("Validate(invalid YAML) should return error")
	}
	if !strings.Contains(err.Error(), "invalid YAML") {
		t.Errorf("expected 'invalid YAML' in error, got: %v", err)
	}
}

func TestValidate_MissingAPIVersion(t *testing.T) {
	data := `kind: Config
clusters:
- name: c
  cluster:
    server: https://localhost
contexts:
- name: ctx
  context:
    cluster: c
    user: u
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(no apiVersion) should return error")
	}
	if !strings.Contains(err.Error(), "apiVersion") {
		t.Errorf("expected 'apiVersion' in error, got: %v", err)
	}
}

func TestValidate_WrongAPIVersion(t *testing.T) {
	data := `apiVersion: v2
kind: Config
clusters:
- name: c
  cluster:
    server: https://localhost
contexts:
- name: ctx
  context:
    cluster: c
    user: u
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(v2) should return error")
	}
	if !strings.Contains(err.Error(), "unsupported apiVersion") {
		t.Errorf("expected 'unsupported apiVersion' in error, got: %v", err)
	}
}

func TestValidate_WrongKind(t *testing.T) {
	data := `apiVersion: v1
kind: Secret
clusters:
- name: c
  cluster:
    server: https://localhost
contexts:
- name: ctx
  context:
    cluster: c
    user: u
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(wrong kind) should return error")
	}
	if !strings.Contains(err.Error(), "invalid kind") {
		t.Errorf("expected 'invalid kind' in error, got: %v", err)
	}
}

func TestValidate_NoClusters(t *testing.T) {
	data := `apiVersion: v1
kind: Config
clusters: []
contexts:
- name: ctx
  context:
    cluster: c
    user: u
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(no clusters) should return error")
	}
	if !strings.Contains(err.Error(), "no clusters") {
		t.Errorf("expected 'no clusters' in error, got: %v", err)
	}
}

func TestValidate_NoContexts(t *testing.T) {
	data := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://localhost
contexts: []
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(no contexts) should return error")
	}
	if !strings.Contains(err.Error(), "no contexts") {
		t.Errorf("expected 'no contexts' in error, got: %v", err)
	}
}

func TestValidate_NoUsers(t *testing.T) {
	data := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://localhost
contexts:
- name: ctx
  context:
    cluster: c
    user: u
users: []`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(no users) should return error")
	}
	if !strings.Contains(err.Error(), "no users") {
		t.Errorf("expected 'no users' in error, got: %v", err)
	}
}

func TestValidate_ClusterMissingName(t *testing.T) {
	data := `apiVersion: v1
kind: Config
clusters:
- name: ""
  cluster:
    server: https://localhost
contexts:
- name: ctx
  context:
    cluster: c
    user: u
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(cluster missing name) should return error")
	}
	if !strings.Contains(err.Error(), "cluster[0]: missing name") {
		t.Errorf("expected 'cluster[0]: missing name' in error, got: %v", err)
	}
}

func TestValidate_ClusterMissingServer(t *testing.T) {
	data := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: ""
contexts:
- name: ctx
  context:
    cluster: c
    user: u
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(cluster missing server) should return error")
	}
	if !strings.Contains(err.Error(), "missing server") {
		t.Errorf("expected 'missing server' in error, got: %v", err)
	}
}

func TestValidate_ContextMissingCluster(t *testing.T) {
	data := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://localhost
contexts:
- name: ctx
  context:
    cluster: ""
    user: u
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(context missing cluster ref) should return error")
	}
	if !strings.Contains(err.Error(), "missing cluster reference") {
		t.Errorf("expected 'missing cluster reference' in error, got: %v", err)
	}
}

func TestValidate_ContextMissingUser(t *testing.T) {
	data := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://localhost
contexts:
- name: ctx
  context:
    cluster: c
    user: ""
users:
- name: u`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(context missing user ref) should return error")
	}
	if !strings.Contains(err.Error(), "missing user reference") {
		t.Errorf("expected 'missing user reference' in error, got: %v", err)
	}
}

func TestValidate_UserMissingName(t *testing.T) {
	data := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://localhost
contexts:
- name: ctx
  context:
    cluster: c
    user: u
users:
- name: ""`
	err := Validate([]byte(data))
	if err == nil {
		t.Error("Validate(user missing name) should return error")
	}
	if !strings.Contains(err.Error(), "user[0]: missing name") {
		t.Errorf("expected 'user[0]: missing name' in error, got: %v", err)
	}
}

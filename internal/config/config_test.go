package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	content := `
workspace = "/tmp/project"
image = "custom:v1"
namespace = "dev"
kubecontext = "kind-test"
tools = ["go"]
env_vars = ["API_KEY"]

[resources]
cpu = "4"
memory = "8Gi"

[[credentials]]
local = "/home/user/.ssh/id_rsa"
remote = "/home/coder/.ssh/id_rsa"
`
	path := writeTempFile(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Workspace != "/tmp/project" {
		t.Errorf("workspace = %q, want /tmp/project", cfg.Workspace)
	}
	if cfg.Image != "custom:v1" {
		t.Errorf("image = %q, want custom:v1", cfg.Image)
	}
	if cfg.Namespace != "dev" {
		t.Errorf("namespace = %q, want dev", cfg.Namespace)
	}
	if cfg.Resources.CPU != "4" {
		t.Errorf("cpu = %q, want 4", cfg.Resources.CPU)
	}
	if cfg.Resources.Memory != "8Gi" {
		t.Errorf("memory = %q, want 8Gi", cfg.Resources.Memory)
	}
	if len(cfg.Credentials) != 1 {
		t.Fatalf("credentials len = %d, want 1", len(cfg.Credentials))
	}
	if cfg.Credentials[0].Local != "/home/user/.ssh/id_rsa" {
		t.Errorf("credential local = %q", cfg.Credentials[0].Local)
	}
}

func TestLoadDefaults(t *testing.T) {
	content := `workspace = "/tmp/project"`
	path := writeTempFile(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Image != "ghcr.io/007/yolopod-base:latest" {
		t.Errorf("default image = %q", cfg.Image)
	}
	if cfg.Namespace != "default" {
		t.Errorf("default namespace = %q", cfg.Namespace)
	}
	if cfg.Resources.CPU != "2" {
		t.Errorf("default cpu = %q", cfg.Resources.CPU)
	}
	if cfg.Resources.Memory != "4Gi" {
		t.Errorf("default memory = %q", cfg.Resources.Memory)
	}
}

func TestLoadMissingWorkspace(t *testing.T) {
	content := `image = "test:v1"`
	path := writeTempFile(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}

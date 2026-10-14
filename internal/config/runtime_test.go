package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDockerAndKubernetesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8091\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NMONITOR_CONFIG", path)
	t.Setenv("NMONITOR_DOCKER_ENABLED", "")
	t.Setenv("NMONITOR_KUBERNETES_ENABLED", "")
	t.Setenv("NMONITOR_KUBE_CONTEXT", "")
	t.Setenv("NMONITOR_KUBECONFIG", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Docker.Enabled || !cfg.Kubernetes.Enabled || cfg.Kubernetes.Context != "" {
		t.Fatalf("defaults %+v %+v", cfg.Docker, cfg.Kubernetes)
	}

	if err := os.WriteFile(path, []byte("docker:\n  enabled: false\nkubernetes:\n  enabled: false\n  context: lab\n  kubeconfig: /tmp/kube.config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Docker.Enabled || cfg.Kubernetes.Enabled || cfg.Kubernetes.Context != "lab" || cfg.Kubernetes.Kubeconfig != "/tmp/kube.config" {
		t.Fatalf("yaml %+v %+v", cfg.Docker, cfg.Kubernetes)
	}

	t.Setenv("NMONITOR_DOCKER_ENABLED", "true")
	t.Setenv("NMONITOR_KUBERNETES_ENABLED", "yes")
	t.Setenv("NMONITOR_KUBE_CONTEXT", " prod ")
	t.Setenv("NMONITOR_KUBECONFIG", "/opt/kube/config")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Docker.Enabled || !cfg.Kubernetes.Enabled || cfg.Kubernetes.Context != "prod" || cfg.Kubernetes.Kubeconfig != "/opt/kube/config" {
		t.Fatalf("env %+v %+v", cfg.Docker, cfg.Kubernetes)
	}
}

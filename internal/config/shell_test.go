package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShellConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8091\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NMONITOR_CONFIG", path)
	t.Setenv("NMONITOR_SHELL_ENABLED", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Shell.Enabled || cfg.Shell.MaxSessions != 8 || cfg.Shell.IdleTimeout != 15*time.Minute || cfg.Shell.MaxLifetime != 4*time.Hour {
		t.Fatalf("defaults %+v", cfg.Shell)
	}

	if err := os.WriteFile(path, []byte("shell:\n  enabled: false\n  idle_timeout: 1s\n  max_lifetime: 1s\n  max_sessions: 100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Shell.Enabled || cfg.Shell.IdleTimeout != 15*time.Minute || cfg.Shell.MaxLifetime != 4*time.Hour || cfg.Shell.MaxSessions != 32 {
		t.Fatalf("clamped %+v", cfg.Shell)
	}

	t.Setenv("NMONITOR_SHELL_ENABLED", "true")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Shell.Enabled {
		t.Fatal("env should enable shell")
	}
}

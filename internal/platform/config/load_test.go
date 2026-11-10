package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBaseOnlyWithoutProfile(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "application.yaml", "server:\n  port: 9090\n")
	paths, err := ResolvePaths(Paths{BaseDir: t.TempDir(), ConfigDir: dir, DataDir: t.TempDir(), Profile: ""})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("port %d", cfg.Server.Port)
	}
	if cfg.App.Profile != "" {
		t.Fatalf("profile %q", cfg.App.Profile)
	}
	// SQLite 相对 path 相对 data-dir 解析为绝对路径。
	if !filepath.IsAbs(cfg.Database.Path) {
		t.Fatalf("db path %q", cfg.Database.Path)
	}
}

func TestLoadProfileOverlay(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "application.yaml", "server:\n  host: 127.0.0.1\n  port: 8080\nlogging:\n  level: info\n")
	writeConfig(t, dir, "application-pro.yaml", "server:\n  host: 0.0.0.0\n")
	paths, err := ResolvePaths(Paths{BaseDir: t.TempDir(), ConfigDir: dir, DataDir: t.TempDir(), Profile: "pro"})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Host != "0.0.0.0" || cfg.Server.Port != 8080 {
		t.Fatalf("overlay %+v", cfg.Server)
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("base lost: %s", cfg.Logging.Level)
	}
}

func TestLoadRejectsUnknownProfileAndMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "application.yaml", "server:\n  port: 8080\n")
	if _, err := ResolvePaths(Paths{BaseDir: dir, ConfigDir: dir, DataDir: dir, Profile: "staging"}); err == nil {
		t.Fatal("unknown profile should fail")
	}
	paths, err := ResolvePaths(Paths{BaseDir: dir, ConfigDir: dir, DataDir: dir, Profile: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(paths); err == nil {
		t.Fatal("missing profile file should fail")
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "application.yaml", "server:\n  port: 8080\nnot_a_section:\n  x: 1\n")
	paths, err := ResolvePaths(Paths{BaseDir: dir, ConfigDir: dir, DataDir: dir, Profile: ""})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(paths); err == nil {
		t.Fatal("unknown key should fail")
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "application.yaml", "server:\n  port: 8080\n")
	t.Setenv("APP_SERVER_PORT", "9443")
	paths, err := ResolvePaths(Paths{BaseDir: dir, ConfigDir: dir, DataDir: dir, Profile: ""})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9443 {
		t.Fatalf("env port %d", cfg.Server.Port)
	}
}

func TestLegacyEnvAlias(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "application.yaml", "security:\n  username: admin\n")
	t.Setenv("NMONITOR_PASSWORD", "legacy-pass")
	paths, err := ResolvePaths(Paths{BaseDir: dir, ConfigDir: dir, DataDir: dir, Profile: ""})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Security.Password != "legacy-pass" {
		t.Fatalf("legacy alias %q", cfg.Security.Password)
	}
}

func TestNormalizeValidatesDriver(t *testing.T) {
	cfg := Default()
	cfg.Database.Driver = "oracle"
	if err := cfg.Normalize(); err == nil {
		t.Fatal("unknown driver should fail")
	}
	cfg = Default()
	cfg.Database.Driver = "mysql"
	cfg.Database.DSN = ""
	if err := cfg.Normalize(); err == nil {
		t.Fatal("network driver without dsn should fail")
	}
	cfg.Database.DSN = "user:pass@tcp(127.0.0.1)/db"
	// 未显式配置连接池时，网络库按默认容量初始化。
	cfg.Database.MaxOpenConns = 0
	cfg.Database.MaxIdleConns = 0
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.Database.MaxOpenConns != 10 {
		t.Fatalf("network pool default %d", cfg.Database.MaxOpenConns)
	}
}

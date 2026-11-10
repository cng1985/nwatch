// Package config 负责配置加载、覆盖与验证。
// 所有运行时 YAML 统一放在 configs/ 目录：application.yaml 是基础配置，
// application-dev.yaml / application-pro.yaml 是 profile 差异配置。
// 覆盖顺序：基础配置 < profile 配置 < 环境变量 < 命令行。
package config

import (
	"fmt"
	"strings"
	"time"
)

// Config 是进程配置的唯一来源。App 段是启动定位信息，由 app.Run 解析后传入。
type Config struct {
	App        AppConfig        `yaml:"app"`
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Scheduler  SchedulerConfig  `yaml:"scheduler"`
	Monitor    MonitorConfig    `yaml:"monitor"`
	History    HistoryConfig    `yaml:"history"`
	Security   SecurityConfig   `yaml:"security"`
	Logging    LoggingConfig    `yaml:"logging"`
	Backup     BackupConfig     `yaml:"backup"`
	Mail       MailConfig       `yaml:"mail"`
	Shell      ShellConfig      `yaml:"shell"`
	Service    ServiceConfig    `yaml:"service"`
	Docker     DockerConfig     `yaml:"docker"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
}

// AppConfig 记录启动定位参数。这些值先于 YAML 确定，YAML 中仅作展示且必须一致。
type AppConfig struct {
	Name      string `yaml:"name"`
	Profile   string `yaml:"profile"`
	BaseDir   string `yaml:"base_dir"`
	ConfigDir string `yaml:"config_dir"`
	DataDir   string `yaml:"data_dir"`
}

type ServerConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes    int           `yaml:"max_header_bytes"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	TrustedProxies    []string      `yaml:"trusted_proxies"`
}

type DatabaseConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Driver          string        `yaml:"driver"`
	DSN             string        `yaml:"dsn"`
	Path            string        `yaml:"path"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	BatchSize       int           `yaml:"batch_size"`
	SQLite          SQLiteConfig  `yaml:"sqlite"`
}

// SQLiteConfig 逐连接落实外键、忙等待与日志模式。
type SQLiteConfig struct {
	ForeignKeys bool          `yaml:"foreign_keys"`
	BusyTimeout time.Duration `yaml:"busy_timeout"`
	JournalMode string        `yaml:"journal_mode"`
}

type SchedulerConfig struct {
	WorkerCount  int           `yaml:"worker_count"`
	ScanInterval time.Duration `yaml:"scan_interval"`
	QueueSize    int           `yaml:"queue_size"`
}

type MonitorConfig struct {
	DefaultTimeout  time.Duration `yaml:"default_timeout"`
	DefaultInterval time.Duration `yaml:"default_interval"`
}

type HistoryConfig struct {
	CheckRetentionDays      int `yaml:"check_retention_days"`
	EventRetentionDays      int `yaml:"event_retention_days"`
	MetricRetentionDays     int `yaml:"metric_retention_days"`
	HourMetricRetentionDays int `yaml:"hour_metric_retention_days"`
}

type SecurityConfig struct {
	Username  string        `yaml:"username"`
	Password  string        `yaml:"password"`
	JWTSecret string        `yaml:"jwt_secret"`
	JWTTTL    time.Duration `yaml:"jwt_ttl"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type BackupConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Dir           string `yaml:"dir"`
	RetentionDays int    `yaml:"retention_days"`
	Hour          int    `yaml:"hour"`
}

type MailConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	From       string `yaml:"from"`
	To         string `yaml:"to"`
	Encryption string `yaml:"encryption"`
}

// ServiceConfig 控制管理台里的 systemd 服务管理。
type ServiceConfig struct {
	Enabled bool `yaml:"enabled"`
}

// DockerConfig 控制管理台里的本机 Docker 管理。
type DockerConfig struct {
	Enabled bool `yaml:"enabled"`
}

// KubernetesConfig 控制管理台里的 kubectl 管理。Context 和 Kubeconfig 为空时使用本机默认配置。
type KubernetesConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Context    string `yaml:"context"`
	Kubeconfig string `yaml:"kubeconfig"`
}

// ShellConfig 控制管理台里的交互式终端。
type ShellConfig struct {
	Enabled     bool          `yaml:"enabled"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
	MaxLifetime time.Duration `yaml:"max_lifetime"`
	MaxSessions int           `yaml:"max_sessions"`
}

// Default 返回全量默认值。单元测试和未提供配置文件时使用。
func Default() *Config {
	return &Config{
		App: AppConfig{Name: "nwatch"},
		Server: ServerConfig{
			Host:              "127.0.0.1",
			Port:              8080,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 20,
			ShutdownTimeout:   30 * time.Second,
			TrustedProxies:    []string{},
		},
		Database: DatabaseConfig{
			Enabled:         true,
			Driver:          "sqlite",
			Path:            "app.db",
			MaxOpenConns:    8,
			MaxIdleConns:    4,
			ConnMaxLifetime: time.Hour,
			BatchSize:       100,
			SQLite: SQLiteConfig{
				ForeignKeys: true,
				BusyTimeout: 5 * time.Second,
				JournalMode: "WAL",
			},
		},
		Scheduler: SchedulerConfig{WorkerCount: 20, ScanInterval: time.Second, QueueSize: 200},
		Monitor:   MonitorConfig{DefaultTimeout: 5 * time.Second, DefaultInterval: 60 * time.Second},
		History: HistoryConfig{
			CheckRetentionDays:      7,
			EventRetentionDays:      365,
			MetricRetentionDays:     90,
			HourMetricRetentionDays: 730,
		},
		Security: SecurityConfig{Username: "admin", JWTTTL: 24 * time.Hour},
		Logging:  LoggingConfig{Level: "info", Format: "json"},
		Backup:   BackupConfig{Enabled: true, Dir: "./data/backups", RetentionDays: 7, Hour: 3},
		Mail:     MailConfig{Port: 587, Encryption: "starttls"},
		Shell: ShellConfig{
			Enabled:     true,
			IdleTimeout: 15 * time.Minute,
			MaxLifetime: 4 * time.Hour,
			MaxSessions: 8,
		},
		Service:    ServiceConfig{Enabled: true},
		Docker:     DockerConfig{Enabled: true},
		Kubernetes: KubernetesConfig{Enabled: true},
	}
}

// Addr 返回 HTTP 监听地址。
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Normalize 补全默认值并校验非法配置。返回首个错误。
func (c *Config) Normalize() error {
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 需在 1—65535 之间")
	}
	if c.Server.ReadHeaderTimeout <= 0 {
		c.Server.ReadHeaderTimeout = 5 * time.Second
	}
	if c.Server.IdleTimeout <= 0 {
		c.Server.IdleTimeout = 60 * time.Second
	}
	if c.Server.MaxHeaderBytes <= 0 {
		c.Server.MaxHeaderBytes = 1 << 20
	}
	if c.Server.ShutdownTimeout <= 0 {
		c.Server.ShutdownTimeout = 30 * time.Second
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	switch c.Database.Driver {
	case "sqlite", "mysql", "postgres", "pgsql", "sqlserver":
	default:
		return fmt.Errorf("database.driver 仅支持 sqlite、mysql、postgres、sqlserver")
	}
	if !c.Database.Enabled {
		return c.normalizeCommon()
	}
	if c.Database.Driver != "sqlite" && c.Database.DSN == "" {
		return fmt.Errorf("网络数据库必须在 database.dsn 或 APP_DATABASE_DSN 提供连接串")
	}
	if c.Database.Path == "" {
		c.Database.Path = "app.db"
	}
	if c.Database.MaxOpenConns <= 0 {
		if c.Database.Driver == "sqlite" {
			c.Database.MaxOpenConns = 1
		} else {
			c.Database.MaxOpenConns = 10
		}
	}
	if c.Database.MaxIdleConns <= 0 {
		if c.Database.Driver == "sqlite" {
			c.Database.MaxIdleConns = 1
		} else {
			c.Database.MaxIdleConns = 5
		}
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		c.Database.MaxIdleConns = c.Database.MaxOpenConns
	}
	if c.Database.ConnMaxLifetime <= 0 {
		c.Database.ConnMaxLifetime = 30 * time.Minute
	}
	if c.Database.BatchSize <= 0 {
		c.Database.BatchSize = 100
	}
	if c.Database.SQLite.BusyTimeout <= 0 {
		c.Database.SQLite.BusyTimeout = 5 * time.Second
	}
	if c.Database.SQLite.JournalMode == "" {
		// 记录的默认值覆盖：既有部署依赖 WAL，保留 WAL 为项目默认（见 docs/decisions.md）。
		c.Database.SQLite.JournalMode = "WAL"
	}
	return c.normalizeCommon()
}

func (c *Config) normalizeCommon() error {
	if c.Scheduler.WorkerCount <= 0 {
		c.Scheduler.WorkerCount = 20
	}
	if c.Scheduler.ScanInterval <= 0 {
		c.Scheduler.ScanInterval = time.Second
	}
	if c.Scheduler.QueueSize <= 0 {
		c.Scheduler.QueueSize = 200
	}
	if c.Monitor.DefaultTimeout <= 0 {
		c.Monitor.DefaultTimeout = 5 * time.Second
	}
	if c.Monitor.DefaultInterval <= 0 {
		c.Monitor.DefaultInterval = 60 * time.Second
	}
	if c.History.CheckRetentionDays <= 0 {
		c.History.CheckRetentionDays = 7
	}
	if c.History.EventRetentionDays <= 0 {
		c.History.EventRetentionDays = 365
	}
	if c.History.MetricRetentionDays <= 0 {
		c.History.MetricRetentionDays = 90
	}
	if c.History.HourMetricRetentionDays <= 0 {
		c.History.HourMetricRetentionDays = 730
	}
	if c.Security.Username == "" {
		c.Security.Username = "admin"
	}
	if c.Security.JWTTTL <= 0 {
		c.Security.JWTTTL = 24 * time.Hour
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("logging.level 仅支持 debug、info、warn、error")
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
	if c.Backup.Dir == "" {
		c.Backup.Dir = "./data/backups"
	}
	if c.Backup.RetentionDays <= 0 {
		c.Backup.RetentionDays = 7
	}
	switch lower := strings.TrimSpace(strings.ToLower(c.Mail.Encryption)); lower {
	case "ssl", "tls", "smtps":
		c.Mail.Encryption = "ssl"
	case "none", "plain", "off":
		c.Mail.Encryption = "none"
	default:
		c.Mail.Encryption = "starttls"
	}
	if c.Mail.Port <= 0 {
		if c.Mail.Encryption == "ssl" {
			c.Mail.Port = 465
		} else {
			c.Mail.Port = 587
		}
	}
	if c.Shell.IdleTimeout < time.Minute {
		c.Shell.IdleTimeout = 15 * time.Minute
	}
	if c.Shell.MaxLifetime < 10*time.Minute {
		c.Shell.MaxLifetime = 4 * time.Hour
	}
	if c.Shell.MaxLifetime < c.Shell.IdleTimeout {
		c.Shell.MaxLifetime = c.Shell.IdleTimeout
	}
	if c.Shell.MaxSessions <= 0 {
		c.Shell.MaxSessions = 8
	}
	if c.Shell.MaxSessions > 32 {
		c.Shell.MaxSessions = 32
	}
	c.Kubernetes.Context = strings.TrimSpace(c.Kubernetes.Context)
	c.Kubernetes.Kubeconfig = strings.TrimSpace(c.Kubernetes.Kubeconfig)
	return nil
}

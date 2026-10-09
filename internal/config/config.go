package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Monitor   MonitorConfig   `yaml:"monitor"`
	History   HistoryConfig   `yaml:"history"`
	Security  SecurityConfig  `yaml:"security"`
	Log       LogConfig       `yaml:"log"`
	Backup    BackupConfig    `yaml:"backup"`
	Mail      MailConfig      `yaml:"mail"`

	ConfigPath string `yaml:"-"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
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

type LogConfig struct {
	Level string `yaml:"level"`
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

func Default() *Config {
	return &Config{
		Server:    ServerConfig{Host: "0.0.0.0", Port: 8080},
		Database:  DatabaseConfig{Path: "./data/nmonitor.db"},
		Scheduler: SchedulerConfig{WorkerCount: 20, ScanInterval: time.Second, QueueSize: 200},
		Monitor:   MonitorConfig{DefaultTimeout: 5 * time.Second, DefaultInterval: 60 * time.Second},
		History: HistoryConfig{
			CheckRetentionDays:      7,
			EventRetentionDays:      365,
			MetricRetentionDays:     90,
			HourMetricRetentionDays: 730,
		},
		Security: SecurityConfig{Username: "admin", JWTTTL: 24 * time.Hour},
		Log:      LogConfig{Level: "info"},
		Backup:   BackupConfig{Enabled: true, Dir: "./data/backups", RetentionDays: 7, Hour: 3},
		Mail:     MailConfig{Port: 587, Encryption: "starttls"},
	}
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func Load() (*Config, error) {
	path := os.Getenv("NMONITOR_CONFIG")
	if path == "" {
		for _, candidate := range []string{"config.yaml", "/opt/nmonitor/config.yaml"} {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	cfg := Default()
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取配置 %s: %w", path, err)
		}
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("解析配置: %w", err)
		}
		cfg.ConfigPath = path
	}
	cfg.applyEnv()
	cfg.normalize()
	return cfg, nil
}

func (c *Config) applyEnv() {
	if v := os.Getenv("NMONITOR_PASSWORD"); v != "" {
		c.Security.Password = v
	}
	if v := os.Getenv("NMONITOR_JWT_SECRET"); v != "" {
		c.Security.JWTSecret = v
	}
	if v := os.Getenv("NMONITOR_USERNAME"); v != "" {
		c.Security.Username = v
	}
	if v := os.Getenv("NMONITOR_SMTP_ENABLED"); v != "" {
		c.Mail.Enabled = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}
	if v := os.Getenv("NMONITOR_SMTP_HOST"); v != "" {
		c.Mail.Host = v
	}
	if v := os.Getenv("NMONITOR_SMTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Mail.Port = n
		}
	}
	if v := os.Getenv("NMONITOR_SMTP_USERNAME"); v != "" {
		c.Mail.Username = v
	}
	if v := os.Getenv("NMONITOR_SMTP_PASSWORD"); v != "" {
		c.Mail.Password = v
	}
	if v := os.Getenv("NMONITOR_SMTP_FROM"); v != "" {
		c.Mail.From = v
	}
	if v := os.Getenv("NMONITOR_SMTP_TO"); v != "" {
		c.Mail.To = v
	}
	if v := os.Getenv("NMONITOR_SMTP_ENCRYPTION"); v != "" {
		c.Mail.Encryption = v
	}
}

func (c *Config) normalize() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Database.Path == "" {
		c.Database.Path = "./data/nmonitor.db"
	}
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
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Backup.Dir == "" {
		c.Backup.Dir = "./data/backups"
	}
	if c.Backup.RetentionDays <= 0 {
		c.Backup.RetentionDays = 7
	}
	switch strings.ToLower(strings.TrimSpace(c.Mail.Encryption)) {
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
}

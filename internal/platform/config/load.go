package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Paths 是启动定位参数：先从命令行/环境变量/默认值确定，再加载 YAML。
type Paths struct {
	BaseDir   string // 默认启动工作目录，固定为绝对路径
	ConfigDir string // 默认 configs，相对 base-dir
	DataDir   string // 默认 ./data，相对 base-dir
	Profile   string // 空表示只加载基础配置；仅 dev/pro
}

// ResolvePaths 按命令行 > 环境变量 > 默认值解析启动定位参数。
func ResolvePaths(explicit Paths) (Paths, error) {
	p := explicit
	if p.BaseDir == "" {
		p.BaseDir = os.Getenv("APP_BASE_DIR")
	}
	if p.BaseDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return p, fmt.Errorf("读取工作目录: %w", err)
		}
		p.BaseDir = wd
	}
	abs, err := filepath.Abs(p.BaseDir)
	if err != nil {
		return p, fmt.Errorf("解析 base-dir: %w", err)
	}
	p.BaseDir = abs
	if p.ConfigDir == "" {
		p.ConfigDir = os.Getenv("APP_CONFIG_DIR")
	}
	if p.ConfigDir == "" {
		p.ConfigDir = "configs"
	}
	if p.DataDir == "" {
		p.DataDir = os.Getenv("APP_DATA_DIR")
	}
	if p.DataDir == "" {
		p.DataDir = "./data"
	}
	if p.Profile == "" {
		p.Profile = os.Getenv("APP_PROFILE")
	}
	// 未指定 profile 时只加载基础配置；仅允许 dev/pro。
	switch p.Profile {
	case "", "dev", "pro":
	default:
		return p, fmt.Errorf("profile 仅支持 dev、pro，当前为 %q", p.Profile)
	}
	p.ConfigDir = resolveUnder(p.ConfigDir, p.BaseDir)
	p.DataDir = resolveUnder(p.DataDir, p.BaseDir)
	return p, nil
}

func resolveUnder(path, base string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}

// Load 读取基础配置与选中的 profile 配置，合并环境变量后校验。
// SQLite 相对 path 相对 data-dir；绝对路径保持不变。
func Load(paths Paths) (*Config, error) {
	baseFile := filepath.Join(paths.ConfigDir, "application.yaml")
	rawBase, err := os.ReadFile(baseFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取配置 %s: %w", baseFile, err)
		}
		// 记录的行为：未提供基础配置文件时使用内置默认值，便于源码直接运行。
		rawBase = nil
	}
	merged := map[string]any{}
	if rawBase != nil {
		if err := unmarshalInto(rawBase, merged, baseFile); err != nil {
			return nil, err
		}
	}
	if paths.Profile != "" {
		profileFile := filepath.Join(paths.ConfigDir, "application-"+paths.Profile+".yaml")
		rawProfile, err := os.ReadFile(profileFile)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("配置文件 %s 不存在：profile=%s", profileFile, paths.Profile)
			}
			return nil, fmt.Errorf("读取配置 %s: %w", profileFile, err)
		}
		overlay := map[string]any{}
		if err := unmarshalInto(rawProfile, overlay, profileFile); err != nil {
			return nil, err
		}
		mergeInto(merged, overlay)
	}

	cfg := Default()
	if len(merged) > 0 {
		raw, err := yaml.Marshal(merged)
		if err != nil {
			return nil, fmt.Errorf("合并配置: %w", err)
		}
		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		dec.KnownFields(true)
		if err := dec.Decode(cfg); err != nil {
			return nil, fmt.Errorf("解析配置（含未知键校验）: %w", err)
		}
	}

	cfg.App = AppConfig{
		Name:      appName(cfg.App.Name),
		Profile:   paths.Profile,
		BaseDir:   paths.BaseDir,
		ConfigDir: paths.ConfigDir,
		DataDir:   paths.DataDir,
	}
	cfg.applyEnv()
	if cfg.Database.Enabled {
		if cfg.Database.Driver == "sqlite" && !filepath.IsAbs(cfg.Database.Path) {
			cfg.Database.Path = filepath.Join(paths.DataDir, cfg.Database.Path)
		}
	}
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func appName(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return strings.TrimSpace(configured)
	}
	return "nwatch"
}

func unmarshalInto(raw []byte, out map[string]any, name string) error {
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("解析配置 %s: %w", name, err)
	}
	if m == nil {
		return nil
	}
	mergeInto(out, m)
	return nil
}

// mergeInto 递归合并映射：标量与列表按高优先级替换，false、0 和空列表不视为未配置。
func mergeInto(dst, src map[string]any) {
	for key, value := range src {
		if sub, ok := value.(map[string]any); ok {
			if dstSub, ok := dst[key].(map[string]any); ok {
				mergeInto(dstSub, sub)
				continue
			}
		}
		dst[key] = value
	}
}

// envBinding 是环境变量到配置项的显式登记。NMONITOR_* 是已登记的兼容别名。
type envBinding struct {
	env   string
	alias string
	apply func(*Config, string)
}

var envBindings = []envBinding{
	{"APP_SERVER_HOST", "NMONITOR_SERVER_HOST", func(c *Config, v string) { c.Server.Host = v }},
	{"APP_SERVER_PORT", "NMONITOR_SERVER_PORT", func(c *Config, v string) { setInt(v, &c.Server.Port) }},
	{"APP_DATABASE_ENABLED", "NMONITOR_DATABASE_ENABLED", func(c *Config, v string) { c.Database.Enabled = truthy(v) }},
	{"APP_DATABASE_DRIVER", "NMONITOR_DATABASE_DRIVER", func(c *Config, v string) { c.Database.Driver = v }},
	{"APP_DATABASE_DSN", "NMONITOR_DATABASE_DSN", func(c *Config, v string) { c.Database.DSN = v }},
	{"APP_DATABASE_BATCH_SIZE", "NMONITOR_DATABASE_BATCH_SIZE", func(c *Config, v string) { setInt(v, &c.Database.BatchSize) }},
	{"APP_ADMIN_USERNAME", "NMONITOR_USERNAME", func(c *Config, v string) { c.Security.Username = v }},
	{"APP_ADMIN_PASSWORD", "NMONITOR_PASSWORD", func(c *Config, v string) { c.Security.Password = v }},
	{"APP_JWT_SECRET", "NMONITOR_JWT_SECRET", func(c *Config, v string) { c.Security.JWTSecret = v }},
	{"APP_LOGGING_LEVEL", "NMONITOR_LOG_LEVEL", func(c *Config, v string) { c.Logging.Level = v }},
	{"APP_LOGGING_FORMAT", "NMONITOR_LOG_FORMAT", func(c *Config, v string) { c.Logging.Format = v }},
	{"APP_SMTP_ENABLED", "NMONITOR_SMTP_ENABLED", func(c *Config, v string) { c.Mail.Enabled = truthy(v) }},
	{"APP_SMTP_HOST", "NMONITOR_SMTP_HOST", func(c *Config, v string) { c.Mail.Host = v }},
	{"APP_SMTP_PORT", "NMONITOR_SMTP_PORT", func(c *Config, v string) { setInt(v, &c.Mail.Port) }},
	{"APP_SMTP_USERNAME", "NMONITOR_SMTP_USERNAME", func(c *Config, v string) { c.Mail.Username = v }},
	{"APP_SMTP_PASSWORD", "NMONITOR_SMTP_PASSWORD", func(c *Config, v string) { c.Mail.Password = v }},
	{"APP_SMTP_FROM", "NMONITOR_SMTP_FROM", func(c *Config, v string) { c.Mail.From = v }},
	{"APP_SMTP_TO", "NMONITOR_SMTP_TO", func(c *Config, v string) { c.Mail.To = v }},
	{"APP_SMTP_ENCRYPTION", "NMONITOR_SMTP_ENCRYPTION", func(c *Config, v string) { c.Mail.Encryption = v }},
	{"APP_SHELL_ENABLED", "NMONITOR_SHELL_ENABLED", func(c *Config, v string) { c.Shell.Enabled = truthy(v) }},
	{"APP_SERVICE_ENABLED", "NMONITOR_SERVICE_ENABLED", func(c *Config, v string) { c.Service.Enabled = truthy(v) }},
	{"APP_DOCKER_ENABLED", "NMONITOR_DOCKER_ENABLED", func(c *Config, v string) { c.Docker.Enabled = truthy(v) }},
	{"APP_KUBERNETES_ENABLED", "NMONITOR_KUBERNETES_ENABLED", func(c *Config, v string) { c.Kubernetes.Enabled = truthy(v) }},
	{"APP_KUBE_CONTEXT", "NMONITOR_KUBE_CONTEXT", func(c *Config, v string) { c.Kubernetes.Context = v }},
	{"APP_KUBECONFIG", "NMONITOR_KUBECONFIG", func(c *Config, v string) { c.Kubernetes.Kubeconfig = v }},
}

func (c *Config) applyEnv() {
	for _, binding := range envBindings {
		if v, ok := os.LookupEnv(binding.env); ok && v != "" {
			binding.apply(c, v)
			continue
		}
		if v, ok := os.LookupEnv(binding.alias); ok && v != "" {
			binding.apply(c, v)
		}
	}
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func setInt(v string, dst *int) {
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		*dst = n
	}
}

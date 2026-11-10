package settings

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cng1985/nwatch/internal/platform/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	KeyTimezone            = "timezone"
	KeyCheckRetention      = "check_retention_days"
	KeyEventRetention      = "event_retention_days"
	KeyMetricRetention     = "metric_retention_days"
	KeyHourMetricRetention = "hour_metric_retention_days"
	KeyDefaultTimeout      = "default_timeout"
	KeyDefaultInterval     = "default_interval"
	KeyPasswordHash        = "password_hash"
	KeySMTPEnabled         = "smtp_enabled"
	KeySMTPHost            = "smtp_host"
	KeySMTPPort            = "smtp_port"
	KeySMTPUsername        = "smtp_username"
	KeySMTPPassword        = "smtp_password"
	KeySMTPFrom            = "smtp_from"
	KeySMTPTo              = "smtp_to"
	KeySMTPEncryption      = "smtp_encryption"
)

type SMTPConfig struct {
	Enabled    bool
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	To         string
	Encryption string
}

type Store struct {
	mu  sync.RWMutex
	db  *gorm.DB
	cfg *config.Config

	Timezone                string
	CheckRetentionDays      int
	EventRetentionDays      int
	MetricRetentionDays     int
	HourMetricRetentionDays int
	DefaultTimeout          int
	DefaultInterval         int
	PasswordHash            string
	smtp                    SMTPConfig
}

func New(db *gorm.DB, cfg *config.Config) (*Store, error) {
	s := &Store{db: db, cfg: cfg}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	var rows []Setting
	if err := s.db.Find(&rows).Error; err != nil {
		return err
	}
	values := map[string]string{}
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Timezone = pick(values[KeyTimezone], "Asia/Shanghai")
	s.CheckRetentionDays = pickInt(values[KeyCheckRetention], s.cfg.History.CheckRetentionDays)
	s.EventRetentionDays = pickInt(values[KeyEventRetention], s.cfg.History.EventRetentionDays)
	s.MetricRetentionDays = pickInt(values[KeyMetricRetention], s.cfg.History.MetricRetentionDays)
	s.HourMetricRetentionDays = pickInt(values[KeyHourMetricRetention], s.cfg.History.HourMetricRetentionDays)
	s.DefaultTimeout = pickInt(values[KeyDefaultTimeout], int(s.cfg.Monitor.DefaultTimeout.Seconds()))
	s.DefaultInterval = pickInt(values[KeyDefaultInterval], int(s.cfg.Monitor.DefaultInterval.Seconds()))
	s.PasswordHash = values[KeyPasswordHash]
	s.smtp = SMTPConfig{
		Enabled:    pickBool(values, KeySMTPEnabled, s.cfg.Mail.Enabled),
		Host:       pickPresent(values, KeySMTPHost, s.cfg.Mail.Host),
		Port:       pickIntPresent(values, KeySMTPPort, s.cfg.Mail.Port),
		Username:   pickPresent(values, KeySMTPUsername, s.cfg.Mail.Username),
		Password:   pickPresent(values, KeySMTPPassword, s.cfg.Mail.Password),
		From:       pickPresent(values, KeySMTPFrom, s.cfg.Mail.From),
		To:         pickPresent(values, KeySMTPTo, s.cfg.Mail.To),
		Encryption: pickPresent(values, KeySMTPEncryption, s.cfg.Mail.Encryption),
	}
	if s.smtp.Port <= 0 {
		s.smtp.Port = 587
	}
	if s.smtp.Encryption == "" {
		s.smtp.Encryption = "starttls"
	}
	return nil
}

func (s *Store) SMTP() SMTPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.smtp
}

// Snapshot 是设置的一致性快照，不包含锁与数据库句柄。
type Snapshot struct {
	Timezone                string
	CheckRetentionDays      int
	EventRetentionDays      int
	MetricRetentionDays     int
	HourMetricRetentionDays int
	DefaultTimeout          int
	DefaultInterval         int
	PasswordHash            string
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{
		Timezone:                s.Timezone,
		CheckRetentionDays:      s.CheckRetentionDays,
		EventRetentionDays:      s.EventRetentionDays,
		MetricRetentionDays:     s.MetricRetentionDays,
		HourMetricRetentionDays: s.HourMetricRetentionDays,
		DefaultTimeout:          s.DefaultTimeout,
		DefaultInterval:         s.DefaultInterval,
		PasswordHash:            s.PasswordHash,
	}
}

func (s *Store) Location() *time.Location {
	s.mu.RLock()
	name := s.Timezone
	s.mu.RUnlock()
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Local
	}
	return loc
}

func (s *Store) Update(values map[string]string) error {
	now := time.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for k, v := range values {
			row := Setting{Key: k, Value: v, UpdatedAt: now}
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.load()
}

func (s *Store) SetPassword(plain string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.Update(map[string]string{KeyPasswordHash: string(hash)})
}

func (s *Store) CheckPassword(plain string) bool {
	s.mu.RLock()
	hash := s.PasswordHash
	fallback := s.cfg.Security.Password
	s.mu.RUnlock()
	if hash != "" {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
	}
	if fallback == "" {
		fallback = "admin"
	}
	return plain == fallback
}

func (s *Store) UsingDefaultPassword() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.PasswordHash != "" {
		return false
	}
	return s.cfg.Security.Password == "" || s.cfg.Security.Password == "admin"
}

func pick(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func pickPresent(values map[string]string, key, def string) string {
	v, ok := values[key]
	if !ok {
		return def
	}
	return v
}

func pickIntPresent(values map[string]string, key string, def int) int {
	v, ok := values[key]
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func pickBool(values map[string]string, key string, def bool) bool {
	v, ok := values[key]
	if !ok || v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func pickInt(v string, def int) int {
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Setting 是运行时可变的系统配置项。
type Setting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Setting) TableName() string { return "settings" }

// Package settings 提供系统设置的查询与修改业务，含邮件通道设置。
package settings

import (
	"github.com/cng1985/nwatch/internal/modules/notify/mailer"
	"github.com/cng1985/nwatch/internal/platform/settings"
)

// SettingsRequest 是系统设置修改参数。
type SettingsRequest struct {
	Timezone                string `json:"timezone"`
	CheckRetentionDays      int    `json:"checkRetentionDays"`
	EventRetentionDays      int    `json:"eventRetentionDays"`
	MetricRetentionDays     int    `json:"metricRetentionDays"`
	HourMetricRetentionDays int    `json:"hourMetricRetentionDays"`
	DefaultTimeout          int    `json:"defaultTimeout"`
	DefaultInterval         int    `json:"defaultInterval"`
}

// MailView 是邮件通道的展示视图，密码仅提示是否已设置。
type MailView struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	PasswordSet bool   `json:"passwordSet"`
	From        string `json:"from"`
	To          string `json:"to"`
	Encryption  string `json:"encryption"`
}

// SettingsResponse 是系统设置视图。
type SettingsResponse struct {
	Timezone                string   `json:"timezone"`
	CheckRetentionDays      int      `json:"checkRetentionDays"`
	EventRetentionDays      int      `json:"eventRetentionDays"`
	MetricRetentionDays     int      `json:"metricRetentionDays"`
	HourMetricRetentionDays int      `json:"hourMetricRetentionDays"`
	DefaultTimeout          int      `json:"defaultTimeout"`
	DefaultInterval         int      `json:"defaultInterval"`
	WorkerCount             int      `json:"workerCount"`
	ScanInterval            string   `json:"scanInterval"`
	UsingDefaultPassword    bool     `json:"usingDefaultPassword"`
	Version                 string   `json:"version"`
	Database                string   `json:"database"`
	Mail                    MailView `json:"mail"`
}

// MailRequest 是邮件通道修改与测试参数。
type MailRequest struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	From       string `json:"from"`
	To         string `json:"to"`
	Encryption string `json:"encryption"`
}

// MailTestResponse 是测试邮件结果。
type MailTestResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func toMailView(cfg settings.SMTPConfig) MailView {
	return MailView{
		Enabled:     cfg.Enabled,
		Host:        cfg.Host,
		Port:        cfg.Port,
		Username:    cfg.Username,
		PasswordSet: cfg.Password != "",
		From:        cfg.From,
		To:          cfg.To,
		Encryption:  cfg.Encryption,
	}
}

func toMailerConfig(req MailRequest, password string) mailer.Config {
	return mailer.Normalize(mailer.Config{
		Enabled:    req.Enabled,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		Password:   password,
		From:       req.From,
		To:         req.To,
		Encryption: req.Encryption,
	})
}

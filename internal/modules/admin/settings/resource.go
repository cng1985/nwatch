package settings

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/modules/jobs/cronjob"
	"github.com/cng1985/nwatch/internal/modules/notify/mailer"
	"github.com/cng1985/nwatch/internal/platform/config"
	apperrors "github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"github.com/cng1985/nwatch/internal/platform/version"
)

// Resource 编排系统设置业务。
type Resource struct {
	cfg   *config.Config
	store *settings.Store
	mail  *mailer.Service
	cron  *cronjob.Service
}

func NewResource(cfg *config.Config, store *settings.Store, mail *mailer.Service, cron *cronjob.Service) *Resource {
	return &Resource{cfg: cfg, store: store, mail: mail, cron: cron}
}

// Get 返回系统设置视图。
func (r *Resource) Get(ctx context.Context) (response.Response[SettingsResponse], error) {
	return response.OK(r.view()), nil
}

// Update 修改系统设置；时区变化后重算定时任务。
func (r *Resource) Update(ctx context.Context, request *SettingsRequest) (response.Response[SettingsResponse], error) {
	if request == nil {
		return response.Response[SettingsResponse]{}, apperrors.BadRequest("请求格式不正确")
	}
	if _, err := time.LoadLocation(request.Timezone); err != nil {
		return response.Response[SettingsResponse]{}, apperrors.BadRequest("时区无效")
	}
	if request.CheckRetentionDays < 1 || request.EventRetentionDays < 1 || request.MetricRetentionDays < 1 || request.HourMetricRetentionDays < 1 {
		return response.Response[SettingsResponse]{}, apperrors.BadRequest("保留天数至少为 1")
	}
	if request.DefaultTimeout < 1 || request.DefaultTimeout > 120 || request.DefaultInterval < 5 {
		return response.Response[SettingsResponse]{}, apperrors.BadRequest("默认超时或频率不合法")
	}
	previousZone := r.store.Snapshot().Timezone
	if err := r.store.Update(map[string]string{
		settings.KeyTimezone:            request.Timezone,
		settings.KeyCheckRetention:      strconv.Itoa(request.CheckRetentionDays),
		settings.KeyEventRetention:      strconv.Itoa(request.EventRetentionDays),
		settings.KeyMetricRetention:     strconv.Itoa(request.MetricRetentionDays),
		settings.KeyHourMetricRetention: strconv.Itoa(request.HourMetricRetentionDays),
		settings.KeyDefaultTimeout:      strconv.Itoa(request.DefaultTimeout),
		settings.KeyDefaultInterval:     strconv.Itoa(request.DefaultInterval),
	}); err != nil {
		return response.Response[SettingsResponse]{}, apperrors.Internal("保存失败")
	}
	if previousZone != request.Timezone {
		if err := r.cron.RescheduleAll(); err != nil {
			slog.Error("重算定时任务失败", "err", err.Error())
		}
	}
	return response.OK(r.view()), nil
}

// UpdateMail 修改邮件通道设置。
func (r *Resource) UpdateMail(ctx context.Context, request *MailRequest) (response.Response[SettingsResponse], error) {
	if request == nil {
		return response.Response[SettingsResponse]{}, apperrors.BadRequest("请求格式不正确")
	}
	cfg, err := r.mailConfig(*request)
	if err != nil {
		return response.Response[SettingsResponse]{}, err
	}
	if cfg.Enabled {
		if err := cfg.Validate(); err != nil {
			return response.Response[SettingsResponse]{}, apperrors.BadRequest(err.Error())
		}
	} else if err := validateMailDraft(cfg); err != nil {
		return response.Response[SettingsResponse]{}, apperrors.BadRequest(err.Error())
	}
	values := map[string]string{
		settings.KeySMTPEnabled:    strconv.FormatBool(cfg.Enabled),
		settings.KeySMTPHost:       cfg.Host,
		settings.KeySMTPPort:       strconv.Itoa(cfg.Port),
		settings.KeySMTPUsername:   cfg.Username,
		settings.KeySMTPFrom:       cfg.From,
		settings.KeySMTPTo:         cfg.To,
		settings.KeySMTPEncryption: cfg.Encryption,
	}
	if password := strings.TrimSpace(request.Password); password != "" && !strings.Contains(password, "****") {
		values[settings.KeySMTPPassword] = password
	}
	if err := r.store.Update(values); err != nil {
		return response.Response[SettingsResponse]{}, apperrors.Internal("保存失败")
	}
	return response.OK(r.view()), nil
}

// TestMail 发送测试邮件。
func (r *Resource) TestMail(ctx context.Context, request *MailRequest) (response.Response[MailTestResponse], error) {
	if request == nil {
		return response.Response[MailTestResponse]{}, apperrors.BadRequest("请求格式不正确")
	}
	cfg, err := r.mailConfig(*request)
	if err != nil {
		return response.Response[MailTestResponse]{}, err
	}
	if err := r.mail.SendTest(ctx, cfg); err != nil {
		return response.Response[MailTestResponse]{}, apperrors.Unavailable(err.Error())
	}
	return response.OK(MailTestResponse{OK: true, Message: "测试邮件已发送"}), nil
}

func (r *Resource) mailConfig(req MailRequest) (mailer.Config, error) {
	switch strings.ToLower(strings.TrimSpace(req.Encryption)) {
	case "", "starttls", "ssl", "tls", "smtps", "none", "plain", "off":
	default:
		return mailer.Config{}, apperrors.BadRequest("加密方式仅支持 STARTTLS、SSL 或不加密")
	}
	if req.Port < 0 || req.Port > 65535 {
		return mailer.Config{}, apperrors.BadRequest("SMTP 端口不合法")
	}
	current := r.store.SMTP()
	password := strings.TrimSpace(req.Password)
	if password == "" || strings.Contains(password, "****") {
		password = current.Password
	}
	return toMailerConfig(req, password), nil
}

func (r *Resource) view() SettingsResponse {
	snap := r.store.Snapshot()
	return SettingsResponse{
		Timezone:                snap.Timezone,
		CheckRetentionDays:      snap.CheckRetentionDays,
		EventRetentionDays:      snap.EventRetentionDays,
		MetricRetentionDays:     snap.MetricRetentionDays,
		HourMetricRetentionDays: snap.HourMetricRetentionDays,
		DefaultTimeout:          snap.DefaultTimeout,
		DefaultInterval:         snap.DefaultInterval,
		WorkerCount:             r.cfg.Scheduler.WorkerCount,
		ScanInterval:            r.cfg.Scheduler.ScanInterval.String(),
		UsingDefaultPassword:    r.store.UsingDefaultPassword(),
		Version:                 version.Version,
		Database:                r.cfg.Database.Path,
		Mail:                    toMailView(r.store.SMTP()),
	}
}

// validateMailDraft 在停用邮件时仍校验已填字段的格式。
func validateMailDraft(cfg mailer.Config) error {
	if cfg.From != "" {
		draft := cfg
		draft.To = "ops@example.com"
		draft.Host = "smtp.example.com"
		if cfg.Host != "" {
			draft.Host = cfg.Host
		}
		if err := draft.Validate(); err != nil && strings.Contains(err.Error(), "发件人") {
			return err
		}
	}
	if cfg.To != "" {
		draft := cfg
		draft.From = "nmonitor@example.com"
		draft.Host = "smtp.example.com"
		if err := draft.Validate(); err != nil && strings.Contains(err.Error(), "收件人") {
			return err
		}
	}
	return nil
}

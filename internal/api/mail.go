package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/gin-gonic/gin"
)

type mailRequest struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	From       string `json:"from"`
	To         string `json:"to"`
	Encryption string `json:"encryption"`
}

func mailView(cfg settings.SMTPConfig) gin.H {
	return gin.H{
		"enabled":     cfg.Enabled,
		"host":        cfg.Host,
		"port":        cfg.Port,
		"username":    cfg.Username,
		"passwordSet": cfg.Password != "",
		"from":        cfg.From,
		"to":          cfg.To,
		"encryption":  cfg.Encryption,
	}
}

func (s *Server) updateMail(c *gin.Context) {
	var req mailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	cfg, err := s.mailConfig(req)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if cfg.Enabled {
		if err := cfg.Validate(); err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
	} else if err := validateMailDraft(cfg); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
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
	if password := strings.TrimSpace(req.Password); password != "" && !strings.Contains(password, "****") {
		values[settings.KeySMTPPassword] = password
	}
	if err := s.settings.Update(values); err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	s.getSettings(c)
}

func (s *Server) testMail(c *gin.Context) {
	var req mailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	cfg, err := s.mailConfig(req)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.mail.SendTest(c.Request.Context(), cfg); err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, gin.H{"ok": true, "message": "测试邮件已发送"})
}

func (s *Server) mailConfig(req mailRequest) (mailer.Config, error) {
	switch strings.ToLower(strings.TrimSpace(req.Encryption)) {
	case "", "starttls", "ssl", "tls", "smtps", "none", "plain", "off":
	default:
		return mailer.Config{}, errString("加密方式仅支持 STARTTLS、SSL 或不加密")
	}
	if req.Port < 0 || req.Port > 65535 {
		return mailer.Config{}, errString("SMTP 端口不合法")
	}
	current := s.settings.SMTP()
	password := strings.TrimSpace(req.Password)
	if password == "" || strings.Contains(password, "****") {
		password = current.Password
	}
	return mailer.Normalize(mailer.Config{
		Enabled:    req.Enabled,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		Password:   password,
		From:       req.From,
		To:         req.To,
		Encryption: req.Encryption,
	}), nil
}

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

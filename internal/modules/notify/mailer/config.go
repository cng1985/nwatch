package mailer

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

type Config struct {
	Enabled    bool
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	To         string
	Encryption string
	Recipients []string
}

func Normalize(c Config) Config {
	c.Host = strings.TrimSpace(c.Host)
	c.Username = strings.TrimSpace(c.Username)
	c.Password = strings.TrimSpace(c.Password)
	c.From = strings.TrimSpace(c.From)
	c.To = strings.TrimSpace(c.To)
	switch strings.ToLower(strings.TrimSpace(c.Encryption)) {
	case "ssl", "tls", "smtps":
		c.Encryption = "ssl"
	case "none", "plain", "off":
		c.Encryption = "none"
	default:
		c.Encryption = "starttls"
	}
	if c.Port <= 0 {
		if c.Encryption == "ssl" {
			c.Port = 465
		} else {
			c.Port = 587
		}
	}
	c.Recipients = splitAddresses(c.To)
	return c
}

func (c Config) Validate() error {
	if c.Host == "" {
		return errors.New("SMTP 主机不能为空")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("SMTP 端口不合法")
	}
	if _, err := mail.ParseAddress(c.From); err != nil {
		return errors.New("发件人邮箱不正确")
	}
	if len(c.Recipients) == 0 {
		return errors.New("至少填写一个收件人")
	}
	for _, addr := range c.Recipients {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("收件人邮箱不正确: %s", addr)
		}
	}
	switch c.Encryption {
	case "ssl", "none", "starttls":
	default:
		return errors.New("加密方式仅支持 STARTTLS、SSL 或不加密")
	}
	return nil
}

func splitAddresses(raw string) []string {
	raw = strings.NewReplacer("，", ",", "；", ",", ";", ",", "\n", ",", "\r", ",").Replace(raw)
	var out []string
	seen := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(part)]; ok {
			continue
		}
		seen[strings.ToLower(part)] = struct{}{}
		out = append(out, part)
	}
	return out
}

package config

import "testing"

func TestSMTPEnvOverrides(t *testing.T) {
	t.Setenv("NMONITOR_SMTP_ENABLED", "true")
	t.Setenv("NMONITOR_SMTP_HOST", "smtp.example.com")
	t.Setenv("NMONITOR_SMTP_PORT", "2525")
	t.Setenv("NMONITOR_SMTP_USERNAME", "mailer")
	t.Setenv("NMONITOR_SMTP_PASSWORD", "secret")
	t.Setenv("NMONITOR_SMTP_FROM", "nmonitor@example.com")
	t.Setenv("NMONITOR_SMTP_TO", "ops@example.com")
	t.Setenv("NMONITOR_SMTP_ENCRYPTION", "none")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Mail.Enabled || cfg.Mail.Host != "smtp.example.com" || cfg.Mail.Port != 2525 || cfg.Mail.Encryption != "none" {
		t.Fatalf("%+v", cfg.Mail)
	}
	if cfg.Mail.Password != "secret" || cfg.Mail.From != "nmonitor@example.com" || cfg.Mail.To != "ops@example.com" {
		t.Fatalf("%+v", cfg.Mail)
	}
}

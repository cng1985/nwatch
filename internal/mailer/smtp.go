package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

func Send(ctx context.Context, cfg Config, subject, body string) error {
	cfg = Normalize(cfg)
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var (
		conn net.Conn
		err  error
	)
	if cfg.Encryption == "ssl" {
		conn, err = (&tls.Dialer{
			NetDialer: dialer,
			Config:    &tls.Config{ServerName: serverName(cfg.Host)},
		}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("连接邮件服务器失败: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("邮件服务器握手失败: %w", err)
	}
	defer client.Close()
	if cfg.Encryption == "starttls" {
		if err := client.StartTLS(&tls.Config{ServerName: serverName(cfg.Host)}); err != nil {
			return fmt.Errorf("STARTTLS 失败: %w", err)
		}
	}
	if cfg.Username != "" {
		if err := client.Auth(chooseAuth(client, cfg)); err != nil {
			return fmt.Errorf("邮件登录失败: %w", err)
		}
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return errors.New("发件人邮箱不正确")
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("发件人被拒绝: %w", err)
	}
	var toHeaders []string
	for _, raw := range cfg.Recipients {
		addr, err := mail.ParseAddress(raw)
		if err != nil {
			return fmt.Errorf("收件人邮箱不正确: %s", raw)
		}
		if err := client.Rcpt(addr.Address); err != nil {
			return fmt.Errorf("收件人被拒绝 %s: %w", addr.Address, err)
		}
		toHeaders = append(toHeaders, addr.String())
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("写入邮件失败: %w", err)
	}
	payload := renderMessage(from.String(), strings.Join(toHeaders, ", "), subject, body)
	if _, err := writer.Write([]byte(payload)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("写入邮件失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("邮件服务器拒绝接受: %w", err)
	}
	_ = client.Quit()
	return nil
}

func renderMessage(from, to, subject, body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	if !strings.HasSuffix(body, "\r\n") {
		body += "\r\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

func serverName(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func chooseAuth(client *smtp.Client, cfg Config) smtp.Auth {
	_, mechs := client.Extension("AUTH")
	mechs = strings.ToUpper(mechs)
	if strings.Contains(mechs, "PLAIN") || !strings.Contains(mechs, "LOGIN") {
		return plainAuth{username: cfg.Username, password: cfg.Password, host: cfg.Host}
	}
	return &loginAuth{username: cfg.Username, password: cfg.Password}
}

type plainAuth struct {
	username, password, host string
}

func (a plainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server.Name != a.host {
		return "", nil, errors.New("邮件服务器名称不匹配")
	}
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a plainAuth) Next([]byte, bool) ([]byte, error) {
	return nil, nil
}

type loginAuth struct {
	username, password string
	step               int
}

func (a *loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	a.step = 0
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(from []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	a.step++
	prompt := strings.ToLower(string(from))
	if strings.Contains(prompt, "password") || a.step > 1 {
		return []byte(a.password), nil
	}
	return []byte(a.username), nil
}

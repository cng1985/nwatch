package checker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

type TLSChecker struct {
	roots *x509.CertPool
}

func NewTLSChecker() *TLSChecker { return &TLSChecker{} }

func (c *TLSChecker) Type() string { return model.TypeTLS }

func (c *TLSChecker) Check(ctx context.Context, m *model.Monitor) (*Result, error) {
	timeout := time.Duration(m.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	host, port, serverName := TLSEndpoint(m)
	if host == "" {
		return failResult("INVALID_TARGET", "缺少证书监控地址"), nil
	}
	addr := net.JoinHostPort(host, port)
	dialer := &net.Dialer{Timeout: timeout}
	cfg := &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}
	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, cfg)
	elapsed := int(time.Since(start).Milliseconds())
	if err != nil {
		status := "ERROR"
		msg := err.Error()
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() || strings.Contains(strings.ToLower(msg), "timeout") || errors.Is(err, context.DeadlineExceeded) {
			status = "TIMEOUT"
			msg = "connection timeout"
		}
		return &Result{
			Success:      false,
			Status:       status,
			ResponseTime: elapsed,
			Message:      msg,
			CheckedAt:    time.Now(),
		}, nil
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return &Result{
			Success:      false,
			Status:       "INVALID",
			ResponseTime: elapsed,
			Message:      "未获取到证书",
			CheckedAt:    time.Now(),
			Immediate:    true,
			TLS:          &TLSInfo{Status: model.TLSInvalid},
		}, nil
	}
	leaf := certs[0]
	info := inspectCert(leaf, serverName, c.roots, certs)
	ApplyThreshold(info, m.TLSWarningDays, m.TLSCriticalDays)
	meta := map[string]any{
		"issuer":        info.Issuer,
		"notAfter":      info.NotAfter.Format(time.RFC3339),
		"notBefore":     info.NotBefore.Format(time.RFC3339),
		"daysRemaining": info.DaysRemaining,
		"subject":       info.CN,
		"serial":        info.Serial,
		"san":           info.SAN,
		"tlsStatus":     info.Status,
	}
	res := &Result{
		Success:      info.Status == model.TLSNormal || info.Status == model.TLSWarning || info.Status == model.TLSCritical,
		Status:       info.Status,
		ResponseTime: elapsed,
		Message:      tlsMessage(info),
		CheckedAt:    time.Now(),
		Metadata:     meta,
		TLS:          info,
		Immediate:    info.Status == model.TLSExpired || info.Status == model.TLSInvalid,
	}
	return res, nil
}

func inspectCert(leaf *x509.Certificate, serverName string, roots *x509.CertPool, chain []*x509.Certificate) *TLSInfo {
	now := time.Now()
	days := 0
	if !leaf.NotAfter.IsZero() {
		remain := leaf.NotAfter.Sub(now)
		if remain > 0 {
			days = int(remain.Hours() / 24)
		}
	}
	info := &TLSInfo{
		CN:            leaf.Subject.CommonName,
		SAN:           sanList(leaf),
		Issuer:        issuerName(leaf),
		Serial:        strings.ToUpper(leaf.SerialNumber.Text(16)),
		NotBefore:     leaf.NotBefore,
		NotAfter:      leaf.NotAfter,
		DaysRemaining: days,
		Status:        model.TLSNormal,
	}
	if now.Before(leaf.NotBefore) {
		info.Status = model.TLSInvalid
		return info
	}
	if !now.Before(leaf.NotAfter) {
		info.Status = model.TLSExpired
		info.DaysRemaining = 0
		return info
	}
	if serverName != "" && leaf.VerifyHostname(serverName) != nil {
		info.Status = model.TLSInvalid
		return info
	}
	opts := x509.VerifyOptions{DNSName: serverName, CurrentTime: now, Roots: roots}
	if len(chain) > 1 {
		pool := x509.NewCertPool()
		for _, c := range chain[1:] {
			pool.AddCert(c)
		}
		opts.Intermediates = pool
	}
	if _, err := leaf.Verify(opts); err != nil {
		info.Status = model.TLSInvalid
		return info
	}
	return info
}

func ClassifyTLS(days int, warningDays, criticalDays int) string {
	if warningDays <= 0 {
		warningDays = 14
	}
	if criticalDays <= 0 {
		criticalDays = 7
	}
	if days <= 0 {
		return model.TLSExpired
	}
	if days <= criticalDays {
		return model.TLSCritical
	}
	if days <= warningDays {
		return model.TLSWarning
	}
	return model.TLSNormal
}

func ApplyThreshold(info *TLSInfo, warningDays, criticalDays int) {
	if info == nil {
		return
	}
	if info.Status == model.TLSInvalid || info.Status == model.TLSExpired {
		return
	}
	info.Status = ClassifyTLS(info.DaysRemaining, warningDays, criticalDays)
}

func sanList(cert *x509.Certificate) string {
	parts := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		parts = append(parts, ip.String())
	}
	return strings.Join(parts, ", ")
}

func issuerName(cert *x509.Certificate) string {
	if cert.Issuer.CommonName != "" {
		return cert.Issuer.CommonName
	}
	return cert.Issuer.String()
}

func tlsMessage(info *TLSInfo) string {
	switch info.Status {
	case model.TLSExpired:
		return "证书已过期"
	case model.TLSInvalid:
		return "证书无效"
	case model.TLSCritical:
		return "证书即将过期，剩余 " + strconv.Itoa(info.DaysRemaining) + " 天"
	case model.TLSWarning:
		return "证书进入预警，剩余 " + strconv.Itoa(info.DaysRemaining) + " 天"
	default:
		return "证书正常，剩余 " + strconv.Itoa(info.DaysRemaining) + " 天"
	}
}

package checker

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

const maxBody = 1 << 20

type HTTPChecker struct {
	transport *http.Transport
}

func NewHTTPChecker() *HTTPChecker {
	return &HTTPChecker{
		transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

func (c *HTTPChecker) Type() string { return model.TypeHTTP }

func (c *HTTPChecker) Check(ctx context.Context, m *model.Monitor) (*Result, error) {
	timeout := time.Duration(m.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	method := strings.ToUpper(strings.TrimSpace(m.Method))
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if m.Body != "" && method != http.MethodHead {
		body = strings.NewReader(m.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, m.URL, body)
	if err != nil {
		return failResult("INVALID_URL", err.Error()), nil
	}
	for k, v := range m.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "NMonitor/1.0")
	}

	client := &http.Client{Transport: c.transport, Timeout: timeout}
	if !m.FollowRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := int(time.Since(start).Milliseconds())
	if err != nil {
		status := "ERROR"
		msg := err.Error()
		if ne, ok := err.(net.Error); ok && ne.Timeout() || strings.Contains(strings.ToLower(msg), "timeout") || strings.Contains(msg, "context deadline") {
			status = "TIMEOUT"
			msg = "connection timeout"
		}
		return &Result{
			Success:      false,
			Status:       status,
			ResponseTime: elapsed,
			Message:      msg,
			CheckedAt:    time.Now(),
			Metadata:     map[string]any{"error": msg},
		}, nil
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	text := string(payload)

	res := &Result{
		Success:      true,
		Status:       "HTTP_OK",
		StatusCode:   resp.StatusCode,
		ResponseTime: elapsed,
		Message:      resp.Status,
		CheckedAt:    time.Now(),
		Metadata: map[string]any{
			"statusCode": resp.StatusCode,
		},
	}
	if !MatchStatus(resp.StatusCode, m.ExpectedStatusCodes) {
		res.Success = false
		res.Status = "UNEXPECTED_STATUS"
		res.Message = "HTTP 状态码 " + strconv.Itoa(resp.StatusCode) + "，期望 " + defaultStatus(m.ExpectedStatusCodes)
	}
	if res.Success && m.BodyContains != "" && !strings.Contains(text, m.BodyContains) {
		res.Success = false
		res.Status = "BODY_MISMATCH"
		res.Message = "响应未包含指定内容"
	}
	if res.Success && m.BodyNotContains != "" && strings.Contains(text, m.BodyNotContains) {
		res.Success = false
		res.Status = "BODY_MISMATCH"
		res.Message = "响应包含了被禁止的内容"
	}
	return res, nil
}

func defaultStatus(s string) string {
	if strings.TrimSpace(s) == "" {
		return "200"
	}
	return s
}

func failResult(status, msg string) *Result {
	return &Result{
		Success:   false,
		Status:    status,
		Message:   msg,
		CheckedAt: time.Now(),
		Metadata:  map[string]any{"error": msg},
	}
}

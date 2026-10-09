package checker

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

type TCPChecker struct{}

func NewTCPChecker() *TCPChecker { return &TCPChecker{} }

func (c *TCPChecker) Type() string { return model.TypeTCP }

func (c *TCPChecker) Check(ctx context.Context, m *model.Monitor) (*Result, error) {
	timeout := time.Duration(m.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	dialer := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	elapsed := int(time.Since(start).Milliseconds())
	if err != nil {
		status := "ERROR"
		msg := err.Error()
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() || strings.Contains(strings.ToLower(msg), "timeout") || errors.Is(err, context.DeadlineExceeded) {
			status = "TIMEOUT"
			msg = "connection timeout"
		} else if strings.Contains(strings.ToLower(msg), "refused") {
			status = "REFUSED"
			msg = "connection refused"
		}
		return &Result{
			Success:      false,
			Status:       status,
			ResponseTime: elapsed,
			Message:      msg,
			CheckedAt:    time.Now(),
			Metadata:     map[string]any{"connectTime": elapsed, "error": msg},
		}, nil
	}
	_ = conn.Close()
	return &Result{
		Success:      true,
		Status:       "CONNECTED",
		ResponseTime: elapsed,
		Message:      "connected",
		CheckedAt:    time.Now(),
		Metadata:     map[string]any{"connectTime": elapsed},
	}, nil
}

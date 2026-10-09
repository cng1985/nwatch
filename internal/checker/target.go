package checker

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/cng1985/nwatch/internal/model"
)

func Target(m *model.Monitor) string {
	switch m.Type {
	case model.TypeTCP:
		return net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	case model.TypeTLS:
		host, port, _ := TLSEndpoint(m)
		return net.JoinHostPort(host, port)
	default:
		return m.URL
	}
}

func TLSEndpoint(m *model.Monitor) (host, port, serverName string) {
	raw := strings.TrimSpace(m.URL)
	if raw == "" {
		raw = strings.TrimSpace(m.Host)
	}
	if raw == "" {
		return "", "443", ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		host = strings.TrimSpace(m.Host)
		port = "443"
		if m.Port > 0 {
			port = strconv.Itoa(m.Port)
		}
		return host, port, host
	}
	host = u.Hostname()
	port = u.Port()
	if port == "" {
		if m.Port > 0 {
			port = strconv.Itoa(m.Port)
		} else {
			port = "443"
		}
	}
	return host, port, host
}

func FormatTarget(m *model.Monitor) string {
	t := Target(m)
	if t == "" {
		return fmt.Sprintf("%s:%d", m.Host, m.Port)
	}
	return t
}

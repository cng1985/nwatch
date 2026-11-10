package checker

import (
	"fmt"
	"github.com/cng1985/nwatch/internal/modules/monitoring/host"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func Target(m *shared.Monitor) string {
	switch m.Type {
	case shared.TypeTCP:
		return net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	case shared.TypeTLS:
		host, port, _ := TLSEndpoint(m)
		return net.JoinHostPort(host, port)
	case shared.TypeCPU:
		return fmt.Sprintf("CPU %.0f%%", thresholdOf(m))
	case shared.TypeMemory:
		return fmt.Sprintf("内存 %.0f%%", thresholdOf(m))
	case shared.TypeDisk:
		path := m.Host
		if path == "" {
			path = host.DefaultDiskPath()
		}
		return fmt.Sprintf("%s %.0f%%", path, thresholdOf(m))
	case shared.TypeProcess:
		name := strings.TrimSpace(m.Host)
		if name == "" {
			return "进程"
		}
		return name
	case shared.TypeScript:
		line := strings.TrimSpace(m.Command)
		if i := strings.IndexAny(line, "\r\n"); i >= 0 {
			line = line[:i]
		}
		if line == "" {
			return "脚本"
		}
		return line
	default:
		return m.URL
	}
}

func TLSEndpoint(m *shared.Monitor) (host, port, serverName string) {
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

func FormatTarget(m *shared.Monitor) string {
	t := Target(m)
	if t == "" {
		return fmt.Sprintf("%s:%d", m.Host, m.Port)
	}
	return t
}

package monitor

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

type Request struct {
	Name                string            `json:"name"`
	GroupID             *uint             `json:"groupId"`
	Type                string            `json:"type"`
	URL                 string            `json:"url"`
	Host                string            `json:"host"`
	Port                int               `json:"port"`
	Method              string            `json:"method"`
	Headers             map[string]string `json:"headers"`
	Body                string            `json:"body"`
	Interval            int               `json:"interval"`
	Timeout             int               `json:"timeout"`
	FailureThreshold    int               `json:"failureThreshold"`
	RecoveryThreshold   int               `json:"recoveryThreshold"`
	ExpectedStatusCodes string            `json:"expectedStatusCodes"`
	BodyContains        string            `json:"bodyContains"`
	BodyNotContains     string            `json:"bodyNotContains"`
	FollowRedirects     *bool             `json:"followRedirects"`
	TLSWarningDays      int               `json:"tlsWarningDays"`
	TLSCriticalDays     int               `json:"tlsCriticalDays"`
	TLSNotifyDays       string            `json:"tlsNotifyDays"`
	NotifierIDs         []uint            `json:"notifierIds"`
	Enabled             *bool             `json:"enabled"`
}

func (r *Request) Normalize(defaultsInterval, defaultsTimeout int) error {
	r.Name = strings.TrimSpace(r.Name)
	r.Type = strings.ToLower(strings.TrimSpace(r.Type))
	r.URL = strings.TrimSpace(r.URL)
	r.Host = strings.TrimSpace(r.Host)
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	if r.Name == "" {
		return errors.New("名称不能为空")
	}
	if len(r.Name) > 128 {
		return errors.New("名称过长")
	}
	switch r.Type {
	case model.TypeHTTP, model.TypeTLS, model.TypeTCP:
	default:
		return errors.New("监控类型仅支持 http、tls、tcp")
	}
	if r.Interval <= 0 {
		r.Interval = defaultsInterval
	}
	if r.Timeout <= 0 {
		r.Timeout = defaultsTimeout
	}
	if r.Interval < 5 || r.Interval > 7*24*3600 {
		return errors.New("检测频率需在 5 秒到 7 天之间")
	}
	if r.Timeout < 1 || r.Timeout > 120 {
		return errors.New("超时时间需在 1 到 120 秒之间")
	}
	if r.Timeout >= r.Interval {
		return errors.New("超时时间必须小于检测频率")
	}
	if r.FailureThreshold <= 0 {
		r.FailureThreshold = 3
	}
	if r.RecoveryThreshold <= 0 {
		r.RecoveryThreshold = 1
	}
	if r.FailureThreshold > 100 || r.RecoveryThreshold > 100 {
		return errors.New("连续失败次数和恢复次数需在 1 到 100 之间")
	}
	if len(r.Body) > 64*1024 {
		return errors.New("请求体不能超过 64KB")
	}
	if len(r.Headers) > 20 {
		return errors.New("请求头不能超过 20 个")
	}
	for k, v := range r.Headers {
		if strings.TrimSpace(k) == "" || len(k) > 128 || len(v) > 1024 {
			return errors.New("请求头不合法")
		}
	}
	switch r.Type {
	case model.TypeHTTP:
		u, err := url.Parse(r.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("HTTP 监控需要以 http:// 或 https:// 开头的地址")
		}
		if r.Method == "" {
			r.Method = "GET"
		}
		switch r.Method {
		case "GET", "POST", "HEAD":
		default:
			return errors.New("请求方法仅支持 GET、POST、HEAD")
		}
		if strings.TrimSpace(r.ExpectedStatusCodes) == "" {
			r.ExpectedStatusCodes = "200"
		}
		if r.FollowRedirects == nil {
			t := true
			r.FollowRedirects = &t
		}
	case model.TypeTCP:
		if r.Host == "" || r.Port < 1 || r.Port > 65535 {
			return errors.New("TCP 监控需要有效的主机和端口")
		}
	case model.TypeTLS:
		if r.URL == "" && r.Host == "" {
			return errors.New("证书监控需要域名或地址")
		}
		if r.Port < 0 || r.Port > 65535 {
			return errors.New("端口不合法")
		}
		if r.TLSWarningDays <= 0 {
			r.TLSWarningDays = 14
		}
		if r.TLSCriticalDays <= 0 {
			r.TLSCriticalDays = 7
		}
		if r.TLSCriticalDays > r.TLSWarningDays {
			return errors.New("证书严重阈值不能大于警告阈值")
		}
		if strings.TrimSpace(r.TLSNotifyDays) == "" {
			r.TLSNotifyDays = "30,14,7,3,1,0"
		}
	}
	if r.Enabled == nil {
		t := true
		r.Enabled = &t
	}
	return nil
}

func (r *Request) Apply(m *model.Monitor) {
	m.Name = r.Name
	m.GroupID = r.GroupID
	if m.GroupID != nil && *m.GroupID == 0 {
		m.GroupID = nil
	}
	m.Type = r.Type
	m.URL = r.URL
	m.Host = r.Host
	m.Port = r.Port
	m.Method = r.Method
	m.Headers = r.Headers
	m.SyncHeaders()
	m.Body = r.Body
	m.Interval = r.Interval
	m.Timeout = r.Timeout
	m.FailureThreshold = r.FailureThreshold
	m.RecoveryThreshold = r.RecoveryThreshold
	m.ExpectedStatusCodes = r.ExpectedStatusCodes
	m.BodyContains = r.BodyContains
	m.BodyNotContains = r.BodyNotContains
	if r.FollowRedirects != nil {
		m.FollowRedirects = *r.FollowRedirects
	}
	m.TLSWarningDays = r.TLSWarningDays
	m.TLSCriticalDays = r.TLSCriticalDays
	m.TLSNotifyDays = r.TLSNotifyDays
	if r.Enabled != nil {
		m.Enabled = *r.Enabled
	}
}

func HeadersJSON(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	raw, _ := json.Marshal(headers)
	return string(raw)
}

func TouchNew(m *model.Monitor) {
	now := time.Now()
	if m.Status == "" {
		m.Status = model.StatusUnknown
	}
	if !m.Enabled {
		m.Status = model.StatusPaused
	}
	m.NextCheckAt = &now
	m.LastTLSNotified = ""
}

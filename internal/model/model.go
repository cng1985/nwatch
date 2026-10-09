package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	TypeHTTP = "http"
	TypeTLS  = "tls"
	TypeTCP  = "tcp"

	StatusUnknown = "UNKNOWN"
	StatusUp      = "UP"
	StatusDown    = "DOWN"
	StatusPaused  = "PAUSED"

	TLSNormal   = "NORMAL"
	TLSWarning  = "WARNING"
	TLSCritical = "CRITICAL"
	TLSExpired  = "EXPIRED"
	TLSInvalid  = "INVALID"

	EventDown       = "MONITOR_DOWN"
	EventRecovered  = "MONITOR_RECOVERED"
	EventTLSWarn    = "TLS_WARNING"
	EventTLSCrit    = "TLS_CRITICAL"
	EventTLSExpire  = "TLS_EXPIRED"
	EventTLSInvalid = "TLS_INVALID"
	EventTLSRecover = "TLS_RECOVERED"

	NotifierDingTalk = "dingtalk"
	NotifierWeCom    = "wecom"
	NotifierWebhook  = "webhook"

	BucketMinute = "minute"
	BucketHour   = "hour"
	BucketDay    = "day"
)

type Group struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null;uniqueIndex" json:"name"`
	Description string    `gorm:"size:512" json:"description"`
	Sort        int       `json:"sort"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Monitor struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Name    string `gorm:"size:128;not null;index" json:"name"`
	GroupID *uint  `gorm:"index" json:"groupId"`
	Group   *Group `json:"group,omitempty"`
	Type    string `gorm:"size:16;not null;index" json:"type"`

	URL    string `gorm:"size:2048" json:"url"`
	Host   string `gorm:"size:255" json:"host"`
	Port   int    `json:"port"`
	Method string `gorm:"size:16" json:"method"`
	// HeadersRaw is the persisted JSON object. Headers is filled for API use.
	HeadersRaw string            `gorm:"column:headers;type:text" json:"-"`
	Headers    map[string]string `gorm:"-" json:"headers"`
	Body       string            `gorm:"type:text" json:"body"`

	Interval int    `gorm:"not null" json:"interval"`
	Timeout  int    `gorm:"not null" json:"timeout"`
	Enabled  bool   `gorm:"not null;index" json:"enabled"`
	Status   string `gorm:"size:16;not null;index" json:"status"`

	FailureThreshold  int `gorm:"not null" json:"failureThreshold"`
	RecoveryThreshold int `gorm:"not null" json:"recoveryThreshold"`

	ExpectedStatusCodes string `gorm:"size:128" json:"expectedStatusCodes"`
	BodyContains        string `gorm:"size:1024" json:"bodyContains"`
	BodyNotContains     string `gorm:"size:1024" json:"bodyNotContains"`
	FollowRedirects     bool   `json:"followRedirects"`

	TLSWarningDays  int    `json:"tlsWarningDays"`
	TLSCriticalDays int    `json:"tlsCriticalDays"`
	TLSNotifyDays   string `gorm:"size:128" json:"tlsNotifyDays"`

	TLSStatus         string     `gorm:"size:16;index" json:"tlsStatus"`
	CertCN            string     `gorm:"size:255" json:"certCN"`
	CertSAN           string     `gorm:"type:text" json:"certSAN"`
	CertIssuer        string     `gorm:"size:512" json:"certIssuer"`
	CertSerial        string     `gorm:"size:128" json:"certSerial"`
	CertNotBefore     *time.Time `json:"certNotBefore"`
	CertNotAfter      *time.Time `json:"certNotAfter"`
	CertDaysRemaining *int       `json:"certDaysRemaining"`
	LastTLSNotified   string     `gorm:"size:16" json:"-"`

	LastCheckAt       *time.Time `json:"lastCheckAt"`
	NextCheckAt       *time.Time `gorm:"index" json:"nextCheckAt"`
	LastSuccessAt     *time.Time `json:"lastSuccessAt"`
	LastFailureAt     *time.Time `json:"lastFailureAt"`
	LastResponseTime  int        `json:"lastResponseTime"`
	LastMessage       string     `gorm:"size:2048" json:"lastMessage"`
	LastStatusCode    int        `json:"lastStatusCode"`
	IncidentStartedAt *time.Time `json:"incidentStartedAt"`
	// NextNotifyAt / NotifyStage 记录故障期间的下一次通知。0 是 15 秒，1 是 45 秒，之后每分钟一档。
	NextNotifyAt *time.Time `gorm:"index" json:"-"`
	NotifyStage  int        `json:"-"`

	ConsecutiveFailures  int `json:"consecutiveFailures"`
	ConsecutiveSuccesses int `json:"consecutiveSuccesses"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Notifiers      []Notifier      `gorm:"many2many:monitor_notifiers;" json:"-"`
	NotifierIDs    []uint          `gorm:"-" json:"notifierIds"`
	NotifierBriefs []NotifierBrief `gorm:"-" json:"notifiers"`
}

type NotifierBrief struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (m *Monitor) AfterFind(*gorm.DB) error {
	m.Headers = map[string]string{}
	if m.HeadersRaw != "" {
		_ = json.Unmarshal([]byte(m.HeadersRaw), &m.Headers)
	}
	m.NotifierIDs = make([]uint, 0, len(m.Notifiers))
	m.NotifierBriefs = make([]NotifierBrief, 0, len(m.Notifiers))
	for _, n := range m.Notifiers {
		m.NotifierIDs = append(m.NotifierIDs, n.ID)
		m.NotifierBriefs = append(m.NotifierBriefs, NotifierBrief{ID: n.ID, Name: n.Name, Type: n.Type})
	}
	return nil
}

func (m *Monitor) SyncHeaders() {
	if m.Headers == nil {
		m.HeadersRaw = ""
		return
	}
	raw, _ := json.Marshal(m.Headers)
	m.HeadersRaw = string(raw)
}

type Notifier struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:128;not null" json:"name"`
	Type       string    `gorm:"size:32;not null;index" json:"type"`
	WebhookURL string    `gorm:"size:2048" json:"-"`
	Secret     string    `gorm:"size:512" json:"-"`
	Enabled    bool      `gorm:"not null" json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type AlertEvent struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	MonitorID    uint       `gorm:"index" json:"monitorId"`
	MonitorName  string     `gorm:"size:128" json:"monitorName"`
	MonitorType  string     `gorm:"size:16" json:"monitorType"`
	EventType    string     `gorm:"size:32;index" json:"eventType"`
	OldStatus    string     `gorm:"size:16" json:"oldStatus"`
	NewStatus    string     `gorm:"size:16" json:"newStatus"`
	Target       string     `gorm:"size:2048" json:"target"`
	Message      string     `gorm:"size:2048" json:"message"`
	ResponseTime int        `json:"responseTime"`
	OccurredAt   time.Time  `gorm:"index" json:"occurredAt"`
	RecoveredAt  *time.Time `json:"recoveredAt"`
	Duration     int64      `json:"duration"`
	CertExpireAt *time.Time `json:"certificateExpireAt"`
	CertDaysLeft *int       `json:"certificateDaysRemaining"`
}

type NotificationLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	MonitorID    uint      `gorm:"index" json:"monitorId"`
	NotifierID   uint      `gorm:"index" json:"notifierId"`
	NotifierName string    `gorm:"size:128" json:"notifierName"`
	EventID      uint      `gorm:"index" json:"eventId"`
	EventType    string    `gorm:"size:32" json:"eventType"`
	Success      bool      `json:"success"`
	Response     string    `gorm:"type:text" json:"response"`
	ErrorMessage string    `gorm:"size:2048" json:"errorMessage"`
	SentAt       time.Time `gorm:"index" json:"sentAt"`
}

type MonitorCheck struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	MonitorID    uint           `gorm:"index:idx_check_monitor_time,priority:1" json:"monitorId"`
	Success      bool           `json:"success"`
	Status       string         `gorm:"size:32" json:"status"`
	StatusCode   int            `json:"statusCode"`
	ResponseTime int            `json:"responseTime"`
	ErrorMessage string         `gorm:"size:2048" json:"errorMessage"`
	Metadata     string         `gorm:"type:text" json:"-"`
	Meta         map[string]any `gorm:"-" json:"metadata"`
	CheckedAt    time.Time      `gorm:"index:idx_check_monitor_time,priority:2" json:"checkedAt"`
}

func (c *MonitorCheck) AfterFind(*gorm.DB) error {
	if c.Metadata == "" {
		c.Meta = map[string]any{}
		return nil
	}
	if err := json.Unmarshal([]byte(c.Metadata), &c.Meta); err != nil {
		c.Meta = map[string]any{}
	}
	return nil
}

type MonitorMetric struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	MonitorID       uint      `gorm:"uniqueIndex:idx_metric_bucket,priority:1" json:"monitorId"`
	BucketTime      time.Time `gorm:"uniqueIndex:idx_metric_bucket,priority:2" json:"bucketTime"`
	BucketType      string    `gorm:"size:16;uniqueIndex:idx_metric_bucket,priority:3" json:"bucketType"`
	Total           int       `json:"total"`
	Success         int       `json:"success"`
	Failure         int       `json:"failure"`
	AvgResponseTime int       `json:"avgResponseTime"`
	MaxResponseTime int       `json:"maxResponseTime"`
	MinResponseTime int       `json:"minResponseTime"`
	SumResponseTime int64     `json:"-"`
}

type Setting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

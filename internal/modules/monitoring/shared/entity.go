// Package shared 存放监控业务分组内共享的领域实体与常量。
// 作用域仅限 monitoring 分组：monitor、checker、scheduler、state、metric、
// alert、dashboard、host、maintenance、script 与分组内测试。
// 跨分组共享的能力必须放在 platform 或专用契约包，不允许反向依赖父组合包。
package shared

import (
	"encoding/json"
	"github.com/cng1985/nwatch/internal/modules/notify/notifier"
	"gorm.io/gorm"
	"time"
)

const (
	TypeHTTP    = "http"
	TypeTLS     = "tls"
	TypeTCP     = "tcp"
	TypeCPU     = "cpu"
	TypeMemory  = "memory"
	TypeDisk    = "disk"
	TypeScript  = "script"
	TypeProcess = "process"

	// MatchName 只按进程名匹配。MatchCommand 还要求命令行包含所选命令。
	MatchName    = "name"
	MatchCommand = "command"

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

	BucketMinute = "minute"
	BucketHour   = "hour"
	BucketDay    = "day"
)

// Group 是监控分组。
type Group struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null;uniqueIndex" json:"name"`
	Description string    `gorm:"size:512" json:"description"`
	Sort        int       `json:"sort"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (Group) TableName() string { return "groups" }

// Monitor 是一个监控项。HeadersRaw 是持久化 JSON，Headers 供业务使用。
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
	// HeadersRaw 是持久化 JSON 对象。Headers 供业务使用。
	HeadersRaw string            `gorm:"column:headers;type:text" json:"-"`
	Headers    map[string]string `gorm:"-" json:"headers"`
	Body       string            `gorm:"type:text" json:"body"`
	BodyType   string            `gorm:"size:16" json:"bodyType"`
	// Threshold 是 CPU、内存、磁盘使用率告警线，单位是百分比。
	// 进程监控里它是 CPU 占用阈值，0 表示不按 CPU 告警。进程 CPU 按单个核心计算，可以超过 100。
	Threshold float64 `json:"threshold"`
	// MemThreshold 是进程内存占用告警线，单位 MB。0 表示不按内存告警。
	MemThreshold float64 `json:"memThreshold"`
	// Match 是进程匹配方式：name 只按进程名，command 还要求命令行包含所选命令。
	Match   string `gorm:"size:16" json:"match"`
	Command string `gorm:"type:text" json:"command"`
	WorkDir string `gorm:"size:1024" json:"workDir"`

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

	Notifiers      []notifier.Notifier `gorm:"many2many:monitor_notifiers;" json:"-"`
	NotifierIDs    []uint              `gorm:"-" json:"notifierIds"`
	NotifierBriefs []NotifierBrief     `gorm:"-" json:"notifiers"`
}

// NotifierBrief 是列表与选择器里的通知渠道摘要。
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

// AlertEvent 是一次监控状态变化或证书事件。
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

func (AlertEvent) TableName() string { return "alert_events" }

// MonitorCheck 是一次检测结果。
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

func (MonitorCheck) TableName() string { return "monitor_checks" }

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

// MonitorMetric 是一个时间桶内的检测聚合。
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

func (MonitorMetric) TableName() string { return "monitor_metrics" }

// ScriptRun 记录一次手动或定时脚本执行的输出。
type ScriptRun struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	MonitorID    *uint     `gorm:"index" json:"monitorId"`
	Name         string    `gorm:"size:128" json:"name"`
	Command      string    `gorm:"type:text" json:"command"`
	WorkDir      string    `gorm:"size:1024" json:"workDir"`
	ExitCode     int       `json:"exitCode"`
	Success      bool      `json:"success"`
	Stdout       string    `gorm:"type:text" json:"stdout"`
	Stderr       string    `gorm:"type:text" json:"stderr"`
	ErrorMessage string    `gorm:"size:2048" json:"errorMessage"`
	DurationMs   int       `json:"durationMs"`
	StartedAt    time.Time `gorm:"index" json:"startedAt"`
	FinishedAt   time.Time `json:"finishedAt"`
}

func (ScriptRun) TableName() string { return "script_runs" }

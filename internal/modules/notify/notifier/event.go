package notifier

import "time"

// 通知渠道视角的事件类型。监控域的原始事件由告警模块转换成本视图，
// 通知域不反向依赖监控域类型。
const (
	KindTest       = "TEST"
	KindDown       = "MONITOR_DOWN"
	KindRecover    = "MONITOR_RECOVERED"
	KindTLSWarn    = "TLS_WARNING"
	KindTLSCrit    = "TLS_CRITICAL"
	KindTLSExpire  = "TLS_EXPIRED"
	KindTLSInvalid = "TLS_INVALID"
	KindTLSRecover = "TLS_RECOVERED"
)

// Event 是通知渠道接收的告警事件视图。
type Event struct {
	Kind         string
	EventType    string
	MonitorID    uint
	MonitorName  string
	MonitorType  string
	OldStatus    string
	NewStatus    string
	Target       string
	Message      string
	ResponseTime int
	OccurredAt   time.Time
	RecoveredAt  *time.Time
	Duration     int64
	CertExpireAt *time.Time
	CertDaysLeft *int
}

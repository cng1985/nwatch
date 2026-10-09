package mailer

import (
	"fmt"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/notifier"
)

type Notice struct {
	Event    model.AlertEvent
	Failures int
	Ongoing  bool
}

func Format(n Notice, loc *time.Location) (string, string) {
	if loc == nil {
		loc = time.Local
	}
	ev := n.Event
	when := ev.OccurredAt
	if when.IsZero() {
		when = time.Now()
	}
	stamp := when.In(loc).Format("2006-01-02 15:04:05")
	if ev.EventType == "TEST" {
		return "NMonitor 邮件通知测试", strings.Join([]string{
			"这是一封测试邮件，说明当前邮件服务器可以正常发信。",
			"",
			"时间：" + stamp,
		}, "\n")
	}

	name := ev.MonitorName
	if name == "" {
		name = "未命名监控"
	}
	subject := subjectOf(n, name)
	lines := []string{subject, "", "应用：" + name}
	if ev.MonitorType != "" {
		lines = append(lines, "类型："+strings.ToUpper(ev.MonitorType))
	}
	if ev.Target != "" {
		lines = append(lines, "地址："+ev.Target)
	}
	if ev.OldStatus != "" || ev.NewStatus != "" {
		lines = append(lines, "状态："+joinStatus(ev.OldStatus, ev.NewStatus))
	}
	if n.Failures > 0 {
		lines = append(lines, fmt.Sprintf("连续失败：%d 次", n.Failures))
	}
	if ev.Duration > 0 {
		lines = append(lines, "故障持续："+notifier.FormatDuration(ev.Duration))
	}
	if ev.CertDaysLeft != nil {
		lines = append(lines, fmt.Sprintf("证书剩余：%d 天", *ev.CertDaysLeft))
	}
	if ev.CertExpireAt != nil {
		lines = append(lines, "到期时间："+ev.CertExpireAt.In(loc).Format("2006-01-02 15:04:05"))
	}
	if ev.Message != "" {
		lines = append(lines, "错误："+ev.Message)
	}
	if ev.ResponseTime > 0 {
		lines = append(lines, fmt.Sprintf("响应时间：%d ms", ev.ResponseTime))
	}
	lines = append(lines, "时间："+stamp)
	if isFailure(ev.EventType) || n.Ongoing {
		lines = append(lines, "", "服务一直异常时，每次检测失败都会再次发送。如果某一封发送失败，系统会持续重试，直到发送成功。")
	}
	return subject, strings.Join(lines, "\n")
}

func subjectOf(n Notice, name string) string {
	switch n.Event.EventType {
	case model.EventRecovered:
		return "服务已恢复 " + name
	case model.EventTLSRecover:
		return "证书已恢复 " + name
	case model.EventTLSWarn:
		return "证书预警 " + name
	case model.EventTLSCrit:
		return "证书即将过期 " + name
	case model.EventTLSExpire:
		return "证书已过期 " + name
	case model.EventTLSInvalid:
		return "证书无效 " + name
	default:
		if n.Ongoing {
			return "服务仍然异常 " + name
		}
		return "服务异常 " + name
	}
}

func joinStatus(old, next string) string {
	if old == "" || old == next {
		return next
	}
	return old + " → " + next
}

func isFailure(eventType string) bool {
	switch eventType {
	case model.EventDown, model.EventTLSWarn, model.EventTLSCrit, model.EventTLSExpire, model.EventTLSInvalid:
		return true
	default:
		return false
	}
}

func isRecovery(eventType string) bool {
	return eventType == model.EventRecovered || eventType == model.EventTLSRecover
}

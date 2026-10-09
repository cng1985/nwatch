package notifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
)

func FormatDuration(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d天%d小时%d分", days, hours, mins)
	case hours > 0:
		return fmt.Sprintf("%d小时%d分%d秒", hours, mins, secs)
	case mins > 0:
		return fmt.Sprintf("%d分%d秒", mins, secs)
	default:
		return fmt.Sprintf("%d秒", secs)
	}
}

func markdown(event *model.AlertEvent, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	when := event.OccurredAt.In(loc).Format("2006-01-02 15:04:05")
	if event.EventType == "TEST" {
		return strings.Join([]string{
			"### ✅ 通知测试",
			"",
			event.Message,
			"",
			"**时间：** " + when,
		}, "\n")
	}
	switch event.EventType {
	case model.EventRecovered, model.EventTLSRecover:
		title := "🟢 服务恢复"
		if event.EventType == model.EventTLSRecover {
			title = "🟢 证书已恢复"
		}
		return strings.Join([]string{
			"### " + title,
			"",
			"**应用：** " + event.MonitorName,
			"",
			"**故障持续：** " + FormatDuration(event.Duration),
			"",
			"**当前响应：** " + fmt.Sprintf("%d ms", event.ResponseTime),
			"",
			"**时间：** " + when,
		}, "\n")
	case model.EventTLSWarn, model.EventTLSCrit, model.EventTLSExpire, model.EventTLSInvalid:
		title := "🟡 证书预警"
		switch event.EventType {
		case model.EventTLSCrit:
			title = "🟠 证书即将过期"
		case model.EventTLSExpire:
			title = "🔴 证书已过期"
		case model.EventTLSInvalid:
			title = "🔴 证书无效"
		}
		days := "-"
		if event.CertDaysLeft != nil {
			days = fmt.Sprintf("%d", *event.CertDaysLeft)
		}
		expire := "-"
		if event.CertExpireAt != nil {
			expire = event.CertExpireAt.In(loc).Format("2006-01-02 15:04:05")
		}
		return strings.Join([]string{
			"### " + title,
			"",
			"**应用：** " + event.MonitorName,
			"",
			"**地址：** " + event.Target,
			"",
			"**剩余天数：** " + days,
			"",
			"**到期时间：** " + expire,
			"",
			"**说明：** " + event.Message,
			"",
			"**时间：** " + when,
		}, "\n")
	default:
		return strings.Join([]string{
			"### 🔴 服务异常",
			"",
			"**应用：** " + event.MonitorName,
			"",
			"**类型：** " + strings.ToUpper(event.MonitorType),
			"",
			"**地址：** " + event.Target,
			"",
			"**状态：** " + event.OldStatus + " → " + event.NewStatus,
			"",
			"**错误：** " + event.Message,
			"",
			"**响应时间：** " + fmt.Sprintf("%d ms", event.ResponseTime),
			"",
			"**时间：** " + when,
		}, "\n")
	}
}

func titleOf(event *model.AlertEvent) string {
	switch event.EventType {
	case "TEST":
		return "通知测试"
	case model.EventRecovered:
		return "服务恢复"
	case model.EventTLSRecover:
		return "证书已恢复"
	case model.EventTLSWarn:
		return "证书预警"
	case model.EventTLSCrit:
		return "证书即将过期"
	case model.EventTLSExpire:
		return "证书已过期"
	case model.EventTLSInvalid:
		return "证书无效"
	default:
		return "服务异常"
	}
}

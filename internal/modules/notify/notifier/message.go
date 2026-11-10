package notifier

import (
	"fmt"
	"strings"
	"time"
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
	default:
		return fmt.Sprintf("%d分%d秒", mins, secs)
	}
}

func markdown(event *Event, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	when := event.OccurredAt.In(loc).Format("2006-01-02 15:04:05")
	if event.Kind == KindTest {
		return strings.Join([]string{
			"### ✅ 通知测试",
			"",
			event.Message,
			"",
			"**时间：** " + when,
		}, "\n")
	}
	switch event.Kind {
	case KindRecover, KindTLSRecover:
		title := "🟢 服务恢复"
		if event.Kind == KindTLSRecover {
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
	case KindTLSWarn, KindTLSCrit, KindTLSExpire, KindTLSInvalid:
		title := "🟡 证书预警"
		switch event.Kind {
		case KindTLSCrit:
			title = "🟠 证书即将过期"
		case KindTLSExpire:
			title = "🔴 证书已过期"
		case KindTLSInvalid:
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
		status := event.NewStatus
		if event.OldStatus != "" && event.OldStatus != event.NewStatus {
			status = event.OldStatus + " → " + event.NewStatus
		}
		lines := []string{
			"### 🔴 " + failureHeadline(event),
			"",
			"**应用：** " + event.MonitorName,
			"",
			"**类型：** " + strings.ToUpper(event.MonitorType),
			"",
			"**地址：** " + event.Target,
			"",
			"**状态：** " + status,
			"",
			"**错误：** " + event.Message,
		}
		if event.Duration > 0 {
			lines = append(lines, "", "**故障持续：** "+FormatDuration(event.Duration))
		}
		lines = append(lines,
			"",
			"**响应时间：** "+fmt.Sprintf("%d ms", event.ResponseTime),
			"",
			"**时间：** "+when,
		)
		return strings.Join(lines, "\n")
	}
}

// 第二次及以后的故障提醒落在 45 秒及之后，标题改为“仍然异常”。
func failureHeadline(event *Event) string {
	if event != nil && event.Kind == KindDown && event.OldStatus == event.NewStatus && event.Duration >= 45 {
		return "服务仍然异常"
	}
	return "服务异常"
}

func titleOf(event *Event) string {
	switch event.Kind {
	case KindTest:
		return "通知测试"
	case KindRecover:
		return "服务恢复"
	case KindTLSRecover:
		return "证书已恢复"
	case KindTLSWarn:
		return "证书预警"
	case KindTLSCrit:
		return "证书即将过期"
	case KindTLSExpire:
		return "证书已过期"
	case KindTLSInvalid:
		return "证书无效"
	default:
		return failureHeadline(event)
	}
}

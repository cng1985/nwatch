// Package migrations 登记受控迁移的模型清单，按数据归属登记。
// app/bootstrap 在启动时执行；测试通过 tests/harness 复用同一清单。
package migrations

import (
	"github.com/cng1985/nwatch/internal/modules/jobs/cronjob"
	"github.com/cng1985/nwatch/internal/modules/monitoring/host"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/modules/notify/notifier"
	"github.com/cng1985/nwatch/internal/modules/ops/shell"
	"github.com/cng1985/nwatch/internal/platform/auth"
	"github.com/cng1985/nwatch/internal/platform/logging"
	"github.com/cng1985/nwatch/internal/platform/settings"
)

// Models 返回全部业务表的迁移模型。
func Models() []any {
	return []any{
		// 监控分组
		&shared.Group{},
		&shared.Monitor{},
		&shared.MonitorCheck{},
		&shared.MonitorMetric{},
		&shared.AlertEvent{},
		&shared.ScriptRun{},
		&host.HostSample{},
		// 通知分组
		&notifier.Notifier{},
		&notifier.NotificationLog{},
		// 任务分组
		&cronjob.CronJob{},
		&cronjob.CronRun{},
		// 运维分组
		&shell.ShellCommand{},
		// 系统管理
		&settings.Setting{},
		&logging.AppLog{},
		&auth.User{},
		&auth.Role{},
	}
}

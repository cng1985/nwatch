// Package monitoring 组合监控业务分组的全部子模块。
// 子模块不反向导入本包；共享实体与常量在 shared/，作用域仅限本分组。
package monitoring

import (
	"github.com/cng1985/nwatch/internal/modules/monitoring/alert"
	"github.com/cng1985/nwatch/internal/modules/monitoring/checker"
	"github.com/cng1985/nwatch/internal/modules/monitoring/dashboard"
	"github.com/cng1985/nwatch/internal/modules/monitoring/group"
	"github.com/cng1985/nwatch/internal/modules/monitoring/host"
	"github.com/cng1985/nwatch/internal/modules/monitoring/maintenance"
	"github.com/cng1985/nwatch/internal/modules/monitoring/metric"
	"github.com/cng1985/nwatch/internal/modules/monitoring/monitor"
	"github.com/cng1985/nwatch/internal/modules/monitoring/scheduler"
	"github.com/cng1985/nwatch/internal/modules/monitoring/script"
	"github.com/cng1985/nwatch/internal/modules/monitoring/state"
	"go.uber.org/fx"
)

// Module 只组合子模块，不包含业务代码。
var Module = fx.Options(
	state.Module,
	checker.Module,
	metric.Module,
	scheduler.Module,
	alert.Module,
	maintenance.Module,
	host.Module,
	dashboard.Module,
	monitor.Module,
	group.Module,
	script.Module,
)

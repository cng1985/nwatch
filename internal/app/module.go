package app

import (
	"github.com/cng1985/nwatch/internal/modules/admin"
	"github.com/cng1985/nwatch/internal/modules/jobs"
	"github.com/cng1985/nwatch/internal/modules/monitoring"
	"github.com/cng1985/nwatch/internal/modules/notify"
	"github.com/cng1985/nwatch/internal/modules/ops"
	"github.com/cng1985/nwatch/internal/platform/auth"
	"github.com/cng1985/nwatch/internal/platform/database"
	"github.com/cng1985/nwatch/internal/platform/logging"
	"github.com/cng1985/nwatch/internal/platform/server"
	"github.com/cng1985/nwatch/internal/platform/settings"
	web "github.com/cng1985/nwatch/web"
	"go.uber.org/fx"
)

// Module 组合平台、业务和 Web 模块。
// bootstrap 排在最前：迁移先于任何依赖数据库表的初始化执行（Invoke 按注册顺序运行）。
var Module = fx.Options(
	bootstrapModule,
	database.Module,
	logging.Module,
	settings.Module,
	auth.Module,
	server.Module,
	monitoring.Module,
	notify.Module,
	jobs.Module,
	ops.Module,
	admin.Module,
	web.Module,
)

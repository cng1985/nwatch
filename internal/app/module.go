package app

import (
	"github.com/cng1985/nwatch/internal/alert"
	"github.com/cng1985/nwatch/internal/api"
	"github.com/cng1985/nwatch/internal/auth"
	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/host"
	"github.com/cng1985/nwatch/internal/logview"
	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/maintenance"
	"github.com/cng1985/nwatch/internal/metric"
	"github.com/cng1985/nwatch/internal/notifier"
	"github.com/cng1985/nwatch/internal/scheduler"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/state"
	"go.uber.org/fx"
)

// Module 只组装各功能自己的 fx 模块。新功能在对应包里声明 Module，再在这里加一行。
var Module = fx.Options(
	loggerModule,
	config.Module,
	database.Module,
	logview.Module,
	host.Module,
	settings.Module,
	checker.Module,
	state.Module,
	notifier.Module,
	mailer.Module,
	alert.Module,
	metric.Module,
	scheduler.Module,
	maintenance.Module,
	auth.Module,
	api.Module,
)

package cronjob

import (
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/routing"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Module 提供定时任务调度，并注册定时任务业务路由。
var Module = fx.Module("cronjob",
	fx.Provide(
		newService,
		NewRepository,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
	fx.Invoke(func(*Service) {}),
)

func newService(cfg *config.Config, db *gorm.DB, store *settings.Store, lc fx.Lifecycle) *Service {
	return New(db, store, cfg.Scheduler.ScanInterval, lc)
}

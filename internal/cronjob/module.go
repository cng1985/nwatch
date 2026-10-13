package cronjob

import (
	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/settings"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Module 提供定时任务调度。
var Module = fx.Module("cronjob",
	fx.Provide(newService),
	fx.Invoke(func(*Service) {}),
)

func newService(cfg *config.Config, db *gorm.DB, store *settings.Store, lc fx.Lifecycle) *Service {
	return New(db, store, cfg.Scheduler.ScanInterval, lc)
}

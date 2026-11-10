package scheduler

import (
	"github.com/cng1985/nwatch/internal/platform/config"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Module 提供检测调度和工作池。
var Module = fx.Options(
	fx.Module("scheduler",
		fx.Provide(NewProcessor, newPool, newScheduler),
		fx.Invoke(func(*Scheduler, *Pool) {}),
	),
	reporterModule,
)

func newPool(cfg *config.Config, proc *Processor, lc fx.Lifecycle) *Pool {
	return NewPool(cfg.Scheduler.WorkerCount, cfg.Scheduler.QueueSize, proc, lc)
}

func newScheduler(cfg *config.Config, db *gorm.DB, pool *Pool, lc fx.Lifecycle) *Scheduler {
	return NewScheduler(cfg.Scheduler.ScanInterval, db, pool, lc)
}

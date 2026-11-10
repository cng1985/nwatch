package scheduler

import (
	"github.com/cng1985/nwatch/internal/platform/server"
	"go.uber.org/fx"
)

// Reporter 向健康端点上报调度器与工作池状态。
type Reporter struct {
	sched *Scheduler
	pool  *Pool
}

func newReporter(sched *Scheduler, pool *Pool) *Reporter {
	return &Reporter{sched: sched, pool: pool}
}

// HealthReport 实现平台健康检查契约。调度器未运行视为不可用。
func (r *Reporter) HealthReport() server.HealthCheck {
	return server.HealthCheck{Name: "scheduler", Up: r.sched.Running()}
}

// reporterModule 在调度模块内注册健康上报。
var reporterModule = fx.Module("scheduler-reporter",
	fx.Provide(fx.Annotate(newReporter, fx.As(new(server.Reporter)), fx.ResultTags(`group:"health"`))),
)

// Package jobs 组合任务业务分组的子模块。
package jobs

import (
	"github.com/cng1985/nwatch/internal/modules/jobs/cronjob"
	"go.uber.org/fx"
)

// Module 只组合子模块，不包含业务代码。
var Module = fx.Options(
	cronjob.Module,
)

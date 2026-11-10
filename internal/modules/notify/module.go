// Package notify 组合通知业务分组的子模块。
package notify

import (
	"github.com/cng1985/nwatch/internal/modules/notify/mailer"
	"github.com/cng1985/nwatch/internal/modules/notify/notifier"
	"go.uber.org/fx"
)

// Module 只组合子模块，不包含业务代码。
var Module = fx.Options(
	notifier.Module,
	mailer.Module,
)

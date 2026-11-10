// Package admin 组合系统管理分组的子模块。
package admin

import (
	"github.com/cng1985/nwatch/internal/modules/admin/access"
	"github.com/cng1985/nwatch/internal/modules/admin/backup"
	"github.com/cng1985/nwatch/internal/modules/admin/logview"
	"github.com/cng1985/nwatch/internal/modules/admin/settings"
	"go.uber.org/fx"
)

// Module 只组合子模块，不包含业务代码。
var Module = fx.Options(
	access.Module,
	settings.Module,
	backup.Module,
	logview.Module,
)

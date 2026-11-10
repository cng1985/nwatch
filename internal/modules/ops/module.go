// Package ops 组合运维操作分组的子模块。
package ops

import (
	"github.com/cng1985/nwatch/internal/modules/ops/docker"
	"github.com/cng1985/nwatch/internal/modules/ops/kube"
	"github.com/cng1985/nwatch/internal/modules/ops/service"
	"github.com/cng1985/nwatch/internal/modules/ops/shell"
	"go.uber.org/fx"
)

// Module 只组合子模块，不包含业务代码。
var Module = fx.Options(
	shell.Module,
	service.Module,
	docker.Module,
	kube.Module,
)

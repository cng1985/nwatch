package access

import (
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册登录会话与用户角色管理业务。
var Module = fx.Module("access",
	fx.Provide(
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
)

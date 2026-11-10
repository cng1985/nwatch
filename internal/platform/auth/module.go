package auth

import "go.uber.org/fx"

// Module 提供身份、会话、角色和权限能力。
// 内置角色与管理员账号的幂等初始化由 app/bootstrap 编排。
var Module = fx.Module("auth",
	fx.Provide(NewStore, New),
)

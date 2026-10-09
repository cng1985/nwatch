package auth

import "go.uber.org/fx"

// Module 提供登录和令牌。
var Module = fx.Module("auth",
	fx.Provide(New),
)

package web

import "go.uber.org/fx"

// Module 注册前端静态资源。
var Module = fx.Module("web",
	fx.Invoke(RegisterStatic),
)

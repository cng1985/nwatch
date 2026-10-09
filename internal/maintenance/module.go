package maintenance

import "go.uber.org/fx"

// Module 提供备份和历史清理。Invoke 让它独立于 HTTP 服务启动。
var Module = fx.Module("maintenance",
	fx.Provide(New),
	fx.Invoke(func(*Service) {}),
)

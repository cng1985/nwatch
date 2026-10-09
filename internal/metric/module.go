package metric

import "go.uber.org/fx"

// Module 提供检测结果聚合。
var Module = fx.Module("metric",
	fx.Provide(NewAggregator),
)

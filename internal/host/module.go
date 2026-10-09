package host

import "go.uber.org/fx"

// Module 采集本机 CPU、内存和磁盘。
var Module = fx.Module("host",
	fx.Provide(NewCollector, NewSampler),
	fx.Invoke(func(*Sampler) {}),
)

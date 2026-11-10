package kube

import (
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/routing"
	"go.uber.org/fx"
)

// Module 注册 Kubernetes 控制业务。
var Module = fx.Module("kube",
	fx.Provide(
		newManager,
		NewResource,
		fx.Annotate(NewController, fx.As(new(routing.Registrar)), fx.ResultTags(`group:"registrars"`)),
	),
)

func newManager(cfg *config.Config) *Manager {
	return New(Options{
		Enabled:    cfg.Kubernetes.Enabled,
		Context:    cfg.Kubernetes.Context,
		Kubeconfig: cfg.Kubernetes.Kubeconfig,
	})
}

package app

import (
	"testing"

	"github.com/cng1985/nwatch/internal/platform/config"
	"go.uber.org/fx"
)

func TestModuleGraph(t *testing.T) {
	// 依赖图校验：配置由组合根直接供给，验证平台、业务与 Web 模块能完成装配。
	if err := fx.ValidateApp(fx.NopLogger, fx.Supply(config.Default()), Module); err != nil {
		t.Fatal(err)
	}
}

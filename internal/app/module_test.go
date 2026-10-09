package app

import (
	"testing"

	"go.uber.org/fx"
)

func TestModuleGraph(t *testing.T) {
	if err := fx.ValidateApp(fx.NopLogger, Module); err != nil {
		t.Fatal(err)
	}
}

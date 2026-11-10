package checker

import (
	"context"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
)

type Checker interface {
	Type() string
	Check(ctx context.Context, monitor *shared.Monitor) (*Result, error)
}

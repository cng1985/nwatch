package checker

import (
	"context"

	"github.com/cng1985/nwatch/internal/model"
)

type Checker interface {
	Type() string
	Check(ctx context.Context, monitor *model.Monitor) (*Result, error)
}

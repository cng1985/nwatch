package notifier

import (
	"context"

	"github.com/cng1985/nwatch/internal/model"
)

type Notifier interface {
	Type() string
	Send(ctx context.Context, cfg *model.Notifier, event *model.AlertEvent) (response string, err error)
}

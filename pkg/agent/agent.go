package agent

import (
	"context"
	"errors"

	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
)

type Agent interface {
	Invoke(
		ctx context.Context,
		checkpoint models.Checkpoint,
		userMessage string,
		tools ...*tools.Tool,
	) (models.Checkpoint, error)

	InvokeStream(
		ctx context.Context,
		checkpoint models.Checkpoint,
		userMessage string,
		onContent func(string),
		tools ...*tools.Tool,
	) (models.Checkpoint, error)
}

var ErrNoResponse = errors.New("no response returned")

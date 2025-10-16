package agent

import (
	"context"

	"github.com/raphael-foliveira/tca/pkg/models"
)

type Summarizer interface {
	Summarize(
		ctx context.Context,
		checkpoint models.Checkpoint,
	) (models.Checkpoint, error)
}

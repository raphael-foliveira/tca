package agent

import (
	"context"

	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
)

type ChatCompletionRequest struct {
	Model    string
	Messages []models.ContextMsg
	Tools    []*tools.Tool
}

type ChatCompletionResponse struct {
	Message      models.ContextMsg
	InputTokens  int64
	OutputTokens int64
}

type ChatCompletionClient interface {
	Complete(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error)

	CompleteStreaming(
		ctx context.Context,
		req ChatCompletionRequest,
		onDelta func(content string) error,
	) (ChatCompletionResponse, error)
}

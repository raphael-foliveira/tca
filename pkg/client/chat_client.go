package client

import (
	"context"

	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
)

type ChatCompletionParams struct {
	Model    string
	Messages []models.ContextMsg
	Tools    []*tools.Tool
}

type ChatCompletionChunk struct {
	ID      string
	Choice  ChatCompletionChunkChoice
	Created int64
	Model   string
	Usage   CompletionUsage
}

type ChatCompletionChunkChoice struct {
	Delta        ChatCompletionChunkDelta
	FinishReason string
	Index        int64
}

type ChatCompletionChunkDelta struct {
	Role      models.MessageRole
	Content   string
	Refusal   string
	ToolCalls []ChatCompletionChunkDeltaToolCall
}

type ChatCompletionChunkDeltaToolCall struct {
	Index    int64
	ID       string
	Type     string
	Function ChatCompletionChunkDeltaToolCallFunction
}

type ChatCompletionChunkDeltaToolCallFunction struct {
	Name      string
	Arguments string
}

type CompletionUsage struct {
	CompletionTokens int64 `json:"completion_tokens"`
	PromptTokens     int64 `json:"prompt_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type ChatCompletion struct {
	Message models.ContextMsg
	Usage   CompletionUsage
}

type ChatCompletionClient interface {
	New(
		ctx context.Context,
		params ChatCompletionParams,
	) (*ChatCompletion, error)

	NewStreaming(
		ctx context.Context,
		params ChatCompletionParams,
	) ChatCompletionStream
}

type ChatCompletionStream interface {
	Close() error
	Err() error
	Next() bool
	Current() ChatCompletionChunk
}

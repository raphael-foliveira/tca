package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/raphael-foliveira/tca/pkg/client"
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

type agent struct {
	completionClient client.ChatCompletionClient
	tools            []*tools.Tool
	model            string
	systemPrompt     string
}

type AgentConfig struct {
	ChatClient   client.ChatCompletionClient
	Model        string
	SystemPrompt string
	Tools        []*tools.Tool
}

func NewAgent(config AgentConfig) Agent {
	return &agent{
		completionClient: config.ChatClient,
		tools:            config.Tools,
		model:            config.Model,
		systemPrompt:     config.SystemPrompt,
	}
}

func (a *agent) Invoke(
	ctx context.Context,
	checkpoint models.Checkpoint,
	userMessage string,
	extraTools ...*tools.Tool,
) (models.Checkpoint, error) {
	context := append(checkpoint.Context, models.ContextMsg{
		Role:    models.MessageRoleUser,
		Content: userMessage,
	})

	for range 10 {
		completion, err := a.completionClient.New(
			ctx,
			client.ChatCompletionParams{
				Model:    a.model,
				Messages: prependMessage(context, a.systemPrompt),
				Tools:    append(a.tools, extraTools...),
			},
		)
		if err != nil {
			return models.Checkpoint{}, err
		}

		msg := completion.Message
		context = append(context, msg)

		if len(msg.ToolCalls) == 0 {
			return models.Checkpoint{
				SessionID:    checkpoint.SessionID,
				Context:      context,
				Prompt:       userMessage,
				Response:     msg.Content,
				InputTokens:  completion.Usage.PromptTokens,
				OutputTokens: completion.Usage.CompletionTokens,
			}, nil
		}

		toolResults, err := a.executeToolCalls(ctx, msg.ToolCalls, extraTools...)
		if err != nil {
			return models.Checkpoint{}, err
		}

		context = append(context, toolResults...)
	}
	return models.Checkpoint{}, errors.New("failed to get final response after 10 retries")
}

func (a *agent) InvokeStream(
	ctx context.Context,
	checkpoint models.Checkpoint,
	userMessage string,
	onContent func(string),
	extraTools ...*tools.Tool,
) (models.Checkpoint, error) {
	context := append(checkpoint.Context, models.ContextMsg{
		Role:    models.MessageRoleUser,
		Content: userMessage,
	})

	for range 10 {
		finalCheckpoint, err := func() (*models.Checkpoint, error) {
			stream := a.completionClient.NewStreaming(
				ctx,
				client.ChatCompletionParams{
					Model:    a.model,
					Messages: prependMessage(context, a.systemPrompt),
					Tools:    append(a.tools, extraTools...),
				},
			)
			defer stream.Close()

			acc := client.NewChatCompletionStreamAccumulator()
			for stream.Next() {
				chunk := stream.Current()
				acc.AddChunk(chunk)

				onContent(chunk.Choice.Delta.Content)
				if chunk.Choice.FinishReason != "" {
					completion := acc.Result()
					if refusal := acc.Refusal(); refusal != "" &&
						completion.Message.Content == "" && len(completion.Message.ToolCalls) == 0 {
						return nil, fmt.Errorf("refusal: %s", refusal)
					}
				}
			}
			if err := stream.Err(); err != nil {
				return nil, err
			}

			if !acc.HasContent() {
				return nil, ErrNoResponse
			}

			completion := acc.Result()
			if refusal := acc.Refusal(); refusal != "" &&
				completion.Message.Content == "" && len(completion.Message.ToolCalls) == 0 {
				return nil, fmt.Errorf("refusal: %s", refusal)
			}

			context = append(context, completion.Message)

			if len(completion.Message.ToolCalls) == 0 {
				return &models.Checkpoint{
					SessionID:    checkpoint.SessionID,
					Context:      context,
					Prompt:       userMessage,
					Response:     completion.Message.Content,
					InputTokens:  completion.Usage.PromptTokens,
					OutputTokens: completion.Usage.CompletionTokens,
				}, nil
			}

			msgs, err := a.executeToolCalls(ctx, completion.Message.ToolCalls, extraTools...)
			if err != nil {
				return nil, err
			}
			context = append(context, msgs...)
			return nil, nil
		}()
		if err != nil {
			return models.Checkpoint{}, err
		}
		if finalCheckpoint != nil {
			return *finalCheckpoint, nil
		}
	}

	return models.Checkpoint{}, errors.New("failed to get final response after 10 retries")
}

func (a *agent) executeToolCalls(
	ctx context.Context,
	toolCalls []models.ToolCall,
	extraTools ...*tools.Tool,
) ([]models.ContextMsg, error) {
	results := []models.ContextMsg{}

	for _, toolCall := range toolCalls {
		tool := a.findToolByName(toolCall.Name, extraTools...)
		if tool == nil {
			return nil, fmt.Errorf("tool not found: %s", toolCall.Name)
		}

		result, err := tool.Run(ctx, toolCall.Arguments)
		if err != nil {
			return nil, err
		}

		results = append(results, models.ContextMsg{
			Role:       models.MessageRoleTool,
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	return results, nil
}

func prependMessage(messages []models.ContextMsg, message string) []models.ContextMsg {
	if message == "" {
		return messages
	}
	return append([]models.ContextMsg{{Role: models.MessageRoleSystem, Content: message}}, messages...)
}

func (a *agent) findToolByName(name string, extraTools ...*tools.Tool) *tools.Tool {
	for _, tool := range append(a.tools, extraTools...) {
		if tool.GetName() == name {
			return tool
		}
	}
	return nil
}

var ErrNoResponse = errors.New("no response returned")

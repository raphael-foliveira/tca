package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
)

type chatAgent struct {
	completionClient ChatCompletionClient
	tools            []*tools.Tool
	systemPrompt     string
}

type AgentConfig struct {
	ChatClient   ChatCompletionClient
	SystemPrompt string
	Tools        []*tools.Tool
}

func NewAgent(config AgentConfig) Agent {
	return &chatAgent{
		completionClient: config.ChatClient,
		tools:            config.Tools,
		systemPrompt:     config.SystemPrompt,
	}
}

func (a *chatAgent) Invoke(
	ctx context.Context,
	checkpoint models.Checkpoint,
	userMessage string,
	extraTools ...*tools.Tool,
) (models.Checkpoint, error) {
	messages := append(checkpoint.Context, models.ContextMsg{
		Role:    models.MessageRoleUser,
		Content: userMessage,
	})

	if a.systemPrompt != "" {
		messages = append([]models.ContextMsg{
			{
				Role:    models.MessageRoleSystem,
				Content: a.systemPrompt,
			},
		}, messages...)
	}

	for range 10 {
		req := ChatCompletionRequest{
			Messages: messages,
			Tools:    append(a.tools, extraTools...),
		}

		completion, err := a.completionClient.Complete(ctx, req)
		if err != nil {
			return models.Checkpoint{}, err
		}

		messages = append(messages, completion.Message)

		if len(completion.Message.ToolCalls) == 0 {
			return models.Checkpoint{
				SessionID:    checkpoint.SessionID,
				Context:      messages,
				Prompt:       userMessage,
				Response:     completion.Message.Content,
				InputTokens:  completion.InputTokens,
				OutputTokens: completion.OutputTokens,
			}, nil
		}

		toolResults, err := a.executeCalls(
			ctx,
			completion.Message.ToolCalls,
			extraTools...,
		)
		if err != nil {
			return models.Checkpoint{}, err
		}

		messages = append(messages, toolResults...)
	}
	return models.Checkpoint{}, errors.New("failed to get final response after 10 retries")
}

func (a *chatAgent) executeCalls(
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

func (a *chatAgent) findToolByName(name string, extraTools ...*tools.Tool) *tools.Tool {
	for _, tool := range append(a.tools, extraTools...) {
		if tool.GetName() == name {
			return tool
		}
	}
	return nil
}

func (a *chatAgent) InvokeStream(
	ctx context.Context,
	checkpoint models.Checkpoint,
	userMessage string,
	onContent func(string) error,
	extraTools ...*tools.Tool,
) (models.Checkpoint, error) {
	messages := append(checkpoint.Context, models.ContextMsg{
		Role:    models.MessageRoleUser,
		Content: userMessage,
	})

	if a.systemPrompt != "" {
		messages = append([]models.ContextMsg{
			{
				Role:    models.MessageRoleSystem,
				Content: a.systemPrompt,
			},
		}, messages...)
	}

	for range 10 {
		req := ChatCompletionRequest{
			Messages: messages,
			Tools:    append(a.tools, extraTools...),
		}

		completion, err := a.completionClient.CompleteStreaming(ctx, req, onContent)
		if err != nil {
			return models.Checkpoint{}, err
		}

		messages = append(messages, completion.Message)

		if len(completion.Message.ToolCalls) == 0 {
			return models.Checkpoint{
				SessionID:    checkpoint.SessionID,
				Context:      messages,
				Prompt:       userMessage,
				Response:     completion.Message.Content,
				InputTokens:  completion.InputTokens,
				OutputTokens: completion.OutputTokens,
			}, nil
		}

		toolResults, err := a.executeCalls(
			ctx,
			completion.Message.ToolCalls,
			extraTools...,
		)
		if err != nil {
			return models.Checkpoint{}, err
		}

		messages = append(messages, toolResults...)
	}

	return models.Checkpoint{}, errors.New("failed to get final response after 10 retries")
}

type openaiSummarizer struct {
	chatClient                ChatCompletionClient
	tokensBeforeSummarization int64
	maxSummaryLength          int64
	summaryPrompt             string
	messagesToKeep            int
}

type SummarizerConfig struct {
	ChatClient                ChatCompletionClient
	TokensBeforeSummarization int64
	MaxSummaryLength          int64
	SummaryPrompt             string
	MessagesToKeep            int
}

func (o *SummarizerConfig) applyDefaults() {
	if o.SummaryPrompt == "" {
		o.SummaryPrompt = `
		You are an assistant that summarizes the conversation history.
		Summarize the following conversation into a single message.
		Your response must clearly indicate that it is a message intended to help yourself have more context
		about the conversation without having to see all of the conversation history.
		Refer to the user's messages as "user" and to your own generated information as "me" or "I".
		`
	}
	if o.MessagesToKeep == 0 {
		o.MessagesToKeep = 5
	}
	if o.TokensBeforeSummarization == 0 {
		o.TokensBeforeSummarization = 16384
	}
	if o.MaxSummaryLength == 0 {
		o.MaxSummaryLength = 2048
	}
}

func NewSummarizer(config SummarizerConfig) Summarizer {
	config.applyDefaults()
	return &openaiSummarizer{
		chatClient:                config.ChatClient,
		tokensBeforeSummarization: config.TokensBeforeSummarization,
		maxSummaryLength:          config.MaxSummaryLength,
		summaryPrompt:             config.SummaryPrompt,
		messagesToKeep:            config.MessagesToKeep,
	}
}

func (s *openaiSummarizer) Summarize(
	ctx context.Context,
	checkpoint models.Checkpoint,
) (models.Checkpoint, error) {
	if !s.shouldSummarize(checkpoint) {
		return checkpoint, nil
	}
	if s.chatClient == nil {
		return models.Checkpoint{}, errors.New("chat completion client is not configured")
	}

	messages := checkpoint.Context
	cutoffIndex := len(messages) - s.messagesToKeep
	if cutoffIndex <= s.messagesToKeep || messages[cutoffIndex].Role == models.MessageRoleTool {
		return checkpoint, nil
	}

	messagesToSummarize := slices.Clone(messages[:cutoffIndex])
	messagesToKeep := slices.Clone(messages[cutoffIndex:])

	prompt := "Summarize the above conversation according to the instructions."

	concatenatedMessages := slices.Concat(
		[]models.ContextMsg{
			{
				Role:    models.MessageRoleSystem,
				Content: s.summaryPrompt,
			},
		},
		messagesToSummarize,
		[]models.ContextMsg{
			{
				Role:    models.MessageRoleUser,
				Content: prompt,
			},
		},
	)

	req := ChatCompletionRequest{
		Messages: concatenatedMessages,
		Tools:    []*tools.Tool{},
	}

	completion, err := s.chatClient.Complete(ctx, req)
	if err != nil {
		return models.Checkpoint{}, err
	}

	newSummary := completion.Message
	newContext := append([]models.ContextMsg{newSummary}, messagesToKeep...)

	newCheckpoint := models.Checkpoint{
		SessionID:    checkpoint.SessionID,
		Context:      newContext,
		Prompt:       prompt,
		Response:     newSummary.Content,
		InputTokens:  completion.InputTokens,
		OutputTokens: completion.OutputTokens,
		IsSummary:    true,
	}
	return newCheckpoint, nil
}

func (s *openaiSummarizer) shouldSummarize(checkpoint models.Checkpoint) bool {
	return checkpoint.TotalTokens() > s.tokensBeforeSummarization
}

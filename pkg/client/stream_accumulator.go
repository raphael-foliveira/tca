package client

import (
	"strings"

	"github.com/raphael-foliveira/tca/pkg/models"
)

type ChatCompletionStreamAccumulator struct {
	usage       CompletionUsage
	role        models.MessageRole
	content     strings.Builder
	toolCalls   []*streamToolCallBuilder
	refusal     strings.Builder
	seenRefusal bool
}

type streamToolCallBuilder struct {
	id        string
	name      strings.Builder
	arguments strings.Builder
}

func NewChatCompletionStreamAccumulator() *ChatCompletionStreamAccumulator {
	return &ChatCompletionStreamAccumulator{
		role: models.MessageRoleAssistant,
	}
}

func (a *ChatCompletionStreamAccumulator) AddChunk(chunk ChatCompletionChunk) {
	a.usage.CompletionTokens += chunk.Usage.CompletionTokens
	a.usage.PromptTokens += chunk.Usage.PromptTokens
	a.usage.TotalTokens += chunk.Usage.TotalTokens
	choice := chunk.Choice

	delta := choice.Delta

	if delta.Role != "" {
		a.role = delta.Role
	}

	if delta.Content != "" {
		a.content.WriteString(delta.Content)
	}

	if delta.Refusal != "" {
		a.refusal.WriteString(delta.Refusal)
		a.seenRefusal = true
	}

	for _, tc := range delta.ToolCalls {
		builder := a.ensureToolCall(int(tc.Index))
		if tc.ID != "" {
			builder.id = tc.ID
		}
		if tc.Function.Name != "" {
			builder.name.WriteString(tc.Function.Name)
		}
		if tc.Function.Arguments != "" {
			builder.arguments.WriteString(tc.Function.Arguments)
		}
	}
}

func (a *ChatCompletionStreamAccumulator) ensureToolCall(index int) *streamToolCallBuilder {
	for len(a.toolCalls) <= index {
		a.toolCalls = append(a.toolCalls, nil)
	}
	if a.toolCalls[index] == nil {
		a.toolCalls[index] = &streamToolCallBuilder{}
	}
	return a.toolCalls[index]
}

func (a *ChatCompletionStreamAccumulator) Result() ChatCompletion {
	msg := models.ContextMsg{
		Role:    a.role,
		Content: a.content.String(),
	}

	if len(a.toolCalls) > 0 {
		calls := make([]models.ToolCall, 0, len(a.toolCalls))
		for _, builder := range a.toolCalls {
			if builder == nil {
				continue
			}
			calls = append(calls, models.ToolCall{
				ID:        builder.id,
				Name:      builder.name.String(),
				Arguments: builder.arguments.String(),
			})
		}
		if len(calls) > 0 {
			msg.ToolCalls = calls
		}
	}

	return ChatCompletion{
		Message: msg,
		Usage:   a.usage,
	}
}

func (a *ChatCompletionStreamAccumulator) Refusal() string {
	if !a.seenRefusal {
		return ""
	}
	return a.refusal.String()
}

func (a *ChatCompletionStreamAccumulator) HasContent() bool {
	if a.content.Len() > 0 {
		return true
	}
	if a.seenRefusal {
		return true
	}
	for _, builder := range a.toolCalls {
		if builder == nil {
			continue
		}
		if builder.name.Len() > 0 || builder.arguments.Len() > 0 || builder.id != "" {
			return true
		}
	}
	return false
}

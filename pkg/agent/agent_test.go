package agent_test

import (
	"context"
	"testing"

	"github.com/raphael-foliveira/tca/mocks"
	"github.com/raphael-foliveira/tca/pkg/agent"
	"github.com/raphael-foliveira/tca/pkg/client"
	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOpenaiAgentInvokeReturnsCheckpoint(t *testing.T) {
	mockClient := mocks.NewMockChatCompletionClient(t)
	completion := &client.ChatCompletion{
		Message: models.ContextMsg{
			Role:    models.MessageRoleAssistant,
			Content: "assistant response",
		},
		Usage: client.CompletionUsage{
			PromptTokens:     4,
			CompletionTokens: 6,
			TotalTokens:      10,
		},
	}

	var captured client.ChatCompletionParams
	mockClient.EXPECT().New(mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context,
			params client.ChatCompletionParams,
		) (*client.ChatCompletion, error) {
			captured = params
			return completion, nil
		})

	testAgent := agent.NewAgent(agent.AgentConfig{
		ChatClient:   mockClient,
		Model:        "test-model",
		SystemPrompt: "be helpful",
	})

	checkpoint, err := testAgent.Invoke(context.Background(), models.Checkpoint{SessionID: "session", Context: []models.ContextMsg{}}, "hello")
	require.NoError(t, err)

	assert.Equal(t, "assistant response", checkpoint.Response)
	assert.Equal(t, int64(4), checkpoint.InputTokens)
	assert.Equal(t, int64(6), checkpoint.OutputTokens)
	assert.Equal(t, "test-model", captured.Model)

	if assert.GreaterOrEqual(t, len(captured.Messages), 2) {
		assert.Equal(t, models.MessageRoleSystem, captured.Messages[0].Role)
		assert.Equal(t, "be helpful", captured.Messages[0].Content)

		last := captured.Messages[len(captured.Messages)-1]
		assert.Equal(t, models.MessageRoleUser, last.Role)
		assert.Equal(t, "hello", last.Content)
	}

	assert.Len(t, checkpoint.Context, 2)
}

func TestOpenaiAgentInvokeExecutesTools(t *testing.T) {
	type toolParams struct {
		Query string `json:"query"`
	}

	var receivedParams toolParams
	tool := tools.NewTool("search", "search tool", func(_ context.Context, params toolParams) (string, error) {
		receivedParams = params
		return "result from tool", nil
	})

	toolCallArguments := `{"query":"answer me"}`

	mockClient := mocks.NewMockChatCompletionClient(t)
	firstCompletion := &client.ChatCompletion{
		Message: models.ContextMsg{
			Role: models.MessageRoleAssistant,
			ToolCalls: []models.ToolCall{
				{
					ID:        "tool_1",
					Name:      "search",
					Arguments: toolCallArguments,
				},
			},
		},
	}
	secondCompletion := &client.ChatCompletion{
		Message: models.ContextMsg{
			Role:    models.MessageRoleAssistant,
			Content: "done",
		},
		Usage: client.CompletionUsage{
			PromptTokens:     3,
			CompletionTokens: 4,
			TotalTokens:      7,
		},
	}

	mockClient.EXPECT().New(mock.Anything, mock.Anything).
		Return(firstCompletion, nil).Once()
	mockClient.EXPECT().New(mock.Anything, mock.Anything).
		Return(secondCompletion, nil).Once()

	agent := agent.NewAgent(agent.AgentConfig{
		ChatClient:   mockClient,
		Model:        "test-model",
		SystemPrompt: "be helpful",
		Tools:        []*tools.Tool{tool},
	})

	checkpoint, err := agent.Invoke(
		context.Background(),
		models.Checkpoint{SessionID: "session", Context: []models.ContextMsg{}},
		"hello",
	)
	require.NoError(t, err)

	assert.Equal(t, "answer me", receivedParams.Query)
	assert.Equal(t, "done", checkpoint.Response)
	assert.Equal(t, int64(3), checkpoint.InputTokens)
	assert.Equal(t, int64(4), checkpoint.OutputTokens)
}

func TestOpenaiAgentInvokeStreamReturnsFinalCheckpoint(t *testing.T) {
	mockClient := mocks.NewMockChatCompletionClient(t)
	mockStream := mocks.NewMockChatCompletionStream(t)

	chunk := client.ChatCompletionChunk{
		ID:      "chunk-1",
		Created: 123,
		Model:   "test-model",
		Choice: client.ChatCompletionChunkChoice{
			Index: 0,
			Delta: client.ChatCompletionChunkDelta{
				Role:    models.MessageRoleAssistant,
				Content: "hi",
			},
		},
		Usage: client.CompletionUsage{
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
		},
	}

	mockStream.EXPECT().Next().Return(true).Once()
	mockStream.EXPECT().Current().Return(chunk).Once()
	mockStream.EXPECT().Next().Return(false).Once()
	mockStream.EXPECT().Err().Return(nil).Once()
	mockStream.EXPECT().Close().Return(nil).Once()

	mockClient.EXPECT().NewStreaming(mock.Anything, mock.Anything).
		Return(mockStream)

	agent := agent.NewAgent(agent.AgentConfig{
		ChatClient:   mockClient,
		Model:        "test-model",
		SystemPrompt: "be helpful",
	})

	var streamed string
	checkpoint, err := agent.InvokeStream(
		context.Background(),
		models.Checkpoint{SessionID: "session", Context: []models.ContextMsg{}},
		"hello",
		func(content string) {
			streamed += content
		},
	)
	require.NoError(t, err)

	assert.Equal(t, "hi", streamed)
	assert.Equal(t, "hi", checkpoint.Response)
	assert.Equal(t, int64(1), checkpoint.InputTokens)
	assert.Equal(t, int64(2), checkpoint.OutputTokens)
}

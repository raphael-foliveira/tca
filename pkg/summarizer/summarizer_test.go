package summarizer_test

import (
	"context"
	"testing"

	"github.com/raphael-foliveira/tca/mocks"
	"github.com/raphael-foliveira/tca/pkg/client"
	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/summarizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOpenaiSummarizerSkipsWhenBelowThreshold(t *testing.T) {
	mockClient := mocks.NewMockChatCompletionClient(t)
	summarizer := summarizer.NewSummarizer(summarizer.SummarizerConfig{
		ChatClient:                mockClient,
		Model:                     "test-model",
		TokensBeforeSummarization: 100,
	})

	checkpoint := models.Checkpoint{SessionID: "session", Context: []models.ContextMsg{}, InputTokens: 1, OutputTokens: 1}

	result, err := summarizer.Summarize(context.Background(), checkpoint)
	require.NoError(t, err)

	mockClient.AssertNotCalled(t, "New", mock.Anything, mock.Anything)
	assert.Equal(t, checkpoint.SessionID, result.SessionID)
	assert.Equal(t, checkpoint.Context, result.Context)
}

func TestOpenaiSummarizerSummarizesConversation(t *testing.T) {
	messages := []models.ContextMsg{
		{
			Role:    models.MessageRoleSystem,
			Content: "system",
		},
		{
			Role:    models.MessageRoleUser,
			Content: "message 1",
		},
		{
			Role:    models.MessageRoleAssistant,
			Content: "assistant reply",
		},
		{
			Role:    models.MessageRoleUser,
			Content: "message 2",
		},
	}
	mockClient := mocks.NewMockChatCompletionClient(t)
	completion := &client.ChatCompletion{
		Message: models.ContextMsg{
			Role:    models.MessageRoleAssistant,
			Content: "summary result",
		},
		Usage: client.CompletionUsage{
			PromptTokens:     2,
			CompletionTokens: 3,
			TotalTokens:      5,
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

	summarizer := summarizer.NewSummarizer(summarizer.SummarizerConfig{
		ChatClient:                mockClient,
		Model:                     "test-model",
		TokensBeforeSummarization: 1,
		MessagesToKeep:            1,
		SummaryPrompt:             "custom summary prompt",
	})

	checkpoint := models.Checkpoint{
		SessionID:    "session",
		Context:      messages,
		InputTokens:  10,
		OutputTokens: 0,
	}

	result, err := summarizer.Summarize(context.Background(), checkpoint)
	require.NoError(t, err)

	assert.Equal(t, "test-model", captured.Model)
	assert.Equal(t, 5, len(captured.Messages))
	assert.Equal(t, models.MessageRoleSystem, captured.Messages[0].Role)
	assert.Equal(t, "custom summary prompt", captured.Messages[0].Content)
	last := captured.Messages[len(captured.Messages)-1]
	assert.Equal(t, models.MessageRoleUser, last.Role)
	assert.Equal(t, "Summarize the above conversation according to the instructions.", last.Content)

	assert.True(t, result.IsSummary)
	assert.Equal(t, "summary result", result.Response)
	assert.Equal(t, int64(2), result.InputTokens)
	assert.Equal(t, int64(3), result.OutputTokens)
	assert.Equal(t, "Summarize the above conversation according to the instructions.", result.Prompt)

	newMessages := result.Context
	assert.Len(t, newMessages, 2)

	assert.Equal(t, models.MessageRoleAssistant, newMessages[0].Role)
	assert.Equal(t, "summary result", newMessages[0].Content)
	assert.Equal(t, models.MessageRoleUser, newMessages[1].Role)
	assert.Equal(t, "message 2", newMessages[1].Content)
}

func TestOpenaiSummarizerErrorsWithoutChatClient(t *testing.T) {
	messages := []models.ContextMsg{
		{
			Role:    models.MessageRoleSystem,
			Content: "system",
		},
		{
			Role:    models.MessageRoleUser,
			Content: "message 1",
		},
		{
			Role:    models.MessageRoleAssistant,
			Content: "assistant reply",
		},
		{
			Role:    models.MessageRoleUser,
			Content: "message 2",
		},
		{
			Role:    models.MessageRoleUser,
			Content: "message 2",
		},
	}

	summarizer := summarizer.NewSummarizer(summarizer.SummarizerConfig{
		Model:                     "test-model",
		TokensBeforeSummarization: 1,
		MessagesToKeep:            1,
	})

	checkpoint := models.Checkpoint{
		SessionID:    "session",
		Context:      messages,
		InputTokens:  10,
		OutputTokens: 0,
	}

	_, err := summarizer.Summarize(context.Background(), checkpoint)
	assert.Error(t, err)
}

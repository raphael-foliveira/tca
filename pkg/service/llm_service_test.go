package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/raphael-foliveira/tca/mocks"
	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/service"
	"github.com/raphael-foliveira/tca/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLLMServiceInvoke(t *testing.T) {
	baseCheckpoint := models.Checkpoint{SessionID: "session", Context: []models.ContextMsg{}}

	repo := mocks.NewMockCheckpointsRepository(t)
	repo.EXPECT().GetLatestBySessionID(mock.Anything, "session").Return(baseCheckpoint, nil)
	repo.EXPECT().Create(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, checkpoint models.Checkpoint) (models.Checkpoint, error) {
			return checkpoint, nil
		})

	summarizedCheckpoint := baseCheckpoint
	summarizedCheckpoint.Response = "summarized"

	summarizer := mocks.NewMockSummarizer(t)
	summarizer.EXPECT().Summarize(mock.Anything, baseCheckpoint).Return(summarizedCheckpoint, nil)

	finalCheckpoint := summarizedCheckpoint
	finalCheckpoint.Response = "agent response"

	agent := mocks.NewMockAgent(t)
	agent.EXPECT().Invoke(mock.Anything, summarizedCheckpoint, "hello").Return(finalCheckpoint, nil)

	service := service.NewLLMService(agent, repo, summarizer)

	resp, err := service.Invoke(context.Background(), "session", "hello")
	require.NoError(t, err)
	assert.Equal(t, "agent response", resp)
}

func TestLLMServiceInvokeStream(t *testing.T) {
	repo := mocks.NewMockCheckpointsRepository(t)
	baseCheckpoint := models.Checkpoint{SessionID: "session", Context: []models.ContextMsg{}}
	repo.EXPECT().GetLatestBySessionID(mock.Anything, "session").Return(baseCheckpoint, nil)
	repo.EXPECT().Create(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, checkpoint models.Checkpoint) (models.Checkpoint, error) {
			return checkpoint, nil
		})

	summarizer := mocks.NewMockSummarizer(t)
	summarizer.EXPECT().Summarize(mock.Anything, baseCheckpoint).Return(baseCheckpoint, nil)

	finalCheckpoint := baseCheckpoint
	finalCheckpoint.Response = "stream response"

	agent := mocks.NewMockAgent(t)
	agent.EXPECT().InvokeStream(mock.Anything, baseCheckpoint, "hello", mock.Anything).
		RunAndReturn(func(ctx context.Context, checkpoint models.Checkpoint, msg string, onChunk func(string) error, tools ...*tools.Tool) (models.Checkpoint, error) {
			if err := onChunk("chunk-data"); err != nil {
				return models.Checkpoint{}, err
			}
			return finalCheckpoint, nil
		})

	service := service.NewLLMService(agent, repo, summarizer)

	var collected string
	err := service.InvokeStream(context.Background(), "session", "hello", func(content string) error {
		collected += content
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "chunk-data", collected)
}

func TestLLMServiceInvokePropagatesRepositoryError(t *testing.T) {
	repo := mocks.NewMockCheckpointsRepository(t)
	repo.EXPECT().GetLatestBySessionID(mock.Anything, "session").Return(models.Checkpoint{}, errors.New("boom"))

	summarizer := mocks.NewMockSummarizer(t)
	agent := mocks.NewMockAgent(t)

	service := service.NewLLMService(agent, repo, summarizer)

	_, err := service.Invoke(context.Background(), "session", "hello")
	assert.Error(t, err)
}

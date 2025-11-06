package service

import (
	"context"
	"fmt"

	"github.com/raphael-foliveira/tca/pkg/agent"
	"github.com/raphael-foliveira/tca/pkg/repository"
)

type LLMService interface {
	Invoke(
		ctx context.Context,
		sessionID string,
		newMessage string,
	) (string, error)

	InvokeStream(
		ctx context.Context,
		sessionID string,
		newMessage string,
		onChunk func(content string) error,
	) error
}

type llmService struct {
	agent                 agent.Agent
	checkpointsRepository repository.CheckpointsRepository
	summarizer            agent.Summarizer
}

func NewLLMService(
	agent agent.Agent,
	messagesRepository repository.CheckpointsRepository,
	summarizer agent.Summarizer,
) LLMService {
	return &llmService{
		agent:                 agent,
		checkpointsRepository: messagesRepository,
		summarizer:            summarizer,
	}
}

func (s *llmService) Invoke(ctx context.Context, sessionID string, newMessage string) (string, error) {
	checkpoint, err := s.checkpointsRepository.GetLatestBySessionID(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get saved messages: %w", err)
	}
	checkpoint, err = s.summarizer.Summarize(ctx, checkpoint)
	if err != nil {
		return "", fmt.Errorf("failed to summarize messages: %w", err)
	}

	responses, err := s.agent.Invoke(ctx, checkpoint, newMessage)
	if err != nil {
		return "", fmt.Errorf("failed to invoke agent: %w", err)
	}

	if _, err := s.checkpointsRepository.Create(ctx, responses); err != nil {
		return "", fmt.Errorf("failed to save messages: %w", err)
	}

	return responses.Response, nil
}

func (s *llmService) InvokeStream(
	ctx context.Context,
	sessionID string,
	newMessage string,
	onChunk func(content string) error,
) error {
	checkpoint, err := s.checkpointsRepository.GetLatestBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get saved messages: %w", err)
	}

	checkpoint, err = s.summarizer.Summarize(ctx, checkpoint)
	if err != nil {
		return fmt.Errorf("failed to handle summarization: %w", err)
	}

	finalCheckpoint, err := s.agent.InvokeStream(
		ctx,
		checkpoint,
		newMessage,
		onChunk,
	)
	if err != nil {
		return fmt.Errorf("failed to invoke agent: %w", err)
	}

	if _, err := s.checkpointsRepository.Create(ctx, finalCheckpoint); err != nil {
		return fmt.Errorf("failed to save messages: %w", err)
	}

	return nil
}

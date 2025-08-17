package summarizer

import (
	"context"
	"errors"
	"slices"

	"github.com/raphael-foliveira/tca/pkg/client"
	"github.com/raphael-foliveira/tca/pkg/models"
)

type Summarizer interface {
	Summarize(ctx context.Context, checkpoint models.Checkpoint) (models.Checkpoint, error)
}

type summarizer struct {
	chatClient                client.ChatCompletionClient
	model                     string
	tokensBeforeSummarization int64
	maxSummaryLength          int64
	summaryPrompt             string
	messagesToKeep            int
}

type SummarizerConfig struct {
	ChatClient                client.ChatCompletionClient
	Model                     string
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
	return &summarizer{
		chatClient:                config.ChatClient,
		model:                     config.Model,
		tokensBeforeSummarization: config.TokensBeforeSummarization,
		maxSummaryLength:          config.MaxSummaryLength,
		summaryPrompt:             config.SummaryPrompt,
		messagesToKeep:            config.MessagesToKeep,
	}
}

func (s *summarizer) Summarize(
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

	completion, err := s.chatClient.New(
		ctx, client.ChatCompletionParams{
			Model:    s.model,
			Messages: concatenatedMessages,
		},
	)
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
		InputTokens:  completion.Usage.PromptTokens,
		OutputTokens: completion.Usage.CompletionTokens,
		IsSummary:    true,
	}
	return newCheckpoint, nil
}

func (s *summarizer) shouldSummarize(checkpoint models.Checkpoint) bool {
	return checkpoint.TotalTokens() > s.tokensBeforeSummarization
}

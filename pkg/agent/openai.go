package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
	"github.com/raphael-foliveira/tca/pkg/utils"
)

type openaiAgent struct {
	completionClient *openai.ChatCompletionService
	toolHandler      *tools.ToolHandler
	model            openai.ChatModel
	systemPrompt     string
}

type OpenaiAgentConfig struct {
	ChatClient   *openai.ChatCompletionService
	Model        openai.ChatModel
	SystemPrompt string
	ToolHandler  *tools.ToolHandler
}

func NewOpenai(config OpenaiAgentConfig) Agent {
	return &openaiAgent{
		completionClient: config.ChatClient,
		toolHandler:      config.ToolHandler,
		model:            config.Model,
		systemPrompt:     config.SystemPrompt,
	}
}

func (a *openaiAgent) Invoke(
	ctx context.Context,
	checkpoint models.Checkpoint,
	userMessage string,
	extraTools ...*tools.Tool,
) (models.Checkpoint, error) {
	context := utils.MapMany(append(checkpoint.Context, models.ContextMsg{
		Role:    models.MessageRoleUser,
		Content: userMessage,
	}), toOpenAIMessageParam)

	for range 10 {
		completion, err := a.completionClient.New(
			ctx,
			openai.ChatCompletionNewParams{
				Model:    a.model,
				Messages: prependOpenaiMessage(context, a.systemPrompt),
				Tools: utils.MapMany(
					append(a.toolHandler.Tools(), extraTools...),
					toOpenAIToolParam,
				),
			},
		)
		if len(completion.Choices) == 0 {
			return models.Checkpoint{}, ErrNoResponse
		}
		if err != nil {
			return models.Checkpoint{}, err
		}

		msg := completion.Choices[0].Message
		context = append(context, msg.ToParam())

		if len(msg.ToolCalls) == 0 {
			return models.Checkpoint{
				SessionID:    checkpoint.SessionID,
				Context:      utils.MapMany(context, fromOpenAIMessageParam),
				Prompt:       userMessage,
				Response:     msg.Content,
				InputTokens:  completion.Usage.PromptTokens,
				OutputTokens: completion.Usage.CompletionTokens,
			}, nil
		}

		toolResults, err := a.toolHandler.ExecuteCalls(
			ctx,
			utils.MapMany(msg.ToolCalls, fromOpenAIToolCall),
			extraTools...,
		)
		if err != nil {
			return models.Checkpoint{}, err
		}

		context = append(context, utils.MapMany(toolResults, toOpenAIMessageParam)...)
	}
	return models.Checkpoint{}, errors.New("failed to get final response after 10 retries")
}

func fromOpenAIToolCall(tc openai.ChatCompletionMessageToolCallUnion) models.ToolCall {
	return models.ToolCall{
		ID:        tc.ID,
		Name:      tc.Function.Name,
		Arguments: tc.Function.Arguments,
	}
}

func (a *openaiAgent) InvokeStream(
	ctx context.Context,
	checkpoint models.Checkpoint,
	userMessage string,
	onContent func(string),
	extraTools ...*tools.Tool,
) (models.Checkpoint, error) {
	context := utils.MapMany(append(checkpoint.Context, models.ContextMsg{
		Role:    models.MessageRoleUser,
		Content: userMessage,
	}), toOpenAIMessageParam)

	for range 10 {
		finalCheckpoint, err := func() (*models.Checkpoint, error) {
			stream := a.completionClient.NewStreaming(
				ctx,
				openai.ChatCompletionNewParams{
					Model:    a.model,
					Messages: prependOpenaiMessage(context, a.systemPrompt),
					Tools: utils.MapMany(
						append(a.toolHandler.Tools(), extraTools...),
						toOpenAIToolParam,
					),
				},
			)
			defer stream.Close()

			acc := openai.ChatCompletionAccumulator{}
			for stream.Next() {
				chunk := stream.Current()
				acc.AddChunk(chunk)

				if len(chunk.Choices) > 0 {
					onContent(chunk.Choices[0].Delta.Content)
				}
			}
			if err := stream.Err(); err != nil {
				return nil, err
			}

			if len(acc.Choices) == 0 {
				return nil, ErrNoResponse
			}

			msg := acc.Choices[0].Message
			if msg.Refusal != "" && msg.Content == "" && len(msg.ToolCalls) == 0 {
				return nil, fmt.Errorf("refusal: %s", msg.Refusal)
			}

			if msg.Content == "" && len(msg.ToolCalls) == 0 {
				return nil, ErrNoResponse
			}

			context = append(context, msg.ToParam())

			if len(msg.ToolCalls) == 0 {
				return &models.Checkpoint{
					SessionID:    checkpoint.SessionID,
					Context:      utils.MapMany(context, fromOpenAIMessageParam),
					Prompt:       userMessage,
					Response:     msg.Content,
					InputTokens:  acc.Usage.PromptTokens,
					OutputTokens: acc.Usage.CompletionTokens,
				}, nil
			}

			toolResults, err := a.toolHandler.ExecuteCalls(
				ctx,
				utils.MapMany(msg.ToolCalls, fromOpenAIToolCall),
				extraTools...,
			)
			if err != nil {
				return nil, err
			}

			context = append(context, utils.MapMany(toolResults, toOpenAIMessageParam)...)
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

func prependOpenaiMessage(messages []openai.ChatCompletionMessageParamUnion, message string) []openai.ChatCompletionMessageParamUnion {
	if message == "" {
		return messages
	}
	return append(
		[]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(message)},
		messages...,
	)
}

func toOpenAIToolParam(t *tools.Tool) openai.ChatCompletionToolUnionParam {
	def, err := t.GetDefinition()
	if err != nil {
		log.Printf("error getting tool definition: %v\n", err)
	}
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: toOpenAIFunctionDefinitionParam(def),
		},
	}
}

func fromOpenAIMessageParam(msg openai.ChatCompletionMessageParamUnion) models.ContextMsg {
	if !param.IsOmitted(msg.OfAssistant) {
		contextMsg := models.ContextMsg{
			Role: models.MessageRoleAssistant,
		}
		content := msg.OfAssistant.Content
		if !param.IsOmitted(content.OfString) {
			contextMsg.Content = msg.OfAssistant.Content.OfString.Value
		}
		if len(msg.OfAssistant.ToolCalls) > 0 {
			toolCalls := make([]models.ToolCall, 0, len(msg.OfAssistant.ToolCalls))
			for _, tc := range msg.OfAssistant.ToolCalls {
				if !param.IsOmitted(tc.OfFunction) {
					toolCalls = append(toolCalls, models.ToolCall{
						ID:        tc.OfFunction.ID,
						Name:      tc.OfFunction.Function.Name,
						Arguments: tc.OfFunction.Function.Arguments,
					})
				}
			}
			contextMsg.ToolCalls = toolCalls
		}
		return contextMsg
	}
	if !param.IsOmitted(msg.OfUser) {
		return models.ContextMsg{
			Role:    models.MessageRoleUser,
			Content: msg.OfUser.Content.OfString.Value,
		}
	}
	if !param.IsOmitted(msg.OfSystem) {
		return models.ContextMsg{
			Role:    models.MessageRoleSystem,
			Content: msg.OfSystem.Content.OfString.Value,
		}
	}
	if !param.IsOmitted(msg.OfTool) {
		return models.ContextMsg{
			Role:       models.MessageRoleTool,
			Content:    msg.OfTool.Content.OfString.Value,
			ToolCallID: msg.OfTool.ToolCallID,
		}
	}
	return models.ContextMsg{}
}

func toOpenAIMessageParam(msg models.ContextMsg) openai.ChatCompletionMessageParamUnion {
	switch msg.Role {
	case models.MessageRoleTool:
		return openai.ToolMessage(msg.Content, msg.ToolCallID)
	case models.MessageRoleUser:
		return openai.UserMessage(msg.Content)
	case models.MessageRoleAssistant:
		assistantMessage := openai.AssistantMessage(msg.Content)
		if len(msg.ToolCalls) > 0 {
			assistantMessage.OfAssistant.ToolCalls = make([]openai.ChatCompletionMessageToolCallUnionParam, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				assistantMessage.OfAssistant.ToolCalls[i] = toOpenAIToolCall(tc)
			}
		}
		return assistantMessage
	case models.MessageRoleSystem:
		return openai.SystemMessage(msg.Content)
	default:
		return openai.ChatCompletionMessageParamUnion{}
	}
}

func toOpenAIToolCall(tc models.ToolCall) openai.ChatCompletionMessageToolCallUnionParam {
	return openai.ChatCompletionMessageToolCallUnionParam{
		OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
			ID: tc.ID,
			Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
				Arguments: tc.Arguments,
				Name:      tc.Name,
			},
		},
	}
}

func toOpenAIFunctionDefinitionParam(def tools.FunctionDefinitionParam) openai.FunctionDefinitionParam {
	return openai.FunctionDefinitionParam{
		Name:        def.Name,
		Description: openai.String(def.Description),
		Parameters:  def.Parameters,
	}
}

type openaiSummarizer struct {
	chatClient                *openai.ChatCompletionService
	model                     openai.ChatModel
	tokensBeforeSummarization int64
	maxSummaryLength          int64
	summaryPrompt             string
	messagesToKeep            int
}

type SummarizerConfig struct {
	ChatClient                *openai.ChatCompletionService
	Model                     openai.ChatModel
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
		model:                     config.Model,
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

	completion, err := s.chatClient.New(
		ctx, openai.ChatCompletionNewParams{
			Model:    s.model,
			Messages: utils.MapMany(concatenatedMessages, toOpenAIMessageParam),
		},
	)
	if err != nil {
		return models.Checkpoint{}, err
	}

	if len(completion.Choices) == 0 {
		return models.Checkpoint{}, errors.New("no response from summarizer")
	}

	msg := completion.Choices[0].Message
	newSummary := models.ContextMsg{
		Role:    models.MessageRoleAssistant,
		Content: msg.Content,
	}

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

func (s *openaiSummarizer) shouldSummarize(checkpoint models.Checkpoint) bool {
	return checkpoint.TotalTokens() > s.tokensBeforeSummarization
}

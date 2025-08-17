package client

import (
	"context"
	"log"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/ssestream"
	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
	"github.com/raphael-foliveira/tca/pkg/utils"
)

type openaiChatCompletionClient struct {
	openai.ChatCompletionService
}

func NewOpenAIChatCompletionClient(client openai.Client) ChatCompletionClient {
	return &openaiChatCompletionClient{ChatCompletionService: client.Chat.Completions}
}

func (c *openaiChatCompletionClient) New(
	ctx context.Context,
	params ChatCompletionParams,
) (*ChatCompletion, error) {
	completion, err := c.ChatCompletionService.New(
		ctx,
		toOpenAIChatCompletionParams(params),
	)
	if err != nil {
		return nil, err
	}
	return fromOpenAICompletion(completion), nil
}

func (c *openaiChatCompletionClient) NewStreaming(
	ctx context.Context,
	params ChatCompletionParams,
) ChatCompletionStream {
	stream := c.ChatCompletionService.NewStreaming(
		ctx,
		toOpenAIChatCompletionParams(params),
	)
	return &openaiChatCompletionStream{Stream: stream}
}

type openaiChatCompletionStream struct {
	*ssestream.Stream[openai.ChatCompletionChunk]
}

func (s *openaiChatCompletionStream) Current() ChatCompletionChunk {
	return fromOpenAIChunk(s.Stream.Current())
}

func fromOpenAIChunk(chunk openai.ChatCompletionChunk) ChatCompletionChunk {
	if len(chunk.Choices) == 0 {
		return ChatCompletionChunk{}
	}
	openaiChoice := chunk.Choices[0]

	toolCalls := make([]ChatCompletionChunkDeltaToolCall, len(openaiChoice.Delta.ToolCalls))

	for j, tc := range openaiChoice.Delta.ToolCalls {
		toolCalls[j] = ChatCompletionChunkDeltaToolCall{
			Index: tc.Index,
			ID:    tc.ID,
			Type:  tc.Type,
			Function: ChatCompletionChunkDeltaToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	choice := ChatCompletionChunkChoice{
		Delta: ChatCompletionChunkDelta{
			Role:      models.MessageRole(openaiChoice.Delta.Role),
			Content:   openaiChoice.Delta.Content,
			Refusal:   openaiChoice.Delta.Refusal,
			ToolCalls: toolCalls,
		},
	}

	usage := CompletionUsage{
		CompletionTokens: chunk.Usage.CompletionTokens,
		PromptTokens:     chunk.Usage.PromptTokens,
		TotalTokens:      chunk.Usage.TotalTokens,
	}

	return ChatCompletionChunk{
		ID:      chunk.ID,
		Choice:  choice,
		Created: chunk.Created,
		Model:   chunk.Model,
		Usage:   usage,
	}
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

func toOpenAIChatCompletionParams(params ChatCompletionParams) openai.ChatCompletionNewParams {
	return openai.ChatCompletionNewParams{
		Messages: utils.MapMany(params.Messages, toOpenAIMessageParam),
		Model:    params.Model,
		Tools:    utils.MapMany(params.Tools, toOpenAIToolParam),
	}
}

func fromOpenAICompletion(c *openai.ChatCompletion) *ChatCompletion {
	var message openai.ChatCompletionMessage
	if len(c.Choices) > 0 {
		message = c.Choices[0].Message
	}
	return &ChatCompletion{
		Message: fromOpenAIMessage(message),
		Usage: CompletionUsage{
			CompletionTokens: c.Usage.CompletionTokens,
			PromptTokens:     c.Usage.PromptTokens,
			TotalTokens:      c.Usage.TotalTokens,
		},
	}
}

func fromOpenAIMessage(msg openai.ChatCompletionMessage) models.ContextMsg {
	contextMsg := models.ContextMsg{
		Role:    models.MessageRoleAssistant,
		Content: msg.Content,
	}
	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]models.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, models.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
		contextMsg.ToolCalls = toolCalls
	}
	return contextMsg
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

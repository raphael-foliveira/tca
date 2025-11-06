package agent

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
	"github.com/raphael-foliveira/tca/pkg/utils"
)

type openaiChatCompletionClient struct {
	client *openai.ChatCompletionService
	model  openai.ChatModel
}

func NewOpenAIChatCompletionClient(client *openai.ChatCompletionService, model openai.ChatModel) ChatCompletionClient {
	return &openaiChatCompletionClient{
		client: client,
		model:  model,
	}
}

func (c *openaiChatCompletionClient) Complete(
	ctx context.Context,
	req ChatCompletionRequest,
) (ChatCompletionResponse, error) {
	openaiMessages := utils.MapMany(req.Messages, toOpenAIMessageParam)
	openaiTools := utils.MapMany(req.Tools, toOpenAIToolParam)

	completion, err := c.client.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model:    c.model,
			Messages: openaiMessages,
			Tools:    openaiTools,
		},
	)
	if err != nil {
		return ChatCompletionResponse{}, err
	}

	if len(completion.Choices) == 0 {
		return ChatCompletionResponse{}, ErrNoResponse
	}

	msg := completion.Choices[0].Message
	contextMsg := fromOpenAIMessageParam(msg.ToParam())

	return ChatCompletionResponse{
		Message:      contextMsg,
		InputTokens:  completion.Usage.PromptTokens,
		OutputTokens: completion.Usage.CompletionTokens,
	}, nil
}

func (c *openaiChatCompletionClient) CompleteStreaming(
	ctx context.Context,
	req ChatCompletionRequest,
	onDelta func(content string) error,
) (ChatCompletionResponse, error) {
	openaiMessages := utils.MapMany(req.Messages, toOpenAIMessageParam)
	openaiTools := utils.MapMany(req.Tools, toOpenAIToolParam)

	stream := c.client.NewStreaming(
		ctx,
		openai.ChatCompletionNewParams{
			Model:    c.model,
			Messages: openaiMessages,
			Tools:    openaiTools,
		},
	)
	defer stream.Close()

	acc := openai.ChatCompletionAccumulator{}
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) > 0 {
			if !param.IsOmitted(chunk.Choices[0].Delta.Content) {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					if err := onDelta(content); err != nil {
						return ChatCompletionResponse{}, fmt.Errorf("failed to handle delta: %w", err)
					}
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return ChatCompletionResponse{}, err
	}

	if len(acc.Choices) == 0 {
		return ChatCompletionResponse{}, ErrNoResponse
	}

	msg := acc.Choices[0].Message
	if msg.Refusal != "" && msg.Content == "" && len(msg.ToolCalls) == 0 {
		return ChatCompletionResponse{}, fmt.Errorf("refusal: %s", msg.Refusal)
	}

	if msg.Content == "" && len(msg.ToolCalls) == 0 {
		return ChatCompletionResponse{}, ErrNoResponse
	}

	contextMsg := fromOpenAIMessageParam(msg.ToParam())
	return ChatCompletionResponse{
		Message:      contextMsg,
		InputTokens:  acc.Usage.PromptTokens,
		OutputTokens: acc.Usage.CompletionTokens,
	}, nil
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

func fromOpenAIToolCall(tc openai.ChatCompletionMessageToolCallUnion) models.ToolCall {
	return models.ToolCall{
		ID:        tc.ID,
		Name:      tc.Function.Name,
		Arguments: tc.Function.Arguments,
	}
}

func toOpenAIToolParam(t *tools.Tool) openai.ChatCompletionToolUnionParam {
	def, err := t.GetDefinition()
	if err != nil {
	}
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: toOpenAIFunctionDefinitionParam(def),
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

package tools

import (
	"context"
	"fmt"

	"github.com/raphael-foliveira/tca/pkg/models"
)

type ToolHandler struct {
	tools []*Tool
}

func NewToolHandler(tools []*Tool) *ToolHandler {
	return &ToolHandler{tools: tools}
}

func (t *ToolHandler) ExecuteCalls(
	ctx context.Context,
	toolCalls []models.ToolCall,
	extraTools ...*Tool,
) ([]models.ContextMsg, error) {
	results := []models.ContextMsg{}

	for _, toolCall := range toolCalls {
		tool := t.findToolByName(toolCall.Name, extraTools...)
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

func (t *ToolHandler) Tools() []*Tool {
	return t.tools
}

func (t *ToolHandler) findToolByName(name string, extraTools ...*Tool) *Tool {
	for _, tool := range append(t.tools, extraTools...) {
		if tool.GetName() == name {
			return tool
		}
	}
	return nil
}

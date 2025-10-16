package tools_test

import (
	"context"
	"errors"
	"testing"

	"github.com/raphael-foliveira/tca/pkg/models"
	"github.com/raphael-foliveira/tca/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolHandlerExecuteCallsSuccessfully(t *testing.T) {
	type searchParams struct {
		Query string `json:"query"`
	}

	var receivedParams searchParams
	searchTool := tools.NewTool("search", "searches for things", func(_ context.Context, params searchParams) (string, error) {
		receivedParams = params
		return "search results", nil
	})

	handler := tools.NewToolHandler([]*tools.Tool{searchTool})

	toolCalls := []models.ToolCall{
		{
			ID:        "call_1",
			Name:      "search",
			Arguments: `{"query":"test query"}`,
		},
	}

	results, err := handler.ExecuteCalls(context.Background(), toolCalls)
	require.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, models.MessageRoleTool, results[0].Role)
	assert.Equal(t, "search results", results[0].Content)
	assert.Equal(t, "call_1", results[0].ToolCallID)
	assert.Equal(t, "test query", receivedParams.Query)
}

func TestToolHandlerExecuteCallsWithMultipleTools(t *testing.T) {
	tool1 := tools.NewTool("tool1", "first tool", func(_ context.Context, _ struct{}) (string, error) {
		return "result1", nil
	})

	tool2 := tools.NewTool("tool2", "second tool", func(_ context.Context, _ struct{}) (string, error) {
		return "result2", nil
	})

	handler := tools.NewToolHandler([]*tools.Tool{tool1, tool2})

	toolCalls := []models.ToolCall{
		{ID: "call_1", Name: "tool1", Arguments: "{}"},
		{ID: "call_2", Name: "tool2", Arguments: "{}"},
	}

	results, err := handler.ExecuteCalls(context.Background(), toolCalls)
	require.NoError(t, err)

	assert.Len(t, results, 2)
	assert.Equal(t, "result1", results[0].Content)
	assert.Equal(t, "call_1", results[0].ToolCallID)
	assert.Equal(t, "result2", results[1].Content)
	assert.Equal(t, "call_2", results[1].ToolCallID)
}

func TestToolHandlerExecuteCallsWithExtraTools(t *testing.T) {
	baseTool := tools.NewTool("base", "base tool", func(_ context.Context, _ struct{}) (string, error) {
		return "base result", nil
	})

	extraTool := tools.NewTool("extra", "extra tool", func(_ context.Context, _ struct{}) (string, error) {
		return "extra result", nil
	})

	handler := tools.NewToolHandler([]*tools.Tool{baseTool})

	toolCalls := []models.ToolCall{
		{ID: "call_1", Name: "base", Arguments: "{}"},
		{ID: "call_2", Name: "extra", Arguments: "{}"},
	}

	results, err := handler.ExecuteCalls(context.Background(), toolCalls, extraTool)
	require.NoError(t, err)

	assert.Len(t, results, 2)
	assert.Equal(t, "base result", results[0].Content)
	assert.Equal(t, "extra result", results[1].Content)
}

func TestToolHandlerExecuteCallsReturnsErrorWhenToolNotFound(t *testing.T) {
	tool := tools.NewTool("exists", "existing tool", func(_ context.Context, _ struct{}) (string, error) {
		return "result", nil
	})

	handler := tools.NewToolHandler([]*tools.Tool{tool})

	toolCalls := []models.ToolCall{
		{ID: "call_1", Name: "nonexistent", Arguments: "{}"},
	}

	_, err := handler.ExecuteCalls(context.Background(), toolCalls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found: nonexistent")
}

func TestToolHandlerExecuteCallsReturnsErrorWhenToolFails(t *testing.T) {
	expectedErr := errors.New("tool execution failed")
	failingTool := tools.NewTool("failing", "failing tool", func(_ context.Context, _ struct{}) (string, error) {
		return "", expectedErr
	})

	handler := tools.NewToolHandler([]*tools.Tool{failingTool})

	toolCalls := []models.ToolCall{
		{ID: "call_1", Name: "failing", Arguments: "{}"},
	}

	_, err := handler.ExecuteCalls(context.Background(), toolCalls)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestToolHandlerToolsReturnsAllTools(t *testing.T) {
	tool1 := tools.NewTool("tool1", "first tool", func(_ context.Context, _ struct{}) (string, error) {
		return "result1", nil
	})

	tool2 := tools.NewTool("tool2", "second tool", func(_ context.Context, _ struct{}) (string, error) {
		return "result2", nil
	})

	handler := tools.NewToolHandler([]*tools.Tool{tool1, tool2})

	toolsList := handler.Tools()
	assert.Len(t, toolsList, 2)
	assert.Equal(t, "tool1", toolsList[0].GetName())
	assert.Equal(t, "tool2", toolsList[1].GetName())
}

func TestToolHandlerWithEmptyToolsList(t *testing.T) {
	handler := tools.NewToolHandler([]*tools.Tool{})

	toolCalls := []models.ToolCall{
		{ID: "call_1", Name: "anything", Arguments: "{}"},
	}

	_, err := handler.ExecuteCalls(context.Background(), toolCalls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
}

package models

import (
	"time"
)

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

type Session struct {
	ID        *string   `json:"id,omitempty" db:"id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ContextMsg struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type Checkpoint struct {
	ID           *string      `json:"id,omitempty" db:"id"`
	SessionID    string       `json:"session_id" db:"session_id"`
	Context      []ContextMsg `json:"context" db:"context"`
	Prompt       string       `json:"prompt" db:"prompt"`
	Response     string       `json:"response" db:"response"`
	InputTokens  int64        `json:"input_tokens,omitempty" db:"input_tokens"`
	OutputTokens int64        `json:"output_tokens,omitempty" db:"output_tokens"`
	IsSummary    bool         `json:"is_summary" db:"is_summary"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
}

func (c *Checkpoint) TotalTokens() int64 {
	return c.InputTokens + c.OutputTokens
}

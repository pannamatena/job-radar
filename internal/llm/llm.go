// Package llm is job-radar's small, provider-neutral view of a chat model with
// tools. The scoring agent (internal/agent) only talks to the Client interface
// below, never to a provider SDK, so:
//
//   - tests drive the agent with a scripted fake client (no network, no cost);
//   - another provider (or a free local model) can be added by implementing
//     one method, without touching the agent loop.
//
// Only Anthropic is implemented (anthropic.go).
//
// JS analogy: Client is like a TypeScript interface for a fetch wrapper. The
// agent code depends on the interface; the real implementation and the test
// mock both satisfy it.
package llm

import (
	"context"
	"encoding/json"
)

// Client sends one request (the whole conversation so far) and returns the
// model's next message. It does not loop; the agent owns the loop.
type Client interface {
	Send(ctx context.Context, req Request) (Response, error)
}

// Request is one call to the model.
type Request struct {
	Model     string
	MaxTokens int // output cap for this call
	// System is the system prompt. When CacheSystem is true it's marked for
	// prompt caching, so repeated calls with the same tools+system pay ~10%
	// for that part (ADR-0008).
	System      string
	CacheSystem bool
	Tools       []Tool
	Messages    []Message
}

// Tool describes one tool the model may call.
type Tool struct {
	Name        string
	Description string
	// Schema is the JSON schema of the tool's input (an "object" schema).
	Schema map[string]any
	// Strict asks the provider to guarantee the input matches Schema.
	Strict bool
}

// Role of a message in the conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn in the conversation.
type Message struct {
	Role   Role
	Blocks []Block
}

// BlockType says which fields of a Block are used.
type BlockType string

const (
	BlockText       BlockType = "text"        // Text
	BlockToolUse    BlockType = "tool_use"    // ToolUseID, ToolName, Input
	BlockToolResult BlockType = "tool_result" // ToolUseID, Text, IsError
)

// Block is one piece of a message: some text, a tool call from the model, or
// our answer to a tool call. (A tagged union, like a TS discriminated union
// on `type`.)
type Block struct {
	Type      BlockType
	Text      string
	ToolUseID string
	ToolName  string
	Input     json.RawMessage
	IsError   bool
}

// StopReason says why the model stopped.
type StopReason string

const (
	StopEndTurn   StopReason = "end_turn"
	StopToolUse   StopReason = "tool_use"
	StopMaxTokens StopReason = "max_tokens"
)

// Response is the model's reply to one Request.
type Response struct {
	Blocks     []Block
	StopReason StopReason
	Usage      Usage
}

// Usage is the token accounting for one call (or a sum of calls).
type Usage struct {
	InputTokens      int64 // uncached input
	OutputTokens     int64
	CacheWriteTokens int64 // input written to the prompt cache
	CacheReadTokens  int64 // input read from the prompt cache
}

// Add returns the sum of two usages.
func (u Usage) Add(o Usage) Usage {
	return Usage{
		InputTokens:      u.InputTokens + o.InputTokens,
		OutputTokens:     u.OutputTokens + o.OutputTokens,
		CacheWriteTokens: u.CacheWriteTokens + o.CacheWriteTokens,
		CacheReadTokens:  u.CacheReadTokens + o.CacheReadTokens,
	}
}

// Total is every token billed, used for the per-posting token budget.
func (u Usage) Total() int64 {
	return u.InputTokens + u.OutputTokens + u.CacheWriteTokens + u.CacheReadTokens
}

// Price is a model's prices in USD per million tokens (from config, never
// hard-coded — prices change).
type Price struct {
	Input, Output, CacheWrite, CacheRead float64
}

// Cost estimates the USD cost of a usage at these prices.
func (p Price) Cost(u Usage) float64 {
	const perMillion = 1_000_000.0
	return (float64(u.InputTokens)*p.Input +
		float64(u.OutputTokens)*p.Output +
		float64(u.CacheWriteTokens)*p.CacheWrite +
		float64(u.CacheReadTokens)*p.CacheRead) / perMillion
}

package driven

import (
	"context"
	"errors"
	"time"
)

// AIAssistant is a driven port over a multi-turn, tool-using LLM. It is
// stateless across calls: the full conversation is passed in on every
// Chat and the adapter does NOT retain session state. The adapter is
// responsible for translating the port's logical messages into the
// provider's wire format (tool_use blocks for Anthropic, function calls
// for OpenAI, etc.) and for applying prompt caching to the System and
// Context fields.
type AIAssistant interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// Tool names used throughout the dialog state machine. Every system
// prompt instructs the model to call exactly one of these per turn.
const (
	ToolAskQuestion    = "ask_question"
	ToolSubmitProposal = "submit_proposal"
)

// ChatRequest is one turn of a dialog sent to the provider. System
// and Context are intended to be cached by the adapter; Messages are
// the growing, per-turn conversation tail and are NOT cached.
type ChatRequest struct {
	Model     string    // e.g. "claude-haiku-4-5-20251001"
	System    string    // system prompt — cacheable
	Context   string    // ancestor tree + links block — cacheable
	Messages  []Message // rolling dialog — not cached
	Tools     []Tool    // available tools (ask_question, submit_proposal)
	MaxTokens int
}

// Message is one turn in the rolling dialog. A turn is either plain
// text or a structured tool_use / tool_result block; mixing is not
// supported in v1 because no command needs it.
type Message struct {
	Role       MessageRole
	Text       string             // set when this is a plain-text turn
	ToolUse    *ToolUseContent    // set on assistant turns that called a tool
	ToolResult *ToolResultContent // set on user turns that answered a tool call
}

// MessageRole identifies which side of the conversation a message
// belongs to. System prompts are carried on ChatRequest.System and
// are not modeled as a Message.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// ToolUseContent is an assistant's request to invoke a tool. ID is
// assigned by the adapter and echoed back on the matching tool_result.
type ToolUseContent struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResultContent is the user-side response to a prior assistant
// tool_use turn.
type ToolResultContent struct {
	ToolUseID string
	Content   string
}

// Tool is the contract the model is told it may invoke. InputSchema is
// a JSON Schema object describing the tool's arguments.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ChatResponse is the parsed result of one Chat call. Exactly one of
// ToolUse or RawText is populated in the happy path; if neither is,
// the adapter returns ErrAIMalformedResponse instead.
type ChatResponse struct {
	ToolUse    *ToolUseContent
	RawText    string
	Usage      Usage
	StopReason string
}

// Usage tracks token consumption for a single call. Cache fields are
// zero when the adapter does not support caching.
type Usage struct {
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}

// AI sentinel errors. The dialog state machine switches on these to
// decide between backoff, fail-fast, and malformed-response retry.
//
// Retry policy at a glance:
//
//   ErrAIAuthFailed, ErrAIContextTooLarge, ErrAIInvalidRequest — fail-fast
//   ErrAIRateLimited, ErrAIOverloaded, ErrAINetworkError       — retry with backoff
//   ErrAIMalformedResponse                                     — retry on malformed budget
var (
	ErrAIAuthFailed        = errors.New("ai: authentication failed")
	ErrAIRateLimited       = errors.New("ai: rate limited")
	ErrAIOverloaded        = errors.New("ai: provider overloaded")
	ErrAINetworkError      = errors.New("ai: network error")
	ErrAIContextTooLarge   = errors.New("ai: context too large")
	ErrAIInvalidRequest    = errors.New("ai: invalid request")
	ErrAIMalformedResponse = errors.New("ai: response did not match expected shape")
)

// RateLimitedError wraps ErrAIRateLimited with a Retry-After hint from
// the provider. The state machine's backoff scheduler uses the hint
// when present and falls back to exponential backoff otherwise.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string { return "ai: rate limited" }

func (e *RateLimitedError) Unwrap() error { return ErrAIRateLimited }

package anthropic_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	sdkoption "github.com/anthropics/anthropic-sdk-go/option"

	anthropicadapter "github.com/c64-io/daedalus/internal/adapter/driven/anthropic"
	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// newClientAgainst builds a Client pointed at the given test server.
// It sets ANTHROPIC_API_KEY through the env so NewClient succeeds,
// then uses the SDK's WithBaseURL to redirect to the stub.
func newClientAgainst(t *testing.T, ts *httptest.Server) *anthropicadapter.Client {
	t.Helper()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	c, err := anthropicadapter.NewClient(
		sdkoption.WithBaseURL(ts.URL+"/"),
		sdkoption.WithMaxRetries(0),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestNewClient_MissingAPIKeyFailsFast(t *testing.T) {
	// Temporarily unset ANTHROPIC_API_KEY for the duration of this
	// test without disturbing other tests. t.Setenv resets on cleanup.
	orig, hadOrig := os.LookupEnv("ANTHROPIC_API_KEY")
	if err := os.Unsetenv("ANTHROPIC_API_KEY"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	t.Cleanup(func() {
		if hadOrig {
			os.Setenv("ANTHROPIC_API_KEY", orig)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	})

	_, err := anthropicadapter.NewClient()
	if !errors.Is(err, driven.ErrAIAuthFailed) {
		t.Fatalf("expected ErrAIAuthFailed, got %v", err)
	}
}

// ---------------------------------------------------------------------
// Request translation
// ---------------------------------------------------------------------

func TestChat_SendsCachedSystemAndContext(t *testing.T) {
	var captured map[string]any

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &captured)

		// Respond with a minimal valid message that contains one tool_use.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "msg_1",
			"type": "message",
			"role": "assistant",
			"model": "claude-haiku-4-5-20251001",
			"content": [{
				"type": "tool_use",
				"id": "toolu_1",
				"name": "ask_question",
				"input": {"question": "who is this for?"}
			}],
			"stop_reason": "tool_use",
			"stop_sequence": null,
			"usage": {
				"input_tokens": 10,
				"output_tokens": 5,
				"cache_creation_input_tokens": 100,
				"cache_read_input_tokens": 50
			}
		}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)
	resp, err := c.Chat(context.Background(), driven.ChatRequest{
		Model:     "claude-haiku-4-5-20251001",
		System:    "you are a test",
		Context:   "test context block",
		MaxTokens: 1024,
		Tools: []driven.Tool{
			{Name: "ask_question", Description: "ask", InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"question": map[string]any{"type": "string"}},
				"required":   []string{"question"},
			}},
		},
		Messages: []driven.Message{
			{Role: driven.RoleUser, Text: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ToolUse == nil || resp.ToolUse.Name != "ask_question" {
		t.Fatalf("expected ask_question tool use; got %+v", resp.ToolUse)
	}
	if resp.ToolUse.Input["question"] != "who is this for?" {
		t.Fatalf("tool input: %+v", resp.ToolUse.Input)
	}
	if resp.Usage.CacheCreationTokens != 100 || resp.Usage.CacheReadTokens != 50 {
		t.Fatalf("usage cache fields not parsed: %+v", resp.Usage)
	}

	// Confirm cache_control is on each system block.
	systemArr, ok := captured["system"].([]any)
	if !ok || len(systemArr) != 2 {
		t.Fatalf("expected 2 system blocks with cache_control, got %v", captured["system"])
	}
	for i, raw := range systemArr {
		blk, _ := raw.(map[string]any)
		cc, ok := blk["cache_control"].(map[string]any)
		if !ok {
			t.Fatalf("system[%d] missing cache_control; got %v", i, blk)
		}
		if cc["type"] != "ephemeral" {
			t.Fatalf("system[%d] cache_control.type = %v, want ephemeral", i, cc["type"])
		}
	}

	// And that a tool was forwarded.
	tools, _ := captured["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
}

func TestChat_TranslatesToolUseAndToolResult(t *testing.T) {
	var captured map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &captured)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "msg_2", "type": "message", "role": "assistant",
			"model": "x", "stop_reason": "end_turn", "stop_sequence": null,
			"content": [{"type": "text", "text": "ok"}],
			"usage": {"input_tokens": 1, "output_tokens": 1,
			          "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}
		}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)

	// Send a rolling dialog with (assistant tool_use) + (user tool_result).
	_, err := c.Chat(context.Background(), driven.ChatRequest{
		Model:     "x",
		MaxTokens: 16,
		Messages: []driven.Message{
			{Role: driven.RoleAssistant, ToolUse: &driven.ToolUseContent{
				ID: "toolu_1", Name: "ask_question",
				Input: map[string]any{"question": "who?"},
			}},
			{Role: driven.RoleUser, ToolResult: &driven.ToolResultContent{
				ToolUseID: "toolu_1", Content: "end users",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	msgs, _ := captured["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages on wire, got %d", len(msgs))
	}

	assistant, _ := msgs[0].(map[string]any)
	if assistant["role"] != "assistant" {
		t.Fatalf("msg[0] role = %v, want assistant", assistant["role"])
	}
	content, _ := assistant["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("assistant content len = %d, want 1", len(content))
	}
	blk, _ := content[0].(map[string]any)
	if blk["type"] != "tool_use" || blk["name"] != "ask_question" {
		t.Fatalf("assistant tool_use block wrong: %v", blk)
	}

	user, _ := msgs[1].(map[string]any)
	userContent, _ := user["content"].([]any)
	userBlk, _ := userContent[0].(map[string]any)
	if userBlk["type"] != "tool_result" || userBlk["tool_use_id"] != "toolu_1" {
		t.Fatalf("user tool_result wrong: %v", userBlk)
	}
}

// ---------------------------------------------------------------------
// Error classification
// ---------------------------------------------------------------------

func TestChat_401MapsToAuthFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)
	_, err := c.Chat(context.Background(), driven.ChatRequest{Model: "x", MaxTokens: 1})
	if !errors.Is(err, driven.ErrAIAuthFailed) {
		t.Fatalf("expected ErrAIAuthFailed, got %v", err)
	}
}

func TestChat_429MapsToRateLimitedWithRetryAfter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)
	_, err := c.Chat(context.Background(), driven.ChatRequest{Model: "x", MaxTokens: 1})
	if !errors.Is(err, driven.ErrAIRateLimited) {
		t.Fatalf("expected ErrAIRateLimited, got %v", err)
	}

	var rl *driven.RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitedError concrete type")
	}
	if rl.RetryAfter != 7*time.Second {
		t.Fatalf("RetryAfter: got %v, want 7s", rl.RetryAfter)
	}
}

func TestChat_503MapsToOverloaded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)
	_, err := c.Chat(context.Background(), driven.ChatRequest{Model: "x", MaxTokens: 1})
	if !errors.Is(err, driven.ErrAIOverloaded) {
		t.Fatalf("expected ErrAIOverloaded, got %v", err)
	}
}

func TestChat_400PromptTooLongMapsToContextTooLarge(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long"}}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)
	_, err := c.Chat(context.Background(), driven.ChatRequest{Model: "x", MaxTokens: 1})
	if !errors.Is(err, driven.ErrAIContextTooLarge) {
		t.Fatalf("expected ErrAIContextTooLarge, got %v", err)
	}
}

func TestChat_413MapsToContextTooLarge(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"request_too_large","message":"too big"}}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)
	_, err := c.Chat(context.Background(), driven.ChatRequest{Model: "x", MaxTokens: 1})
	if !errors.Is(err, driven.ErrAIContextTooLarge) {
		t.Fatalf("expected ErrAIContextTooLarge, got %v", err)
	}
}

func TestChat_UnknownAPIErrorMapsToNetwork(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired) // uncommon but valid HTTP
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"unknown","message":"weird"}}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)
	_, err := c.Chat(context.Background(), driven.ChatRequest{Model: "x", MaxTokens: 1})
	if !errors.Is(err, driven.ErrAINetworkError) {
		t.Fatalf("expected ErrAINetworkError, got %v", err)
	}
}

// ---------------------------------------------------------------------
// Response translation: text-only responses
// ---------------------------------------------------------------------

func TestChat_TextOnlyResponseFillsRawText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "msg_x", "type": "message", "role": "assistant",
			"model": "x", "stop_reason": "end_turn", "stop_sequence": null,
			"content": [{"type": "text", "text": "first"}, {"type": "text", "text": "second"}],
			"usage": {"input_tokens": 1, "output_tokens": 2,
			          "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}
		}`))
	}))
	defer ts.Close()

	c := newClientAgainst(t, ts)
	resp, err := c.Chat(context.Background(), driven.ChatRequest{Model: "x", MaxTokens: 1})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ToolUse != nil {
		t.Fatalf("expected no ToolUse on text-only response")
	}
	if !strings.Contains(resp.RawText, "first") || !strings.Contains(resp.RawText, "second") {
		t.Fatalf("RawText = %q", resp.RawText)
	}
}

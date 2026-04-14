// Package anthropic is the driven adapter that satisfies the
// driven.AIAssistant port by calling the Anthropic Messages API via
// the official Go SDK.
//
// The core knows nothing about this package; the composition root in
// cmd/d7/main.go is the only place it appears. Translating the port's
// logical message model into the provider's wire format — including
// prompt caching (cache_control: ephemeral on system + context) and
// tool_use / tool_result content blocks — is entirely this file's
// responsibility.
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	sdkoption "github.com/anthropics/anthropic-sdk-go/option"

	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// Client is the concrete AIAssistant implementation. It is a thin
// wrapper around the Anthropic SDK's MessageService with our port's
// translation code and error classifier bolted on.
//
// The SDK client is built lazily on the first Chat() call. This lets
// `d7` run data-only commands (idea new, epic list, …) with
// ANTHROPIC_API_KEY unset — the missing-key error only surfaces when
// an AI command actually tries to talk to the model.
type Client struct {
	opts    []sdkoption.RequestOption
	once    sync.Once
	sdk     sdk.Client
	initErr error
}

// Compile-time assertion that Client satisfies the port.
var _ driven.AIAssistant = (*Client)(nil)

// New builds a Client. It does NOT read ANTHROPIC_API_KEY yet — the
// env var is consulted on the first Chat() call, which is what lets
// data-only d7 commands run without it.
//
// Callers may pass additional SDK options for tests (such as
// option.WithBaseURL for a stub server).
func New(opts ...sdkoption.RequestOption) *Client {
	return &Client{opts: opts}
}

// Chat translates a ChatRequest into a MessageNewParams, calls the
// provider, and translates the first tool_use (or text) block back
// into a ChatResponse. Errors are mapped to our sentinel set.
//
// The SDK client is built on the first call under a sync.Once so a
// missing ANTHROPIC_API_KEY surfaces as driven.ErrAIAuthFailed here
// rather than at construction time.
func (c *Client) Chat(ctx context.Context, req driven.ChatRequest) (*driven.ChatResponse, error) {
	c.once.Do(func() {
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			c.initErr = fmt.Errorf("%w: ANTHROPIC_API_KEY is not set", driven.ErrAIAuthFailed)
			return
		}
		c.sdk = sdk.NewClient(c.opts...)
	})
	if c.initErr != nil {
		return nil, c.initErr
	}

	params := c.buildParams(req)

	msg, err := c.sdk.Messages.New(ctx, params)
	if err != nil {
		return nil, classifyError(err)
	}

	return toChatResponse(msg), nil
}

// buildParams is the request-direction translator. Two things matter
// here that are easy to get wrong:
//
//  1. The System and Context strings are sent as two separate
//     TextBlockParams in the top-level System array, each flagged
//     with cache_control: ephemeral. Splitting them means the long
//     prompt stays cached even when the context block is re-rendered
//     for a different target.
//  2. Tool results come in as user-role messages whose content is a
//     ToolResultBlockParam — not a plain text block.
func (c *Client) buildParams(req driven.ChatRequest) sdk.MessageNewParams {
	params := sdk.MessageNewParams{
		Model:     sdk.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
	}

	// System + Context both cached.
	var sys []sdk.TextBlockParam
	if strings.TrimSpace(req.System) != "" {
		sys = append(sys, sdk.TextBlockParam{
			Text:         req.System,
			CacheControl: sdk.CacheControlEphemeralParam{TTL: sdk.CacheControlEphemeralTTLTTL5m},
		})
	}
	if strings.TrimSpace(req.Context) != "" {
		sys = append(sys, sdk.TextBlockParam{
			Text:         req.Context,
			CacheControl: sdk.CacheControlEphemeralParam{TTL: sdk.CacheControlEphemeralTTLTTL5m},
		})
	}
	params.System = sys

	// Rolling dialog messages.
	params.Messages = make([]sdk.MessageParam, 0, len(req.Messages))
	for _, m := range req.Messages {
		params.Messages = append(params.Messages, toSDKMessage(m))
	}

	// Tools.
	if len(req.Tools) > 0 {
		tools := make([]sdk.ToolUnionParam, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, sdk.ToolUnionParam{OfTool: &sdk.ToolParam{
				Name: t.Name,
				Description: sdk.String(t.Description),
				InputSchema: toInputSchema(t.InputSchema),
			}})
		}
		params.Tools = tools
	}

	return params
}

// toSDKMessage translates our port's Message into the SDK's
// MessageParam. Exactly one of Text / ToolUse / ToolResult should be
// set on each Message; defensive in the other case but not required.
func toSDKMessage(m driven.Message) sdk.MessageParam {
	role := sdk.MessageParamRoleUser
	if m.Role == driven.RoleAssistant {
		role = sdk.MessageParamRoleAssistant
	}

	var blocks []sdk.ContentBlockParamUnion
	switch {
	case m.ToolUse != nil:
		blocks = append(blocks, sdk.NewToolUseBlock(m.ToolUse.ID, m.ToolUse.Input, m.ToolUse.Name))
	case m.ToolResult != nil:
		blocks = append(blocks, sdk.NewToolResultBlock(m.ToolResult.ToolUseID, m.ToolResult.Content, false))
	case m.Text != "":
		blocks = append(blocks, sdk.NewTextBlock(m.Text))
	}
	return sdk.MessageParam{Role: role, Content: blocks}
}

// toInputSchema lifts a map[string]any JSON schema into the SDK's
// typed ToolInputSchemaParam. Our prompts give us properties, required
// and additionalProperties; the SDK exposes Properties / Required
// directly and anything else flows through ExtraFields.
func toInputSchema(schema map[string]any) sdk.ToolInputSchemaParam {
	out := sdk.ToolInputSchemaParam{}
	if props, ok := schema["properties"]; ok {
		out.Properties = props
	}
	if req, ok := schema["required"].([]string); ok {
		out.Required = req
	} else if raw, ok := schema["required"].([]any); ok {
		// JSON schemas authored as []any in map literals — coerce to
		// []string for the SDK's typed field.
		reqs := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				reqs = append(reqs, s)
			}
		}
		out.Required = reqs
	}
	extras := map[string]any{}
	for k, v := range schema {
		switch k {
		case "type", "properties", "required":
			continue
		default:
			extras[k] = v
		}
	}
	if len(extras) > 0 {
		out.ExtraFields = extras
	}
	return out
}

// toChatResponse is the response-direction translator. It walks the
// returned content blocks to find the first tool_use (if any),
// accumulates text for the RawText fallback, and copies usage.
//
// We deliberately ignore text blocks when a tool_use is present —
// every prompt in v1 tells the model to call exactly one tool per
// turn and the loop inspects ToolUse first.
func toChatResponse(msg *sdk.Message) *driven.ChatResponse {
	resp := &driven.ChatResponse{
		StopReason: string(msg.StopReason),
		Usage: driven.Usage{
			InputTokens:         int(msg.Usage.InputTokens),
			OutputTokens:        int(msg.Usage.OutputTokens),
			CacheCreationTokens: int(msg.Usage.CacheCreationInputTokens),
			CacheReadTokens:     int(msg.Usage.CacheReadInputTokens),
		},
	}

	var text strings.Builder
	for _, block := range msg.Content {
		switch block.Type {
		case "tool_use":
			if resp.ToolUse != nil {
				continue // keep only the first; the loop classifies on Name
			}
			input := map[string]any{}
			if len(block.Input) > 0 {
				_ = json.Unmarshal(block.Input, &input)
			}
			resp.ToolUse = &driven.ToolUseContent{
				ID:    block.ID,
				Name:  block.Name,
				Input: input,
			}
		case "text":
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(block.Text)
		}
	}
	resp.RawText = text.String()
	return resp
}

// classifyError maps SDK / transport errors onto the port's sentinel
// set. Anything we don't recognize becomes a wrapped network error
// so the state machine can decide whether to retry.
func classifyError(err error) error {
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		return classifyAPIError(apiErr)
	}
	// Context cancellation propagates unchanged; the state machine
	// doesn't treat it as a retryable network error.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	// Everything else (DNS, connection refused, TLS, etc.) is a
	// network error — retryable under the backoff budget.
	return fmt.Errorf("%w: %v", driven.ErrAINetworkError, err)
}

// classifyAPIError routes on HTTP status. See
// https://docs.claude.com/en/api/errors for the full list.
func classifyAPIError(apiErr *sdk.Error) error {
	switch apiErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: %s", driven.ErrAIAuthFailed, apiErr.Error())
	case http.StatusRequestEntityTooLarge:
		return fmt.Errorf("%w: %s", driven.ErrAIContextTooLarge, apiErr.Error())
	case http.StatusTooManyRequests:
		rl := &driven.RateLimitedError{}
		if apiErr.Response != nil {
			rl.RetryAfter = parseRetryAfter(apiErr.Response.Header.Get("Retry-After"))
		}
		return rl
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Errorf("%w: %s", driven.ErrAIOverloaded, apiErr.Error())
	case http.StatusBadRequest:
		// 400 on Anthropic often means malformed request OR context
		// window exceeded; peek at the body text for a hint.
		if looksLikeContextTooLarge(apiErr) {
			return fmt.Errorf("%w: %s", driven.ErrAIContextTooLarge, apiErr.Error())
		}
		return fmt.Errorf("%w: %s", driven.ErrAINetworkError, apiErr.Error())
	default:
		// Treat other 4xx as fail-fast-ish; put them under network so
		// the state machine can decide. Most callers will see them
		// only via test stubs.
		return fmt.Errorf("%w: %s", driven.ErrAINetworkError, apiErr.Error())
	}
}

// looksLikeContextTooLarge sniffs the response body for the known
// "prompt is too long" phrase. The SDK's RawJSON accessor already
// has the body cached on the error.
func looksLikeContextTooLarge(apiErr *sdk.Error) bool {
	raw := strings.ToLower(apiErr.RawJSON())
	if strings.Contains(raw, "prompt is too long") ||
		strings.Contains(raw, "context_length_exceeded") ||
		strings.Contains(raw, "context window") {
		return true
	}
	if apiErr.Response == nil || apiErr.Response.Body == nil {
		return false
	}
	b, _ := io.ReadAll(apiErr.Response.Body)
	body := strings.ToLower(string(b))
	return strings.Contains(body, "prompt is too long") ||
		strings.Contains(body, "context_length_exceeded") ||
		strings.Contains(body, "context window")
}

// parseRetryAfter parses the Retry-After header. The spec allows
// either a delta-seconds integer or an HTTP-date; we only honor the
// integer form because Anthropic uses that form.
func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	n, err := strconv.Atoi(h)
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

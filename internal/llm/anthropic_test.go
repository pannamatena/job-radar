package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// TestAnthropic_RequestAndResponseShape checks the translation to and from the
// SDK against a local fake server: no network, no API key, no cost.
func TestAnthropic_RequestAndResponseShape(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{
		  "id": "msg_1", "type": "message", "role": "assistant", "model": "test-model",
		  "stop_reason": "tool_use",
		  "content": [
		    {"type": "text", "text": "Fetching it."},
		    {"type": "tool_use", "id": "toolu_1", "name": "get_posting", "input": {"posting_id": "p1"}}
		  ],
		  "usage": {"input_tokens": 12, "output_tokens": 34, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 2000}
		}`)
	}))
	defer srv.Close()

	a := &Anthropic{client: anthropic.NewClient(option.WithAPIKey("test"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0))}
	resp, err := a.Send(context.Background(), Request{
		Model: "test-model", MaxTokens: 100, System: "SYSTEM", CacheSystem: true,
		Tools: []Tool{{Name: "get_posting", Description: "d", Strict: true, Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"posting_id": map[string]any{"type": "string"}},
			"required":             []string{"posting_id"},
			"additionalProperties": false,
		}}},
		Messages: []Message{
			{Role: RoleUser, Blocks: []Block{{Type: BlockText, Text: "Score posting p1."}}},
			{Role: RoleAssistant, Blocks: []Block{{Type: BlockToolUse, ToolUseID: "toolu_0", ToolName: "get_posting", Input: json.RawMessage(`{"posting_id":"p1"}`)}}},
			{Role: RoleUser, Blocks: []Block{{Type: BlockToolResult, ToolUseID: "toolu_0", Text: "oops", IsError: true}}},
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Request: system block carries cache_control; tool is strict with
	// additionalProperties false; tool_result keeps is_error.
	sys := body["system"].([]any)[0].(map[string]any)
	if sys["cache_control"] == nil {
		t.Errorf("system block should have cache_control: %v", sys)
	}
	tool := body["tools"].([]any)[0].(map[string]any)
	if tool["strict"] != true {
		t.Errorf("tool should be strict: %v", tool)
	}
	if schema := tool["input_schema"].(map[string]any); schema["additionalProperties"] != false || schema["type"] != "object" {
		t.Errorf("schema not passed through: %v", schema)
	}
	msgs := body["messages"].([]any)
	result := msgs[2].(map[string]any)["content"].([]any)[0].(map[string]any)
	if result["type"] != "tool_result" || result["is_error"] != true || result["tool_use_id"] != "toolu_0" {
		t.Errorf("tool_result not translated: %v", result)
	}

	// Response.
	if resp.StopReason != StopToolUse || len(resp.Blocks) != 2 {
		t.Fatalf("response not translated: %+v", resp)
	}
	if b := resp.Blocks[1]; b.Type != BlockToolUse || b.ToolUseID != "toolu_1" || b.ToolName != "get_posting" {
		t.Errorf("tool_use block: %+v", b)
	}
	if resp.Usage.CacheReadTokens != 2000 || resp.Usage.OutputTokens != 34 {
		t.Errorf("usage: %+v", resp.Usage)
	}
}

func TestNewAnthropic_NeedsKey(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	if _, err := NewAnthropic(); err != ErrNoAPIKey {
		t.Errorf("want ErrNoAPIKey, got %v", err)
	}
}

func TestNewAnthropic_WorkspaceHeader(t *testing.T) {
	for _, tc := range []struct{ name, ws, want string }{
		{"sent when set", "wrkspc_123", "wrkspc_123"},
		{"absent when unset", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("anthropic-workspace-id")
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"id":"m","type":"message","role":"assistant","model":"x","stop_reason":"end_turn","content":[],"usage":{"input_tokens":1,"output_tokens":1}}`)
			}))
			defer srv.Close()
			t.Setenv(EnvAPIKey, "test")
			t.Setenv(EnvWorkspaceID, tc.ws)
			t.Setenv("ANTHROPIC_BASE_URL", srv.URL)
			a, err := NewAnthropic()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.Send(context.Background(), Request{Model: "x", MaxTokens: 1}); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("workspace header = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPriceCost(t *testing.T) {
	// Haiku 5.5 prices (checked 2026-10-09): 1M uncached in + 1M out.
	p := Price{Input: 0.10, Output: 0.50, CacheWrite: 0.125, CacheRead: 0.01}
	if got := p.Cost(Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000}); got != 0.60 {
		t.Errorf("cost = %v, want 0.60", got)
	}
}

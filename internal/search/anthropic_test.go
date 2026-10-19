package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/pannamatena/job-radar/internal/config"
)

func newTestSearcher(t *testing.T, handler http.HandlerFunc) *Anthropic {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Anthropic{
		client: anthropic.NewClient(option.WithAPIKey("test"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0)),
		model:  "test-model",
	}
}

// reply builds a Messages API response with one search result per url.
func reply(stop string, searches int, urls ...string) string {
	results := make([]map[string]any, 0, len(urls))
	for _, u := range urls {
		results = append(results, map[string]any{"type": "web_search_result", "url": u, "title": "Title " + u, "page_age": "1 day", "encrypted_content": "x"})
	}
	b, _ := json.Marshal(map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": "test-model", "stop_reason": stop,
		"content": []any{
			map[string]any{"type": "server_tool_use", "id": "srvtoolu_1", "name": "web_search", "input": map[string]any{"query": "q"}},
			map[string]any{"type": "web_search_tool_result", "tool_use_id": "srvtoolu_1", "content": results},
			map[string]any{"type": "text", "text": "done"},
		},
		"usage": map[string]any{"input_tokens": 1000, "output_tokens": 20, "server_tool_use": map[string]any{"web_search_requests": searches}},
	})
	return string(b)
}

func TestSearch_CollectsLinksAndUsage(t *testing.T) {
	var body map[string]any
	a := newTestSearcher(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, reply("end_turn", 1, "https://example.com/a", "https://example.com/b"))
	})
	res, err := a.Search(context.Background(), `site:jobs.ashbyhq.com "engineering manager"`, Options{MaxSearches: 2, BlockedDomains: []string{"blocked.example"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 || res.Candidates[0].URL != "https://example.com/a" || res.Candidates[0].Title == "" {
		t.Errorf("candidates: %+v", res.Candidates)
	}
	if res.Usage.Searches != 1 || res.Usage.InputTokens != 1000 || res.Usage.OutputTokens != 20 {
		t.Errorf("usage: %+v", res.Usage)
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("want exactly one tool (web search), got %v", body["tools"])
	}
	tool := tools[0].(map[string]any)
	if tool["type"] != "web_search_20250305" || tool["max_uses"].(float64) != 2 || tool["blocked_domains"].([]any)[0] != "blocked.example" {
		t.Errorf("tool settings not sent: %v", tool)
	}
	// Only the query goes out: nothing personal, and the page text is untrusted.
	sent, _ := json.Marshal(body["messages"])
	if !strings.Contains(string(sent), "engineering manager") {
		t.Errorf("the query should be sent: %s", sent)
	}
}

func TestSearch_ContinuesAPausedTurn(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	a := newTestSearcher(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			io.WriteString(w, reply("pause_turn", 1, "https://example.com/one"))
			return
		}
		io.WriteString(w, reply("end_turn", 1, "https://example.com/two"))
	})
	res, err := a.Search(context.Background(), "q", Options{MaxSearches: 2})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(res.Candidates) != 2 || res.Usage.Searches != 2 {
		t.Errorf("calls=%d candidates=%d searches=%d", calls, len(res.Candidates), res.Usage.Searches)
	}
}

func TestSearch_GivesUpIfItKeepsPausing(t *testing.T) {
	a := newTestSearcher(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, reply("pause_turn", 1))
	})
	if _, err := a.Search(context.Background(), "q", Options{MaxSearches: 1}); err == nil || !strings.Contains(err.Error(), "kept pausing") {
		t.Errorf("want a 'kept pausing' error, got %v", err)
	}
}

func TestSearch_APIErrorIsReturned(t *testing.T) {
	a := newTestSearcher(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"type":"error","error":{"type":"invalid_request_error","message":"web search is not enabled"}}`, http.StatusBadRequest)
	})
	if _, err := a.Search(context.Background(), "q", Options{MaxSearches: 1}); err == nil || !strings.Contains(err.Error(), "search:") {
		t.Errorf("want a search error, got %v", err)
	}
}

func TestUsageCost(t *testing.T) {
	u := Usage{Searches: 2, InputTokens: 1_000_000, OutputTokens: 1_000_000}
	got := u.CostUSD(config.ModelPrice{Input: 0.10, Output: 0.50}, 0.01)
	if got < 0.619 || got > 0.621 { // 2 searches = $0.02, tokens = $0.60
		t.Errorf("cost = %f, want 0.62", got)
	}
	if free := u.CostUSD(config.ModelPrice{Input: 0.10, Output: 0.50}, 0); free < 0.599 || free > 0.601 {
		t.Errorf("a provider with no per-search charge costs only tokens, got %f", free)
	}
}

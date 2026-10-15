package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const batchJSON = `{"id":"msgbatch_1","type":"message_batch","processing_status":"%s",
 "request_counts":{"processing":0,"succeeded":%d,"errored":%d,"canceled":0,"expired":0},
 "created_at":"2026-10-15T10:00:00Z","expires_at":"2026-10-16T10:00:00Z","results_url":null}`

// A fake batch API: create → in progress → ended; results come back in the
// opposite order to the requests, one of them errored.
func fakeBatchServer(t *testing.T, gets *atomic.Int32, created *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/messages/batches":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, created)
			io.WriteString(w, strings.NewReplacer("%s", "in_progress", "%d", "0").Replace(batchJSON))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/messages/batches/msgbatch_1":
			if gets.Add(1) < 2 {
				io.WriteString(w, strings.NewReplacer("%s", "in_progress", "%d", "0").Replace(batchJSON))
				return
			}
			b := strings.Replace(batchJSON, "%s", "ended", 1)
			b = strings.Replace(b, "%d", "1", 1)
			b = strings.Replace(b, "%d", "1", 1)
			io.WriteString(w, b)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/messages/batches/msgbatch_1/results":
			w.Header().Set("Content-Type", "application/x-jsonl")
			io.WriteString(w, `{"custom_id":"c1","result":{"type":"errored","error":{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}}}`+"\n")
			io.WriteString(w, `{"custom_id":"c0","result":{"type":"succeeded","message":{"id":"msg_1","type":"message","role":"assistant","model":"m","stop_reason":"tool_use","content":[{"type":"tool_use","id":"toolu_1","name":"get_posting","input":{"posting_id":"p"}}],"usage":{"input_tokens":7,"output_tokens":3,"cache_creation_input_tokens":0,"cache_read_input_tokens":11}}}}`+"\n")
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func TestSendBatch_WaitsThenMatchesResultsByID(t *testing.T) {
	var gets atomic.Int32
	var created map[string]any
	srv := fakeBatchServer(t, &gets, &created)
	defer srv.Close()
	a := &Anthropic{
		client:       anthropic.NewClient(option.WithAPIKey("test"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0)),
		pollInterval: time.Millisecond,
	}
	req := Request{Model: "m", MaxTokens: 10, System: "SYS", CacheSystem: true,
		Messages: []Message{{Role: RoleUser, Blocks: []Block{{Type: BlockText, Text: "hi"}}}}}

	var progressCalls int
	got, err := a.SendBatch(context.Background(), []BatchRequest{{ID: "c0", Request: req}, {ID: "c1", Request: req}},
		func(done, total int) { progressCalls++ })
	if err != nil {
		t.Fatal(err)
	}

	// Two requests were sent, with our ids, as one batch.
	reqs, _ := created["requests"].([]any)
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests in the batch body, got %v", created)
	}
	// Results are returned in request order even though the API sent them reversed.
	if len(got) != 2 || got[0].ID != "c0" || got[1].ID != "c1" {
		t.Fatalf("results out of order: %+v", got)
	}
	if got[0].Err != nil || got[0].Response.StopReason != StopToolUse || got[0].Response.Usage.CacheReadTokens != 11 ||
		len(got[0].Response.Blocks) != 1 || got[0].Response.Blocks[0].ToolName != "get_posting" {
		t.Errorf("succeeded result decoded wrongly: %+v", got[0])
	}
	if got[1].Err == nil || !strings.Contains(got[1].Err.Error(), "bad") {
		t.Errorf("errored result should carry the API's message, got %+v", got[1])
	}
	if gets.Load() < 2 || progressCalls == 0 {
		t.Errorf("should have polled until ended (gets=%d, progress calls=%d)", gets.Load(), progressCalls)
	}
}

func TestSendBatch_CancelsWhenContextIsCancelled(t *testing.T) {
	var cancelled atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel") {
			cancelled.Store(true)
		}
		io.WriteString(w, strings.NewReplacer("%s", "in_progress", "%d", "0").Replace(batchJSON))
	}))
	defer srv.Close()
	a := &Anthropic{
		client:       anthropic.NewClient(option.WithAPIKey("test"), option.WithBaseURL(srv.URL), option.WithMaxRetries(0)),
		pollInterval: 5 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, err := a.SendBatch(ctx, []BatchRequest{{ID: "c0", Request: Request{Model: "m", MaxTokens: 1,
		Messages: []Message{{Role: RoleUser, Blocks: []Block{{Type: BlockText, Text: "x"}}}}}}}, nil)
	if err == nil {
		t.Fatal("want an error when the context is cancelled")
	}
	if !cancelled.Load() {
		t.Error("the batch should be cancelled on the server so nothing keeps running")
	}
}

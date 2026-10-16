package eval

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/llm"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

// fakeConv needs `calls` model calls to finish. The last reply is the score; if a
// call comes back with an error, it gives up unscored.
type fakeConv struct {
	calls, made int
	score       int
	done        bool
	res         scorer.Result
}

func (f *fakeConv) Next() (llm.Request, bool) {
	if f.done {
		return llm.Request{}, false
	}
	f.made++
	return llm.Request{Model: "m"}, true
}

func (f *fakeConv) Apply(_ llm.Response, err error) {
	if err != nil {
		f.done = true
		f.res = scorer.Result{AI: &scorer.AIDetails{Status: scorer.AIUnscored, Note: err.Error()}}
		return
	}
	if f.made >= f.calls {
		f.done = true
		f.res = scorer.Result{Score: f.score, Track: "none", WorkModel: "remote",
			AI: &scorer.AIDetails{Status: scorer.AIScored, Turns: f.made, CostUSD: 0.001}}
	}
}

func (f *fakeConv) Result() scorer.Result { return f.res }

type fakeBatchClient struct {
	batchSizes []int
	failID     string
}

func (c *fakeBatchClient) SendBatch(_ context.Context, reqs []llm.BatchRequest, progress func(done, total int)) ([]llm.BatchResult, error) {
	c.batchSizes = append(c.batchSizes, len(reqs))
	var out []llm.BatchResult
	for _, r := range reqs {
		res := llm.BatchResult{ID: r.ID}
		if r.ID == c.failID {
			res.Err = errors.New("expired")
		}
		out = append(out, res)
	}
	if progress != nil {
		progress(len(reqs), len(reqs))
	}
	return out, nil
}

func TestRunBatch_RunsInRoundsUntilEveryScoringIsDone(t *testing.T) {
	cases := []Case{
		{Name: "quick", Label: Label{Company: "A", Title: "Quick", Score: 4}, Ad: "x"},
		{Name: "slow", Label: Label{Company: "A", Title: "Slow", Score: 2}, Ad: "x"},
		{Name: "dropped", Label: Label{Company: "A", Title: "Rail", Score: 1}, Ad: "x"},
	}
	// quick needs 1 call, slow needs 3.
	newConv := func(p posting.Posting) Conversation {
		if p.Title == "Slow" {
			return &fakeConv{calls: 3, score: 2}
		}
		return &fakeConv{calls: 1, score: 4}
	}
	pre := func(p posting.Posting) (bool, string) { return p.Title != "Rail", "drop: not software" }
	rules := func(context.Context, posting.Posting) scorer.Result { return scorer.Result{Score: 3} }
	client := &fakeBatchClient{}

	var rounds []int
	res, err := RunBatch(context.Background(), cases, rules, client, BatchOptions{
		Options:         Options{Scorer: "llm", Model: "m", PromptVersion: "v", Repeats: 2, AlertThreshold: 4, PreFilter: pre, Now: time.Now()},
		NewConversation: newConv,
		Round:           func(round, active int) { rounds = append(rounds, active) },
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2 repeats of quick + 2 of slow = 4 scorings. Round 1 sends all 4; after that
	// only the two slow ones remain, for rounds 2 and 3.
	if want := []int{4, 2, 2}; len(client.batchSizes) != 3 || client.batchSizes[0] != want[0] || client.batchSizes[1] != want[1] || client.batchSizes[2] != want[2] {
		t.Errorf("batch sizes = %v, want %v", client.batchSizes, want)
	}
	if len(rounds) != 3 {
		t.Errorf("round callback ran %d times, want 3", len(rounds))
	}
	if !res.Batch {
		t.Error("results should be marked as a batch run")
	}
	byName := map[string]CaseResult{}
	for _, c := range res.Cases {
		byName[c.Name] = c
	}
	if got := len(byName["quick"].Attempts); got != 2 || byName["quick"].Attempts[0].Score != 4 {
		t.Errorf("quick: %+v", byName["quick"])
	}
	if got := len(byName["slow"].Attempts); got != 2 || byName["slow"].Attempts[0].Turns != 3 {
		t.Errorf("slow: %+v", byName["slow"])
	}
	if d := byName["dropped"]; !d.Dropped || len(d.Attempts) != 0 {
		t.Errorf("a pre-filtered case must be recorded as dropped and never sent: %+v", d)
	}
	if a := byName["quick"].Attempts[0]; a.LatencyMS != 0 {
		t.Errorf("latency isn't measured in a batch, got %d", a.LatencyMS)
	}
}

func TestRunBatch_AFailedRequestLeavesThatScoringUnscored(t *testing.T) {
	cases := []Case{{Name: "a", Label: Label{Company: "A", Title: "T", Score: 3}, Ad: "x"}}
	rules := func(context.Context, posting.Posting) scorer.Result { return scorer.Result{Score: 3} }
	client := &fakeBatchClient{failID: "c0"}
	res, err := RunBatch(context.Background(), cases, rules, client, BatchOptions{
		Options:         Options{Scorer: "llm", Repeats: 1, AlertThreshold: 4, Now: time.Now()},
		NewConversation: func(posting.Posting) Conversation { return &fakeConv{calls: 2, score: 3} },
	})
	if err != nil {
		t.Fatal(err)
	}
	a := res.Cases[0].Attempts[0]
	if a.Scored || a.Note == "" {
		t.Errorf("an errored batch request should be an unfinished scoring with a reason: %+v", a)
	}
}

func TestSave_BatchRunsAreNamedAsSuch(t *testing.T) {
	r := Results{Date: "2026-10-15", Scorer: "llm", Model: "m", PromptVersion: "abc", Batch: true}
	p, err := Save(t.TempDir(), r)
	if err != nil {
		t.Fatal(err)
	}
	if got := p[len(p)-len("2026-10-15_abc_m_batch.json"):]; got != "2026-10-15_abc_m_batch.json" {
		t.Errorf("path = %s", p)
	}
}

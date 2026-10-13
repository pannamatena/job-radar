package agent

import (
	"context"
	"testing"

	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

// fixedRules returns a preset rules score.
type fixedRules int

func (f fixedRules) Score(context.Context, posting.Posting) (scorer.Result, error) {
	return scorer.Result{Score: int(f), Scorer: scorer.ScorerRules, Breakdown: []string{"+1 base"}}, nil
}

// fakeAI counts calls and returns a preset outcome.
type fakeAI struct {
	calls  int
	status string
	cost   float64
}

func (f *fakeAI) Score(context.Context, posting.Posting) scorer.Result {
	f.calls++
	r := scorer.Result{Score: 5, Scorer: scorer.ScorerLLM}
	if f.status != scorer.AIScored {
		r = scorer.Result{}
	}
	r.AI = &scorer.AIDetails{Status: f.status, CostUSD: f.cost}
	return r
}
func (f *fakeAI) Model() string         { return "m" }
func (f *fakeAI) PromptVersion() string { return "v" }

func TestGate_BelowMinRulesScoreNeverReachesAI(t *testing.T) {
	ai := &fakeAI{status: scorer.AIScored}
	g := &Gated{Rules: fixedRules(1), AI: ai, MinRulesScore: 2}
	r, _ := g.Score(context.Background(), posting.Posting{})
	if ai.calls != 0 || r.Score != 1 || r.AI.Status != scorer.AINotSent {
		t.Errorf("calls=%d score=%d status=%s", ai.calls, r.Score, r.AI.Status)
	}
}

func TestGate_AIScoreReplacesRulesScore(t *testing.T) {
	ai := &fakeAI{status: scorer.AIScored, cost: 0.001}
	g := &Gated{Rules: fixedRules(3), AI: ai, MinRulesScore: 2}
	r, _ := g.Score(context.Background(), posting.Posting{})
	if r.Score != 5 || r.Scorer != scorer.ScorerLLM || g.RunCostUSD != 0.001 {
		t.Errorf("score=%d scorer=%s cost=%v", r.Score, r.Scorer, g.RunCostUSD)
	}
}

func TestGate_UnscoredFallsBackToRules(t *testing.T) {
	ai := &fakeAI{status: scorer.AIUnscored, cost: 0.002}
	g := &Gated{Rules: fixedRules(3), AI: ai, MinRulesScore: 2}
	r, _ := g.Score(context.Background(), posting.Posting{})
	if r.Score != 3 || r.Scorer != scorer.ScorerRules || r.AI.Status != scorer.AIUnscored {
		t.Errorf("want the rules score kept, got %+v", r)
	}
	if g.SpentThisMonthUSD != 0.002 {
		t.Errorf("a failed attempt's spend must still count, got %v", g.SpentThisMonthUSD)
	}
}

func TestGate_PerRunCap(t *testing.T) {
	ai := &fakeAI{status: scorer.AIScored}
	g := &Gated{Rules: fixedRules(3), AI: ai, MinRulesScore: 2, MaxPerRun: 2}
	for i := 0; i < 5; i++ {
		g.Score(context.Background(), posting.Posting{})
	}
	if ai.calls != 2 || g.SkippedCap != 3 {
		t.Errorf("calls=%d skipped=%d, want 2 and 3", ai.calls, g.SkippedCap)
	}
}

func TestGate_MonthlyCap(t *testing.T) {
	ai := &fakeAI{status: scorer.AIScored, cost: 1}
	g := &Gated{Rules: fixedRules(3), AI: ai, MinRulesScore: 2, MonthlyCapUSD: 5, SpentThisMonthUSD: 3.5}
	for i := 0; i < 5; i++ {
		g.Score(context.Background(), posting.Posting{})
	}
	// 3.5 → 4.5 → 5.5, then stop.
	if ai.calls != 2 || g.SkippedCap != 3 {
		t.Errorf("calls=%d skipped=%d, want 2 and 3", ai.calls, g.SkippedCap)
	}
}

func TestGate_CachedScoreIsReusedForFree(t *testing.T) {
	ai := &fakeAI{status: scorer.AIScored}
	cached := scorer.Result{Score: 4, Scorer: scorer.ScorerLLM, AI: &scorer.AIDetails{Status: scorer.AICached}}
	g := &Gated{Rules: fixedRules(3), AI: ai, MinRulesScore: 2,
		Cache: func(context.Context, posting.Posting, string, string) (scorer.Result, bool, error) {
			return cached, true, nil
		}}
	r, _ := g.Score(context.Background(), posting.Posting{})
	if ai.calls != 0 || r.Score != 4 || g.Cached != 1 {
		t.Errorf("calls=%d score=%d cached=%d", ai.calls, r.Score, g.Cached)
	}
}

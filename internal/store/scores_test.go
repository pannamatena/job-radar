package store

import (
	"context"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

func TestScores_CacheAndSpend(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	p := mkPosting("greenhouse:a/1", "Engineering Manager")
	if _, err := s.SaveFetched(ctx, []posting.Posting{p}, now); err != nil {
		t.Fatal(err)
	}

	ai := scorer.Result{Score: 4, Track: "manager", WorkModel: "hybrid", Why: "w", Gap: "g", Flags: []string{"wants_go"},
		Scorer: scorer.ScorerLLM, AI: &scorer.AIDetails{Status: scorer.AIScored, Model: "m", PromptVersion: "v1", CostUSD: 0.002}}
	if err := s.SaveScore(ctx, p, ai, "", now); err != nil {
		t.Fatalf("SaveScore: %v", err)
	}
	// A rules-only score from last month must not count towards this month.
	rules := scorer.Result{Score: 2, Track: "none", WorkModel: "not_stated", Why: "w", Gap: "g", Scorer: scorer.ScorerRules,
		AI: &scorer.AIDetails{CostUSD: 1}}
	if err := s.SaveScore(ctx, p, rules, "", now.AddDate(0, -1, 0)); err != nil {
		t.Fatal(err)
	}

	got, ok, err := s.CachedAIScore(ctx, p, "m", "v1")
	if err != nil || !ok {
		t.Fatalf("want a cached score, got ok=%v err=%v", ok, err)
	}
	if got.Score != 4 || got.Flags[0] != "wants_go" || got.AI.Status != scorer.AICached {
		t.Errorf("cached result: %+v", got)
	}
	// A different prompt version or an edited ad is a cache miss.
	if _, ok, _ := s.CachedAIScore(ctx, p, "m", "v2"); ok {
		t.Errorf("different prompt version should miss")
	}
	edited := p
	edited.DescriptionText = "now requires Go"
	if _, ok, _ := s.CachedAIScore(ctx, edited, "m", "v1"); ok {
		t.Errorf("edited ad should miss")
	}

	spent, err := s.AISpendSince(ctx, StartOfMonth(now))
	if err != nil {
		t.Fatal(err)
	}
	if spent != 0.002 {
		t.Errorf("spend this month = %v, want 0.002", spent)
	}
}

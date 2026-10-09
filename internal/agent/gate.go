package agent

import (
	"context"
	"fmt"

	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

// AIScorer is the agent as the pipeline sees it (an interface so the gate can
// be tested without a model).
type AIScorer interface {
	Score(ctx context.Context, p posting.Posting) scorer.Result
	Model() string
	PromptVersion() string
}

// Cache returns an earlier AI score for the same posting content, model and
// prompt version, so a posting seen on every run is only paid for once.
type Cache func(ctx context.Context, p posting.Posting, model, promptVersion string) (scorer.Result, bool, error)

// Gated is the scorer used when `scorer: llm`. It puts every cost control in
// front of the AI (ADR-0007, ADR-0009):
//
//  1. the free rules scorer runs first; below MinRulesScore the posting keeps
//     its rules score and is never sent to the AI;
//  2. an earlier AI score for the identical ad + prompt + model is reused;
//  3. per-run and monthly spend caps; once reached, postings keep their rules
//     score and are counted as skipped;
//  4. if the AI can't finish (limits, API error), the rules score stands.
//
// It satisfies scorer.Scorer, so the pipeline doesn't care which mode is on.
type Gated struct {
	Rules         scorer.Scorer
	AI            AIScorer
	Cache         Cache // optional
	MinRulesScore int
	MaxPerRun     int
	MonthlyCapUSD float64
	// SpentThisMonthUSD starts at what's already been spent this calendar
	// month (from the database) and grows as this run spends.
	SpentThisMonthUSD float64

	// Stats for the end-of-run summary.
	Sent, Cached, NotSent, SkippedCap, Unscored int
	RunCostUSD                                  float64
}

// Score implements scorer.Scorer.
func (g *Gated) Score(ctx context.Context, p posting.Posting) (scorer.Result, error) {
	rules, err := g.Rules.Score(ctx, p)
	if err != nil {
		return rules, err
	}
	keepRules := func(status, note string) scorer.Result {
		r := rules
		r.AI = &scorer.AIDetails{Status: status, Note: note, Model: g.AI.Model(), PromptVersion: g.AI.PromptVersion()}
		return r
	}

	if rules.Score < g.MinRulesScore {
		g.NotSent++
		return keepRules(scorer.AINotSent, fmt.Sprintf("rules score %d is below the AI gate (%d)", rules.Score, g.MinRulesScore)), nil
	}

	if g.Cache != nil {
		if r, ok, err := g.Cache(ctx, p, g.AI.Model(), g.AI.PromptVersion()); err != nil {
			return rules, err
		} else if ok {
			g.Cached++
			return r, nil
		}
	}

	if g.MaxPerRun > 0 && g.Sent >= g.MaxPerRun {
		g.SkippedCap++
		return keepRules(scorer.AISkippedCap, fmt.Sprintf("per-run AI limit reached (%d)", g.MaxPerRun)), nil
	}
	if g.MonthlyCapUSD > 0 && g.SpentThisMonthUSD >= g.MonthlyCapUSD {
		g.SkippedCap++
		return keepRules(scorer.AISkippedCap, fmt.Sprintf("monthly AI spend cap reached ($%.2f of $%.2f)", g.SpentThisMonthUSD, g.MonthlyCapUSD)), nil
	}

	g.Sent++
	r := g.AI.Score(ctx, p)
	g.SpentThisMonthUSD += r.AI.CostUSD
	g.RunCostUSD += r.AI.CostUSD

	if r.AI.Status != scorer.AIScored {
		g.Unscored++
		fallback := rules
		ai := *r.AI
		ai.Note = ai.Note + " — kept the rules score"
		fallback.AI = &ai
		return fallback, nil
	}
	r.Breakdown = []string{fmt.Sprintf("rules pre-score %d/5 passed the AI gate (≥ %d); the AI's score replaces it", rules.Score, g.MinRulesScore)}
	return r, nil
}

package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
	"github.com/pannamatena/job-radar/internal/store"
)

// cmdScoreOne scores a single stored posting and prints the breakdown, so you
// can see exactly why it got its score. With the AI scorer it also prints the
// full agent transcript (every tool call and result) and the cost. The posting
// must already be in the database (seen by a previous `run` or `list`).
func cmdScoreOne(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("score-one", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	scorerOverride := fs.String("scorer", "", "rules or llm (default: the scorer in your config)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: job-radar score-one [--scorer rules|llm] <posting-id-or-url>  (ids come from `job-radar list --ids` or the `run` output)")
	}
	arg := fs.Arg(0)

	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}
	mode, err := scorerFlagValue(cfg, *scorerOverride)
	if err != nil {
		return err
	}

	st, err := store.Open(filepath.Join(*home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	// Accept either a posting id or a pasted URL.
	var p posting.Posting
	if strings.Contains(arg, "://") {
		p, err = st.GetByURL(ctx, arg)
	} else {
		p, err = st.GetByID(ctx, arg)
	}
	if err == store.ErrNotFound {
		return fmt.Errorf("no posting matching %q in the database yet — run `job-radar run` (or `list`) first, then copy an id (from `list --ids`) or a URL from the output", arg)
	}
	if err != nil {
		return err
	}

	rules, err := scorer.NewRules(cfg.Rules, cfg.Location).Score(ctx, p)
	if err != nil {
		return err
	}

	fmt.Printf("%s — %s\n%s\n\n", p.Company, p.Title, p.URL)
	if mode != config.ScorerLLM {
		printResult(rules)
		return nil
	}

	// AI mode. This is a debugging tool, so it always calls the model (no
	// rules gate, no reuse of a stored score) — but the monthly cap still holds.
	spent, err := st.AISpendSince(ctx, store.StartOfMonth(time.Now()))
	if err != nil {
		return err
	}
	if spent >= cfg.LLM.MonthlySpendCapUSD {
		return fmt.Errorf("the monthly AI spend cap is reached (~$%.2f of $%.2f); raise llm.monthly_spend_cap_usd to continue", spent, cfg.LLM.MonthlySpendCapUSD)
	}
	a, err := newAgent(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("Rules pre-score: %d/5 (the AI gate is %d", rules.Score, cfg.LLM.MinRulesScore)
	if rules.Score < cfg.LLM.MinRulesScore {
		fmt.Print("; `run` would NOT send this one to the AI")
	}
	fmt.Println(")")
	fmt.Printf("Scoring with %s (prompt %s)…\n\n", a.Model(), a.PromptVersion())

	r := a.Score(ctx, p)
	fmt.Println("Agent transcript:")
	for _, line := range r.AI.Transcript {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println()

	if r.AI.Status != scorer.AIScored {
		fmt.Printf("The AI didn't finish: %s. `run` would keep the rules score:\n\n", r.AI.Note)
		ai := r.AI
		r = rules
		r.AI = ai
	}
	printResult(r)
	ai := r.AI
	fmt.Printf("\nTokens: %d input, %d output, %d cache write, %d cache read · %d turn(s)",
		ai.InputTokens, ai.OutputTokens, ai.CacheWriteTokens, ai.CacheReadTokens, ai.Turns)
	if ai.SchemaFailures > 0 {
		fmt.Printf(" · %d invalid record_score call(s) corrected", ai.SchemaFailures)
	}
	fmt.Printf("\nEstimated cost: ~$%.5f (prices last checked %s)\n", ai.CostUSD, cfg.LLM.Pricing.LastChecked)

	// Record it: the spend is real and counts towards the cap, and a good
	// score is reused by the next `run` instead of paying again.
	return st.SaveScore(ctx, p, r, "", time.Now())
}

func printResult(r scorer.Result) {
	label := r.Scorer
	if r.Scorer == scorer.ScorerLLM {
		label = "AI, " + r.AI.Model
	}
	fmt.Printf("Score:      %d/5  (%s)\n", r.Score, label)
	fmt.Printf("Track:      %s\n", r.Track)
	fmt.Printf("Work model: %s\n", r.WorkModel)
	if r.Office != "" {
		fmt.Printf("Office:     %s\n", r.Office)
	}
	if len(r.Flags) > 0 {
		fmt.Printf("Flags:      %s\n", strings.Join(r.Flags, ", "))
	}
	fmt.Printf("Why:        %s\n", r.Why)
	fmt.Printf("Gap:        %s\n", r.Gap)
	if len(r.Breakdown) > 0 {
		fmt.Printf("\nHow the score was reached:\n")
		for _, step := range r.Breakdown {
			fmt.Printf("  %s\n", step)
		}
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pannamatena/job-radar/internal/agent"
	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/filter"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/notify"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
	"github.com/pannamatena/job-radar/internal/store"
	"github.com/pannamatena/job-radar/internal/version"
)

// cmdRun is the main pipeline: fetch → pre-filter → dedupe/store → match
// cross-source duplicates → score (rules, or rules-then-AI) → notify. Strong
// matches are printed to the terminal until email arrives in phase 4.
func cmdRun(ctx context.Context, logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	dryRun := fs.Bool("dry-run", false, "don't mark postings as seen or record the run (AI spend is still recorded: it's real)")
	scorerOverride := fs.String("scorer", "", "override the config's scorer for this run: rules or llm")
	if err := fs.Parse(args); err != nil {
		return err
	}

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

	var sc scorer.Scorer = scorer.NewRules(cfg.Rules, cfg.Location)
	var gate *agent.Gated
	if mode == config.ScorerLLM {
		if gate, err = newGatedScorer(ctx, cfg, st); err != nil {
			return err
		}
		sc = gate
	}

	client := httpclient.New(httpclient.Config{UserAgent: version.UserAgent()})
	pre := filter.New(cfg.PreFilter)

	started := time.Now()
	kept, dropped := gatherPostings(ctx, cfg, client, pre, logger)

	// Record which postings are new to the user (unless this is a dry run).
	newIDs := map[string]bool{}
	if !*dryRun {
		ids, err := st.SaveFetched(ctx, kept, started)
		if err != nil {
			return err
		}
		for _, id := range ids {
			newIDs[id] = true
		}
	}

	// Score each job once, even if it arrived from several sources (ADR-0026).
	toScore, duplicateOf := posting.Group(kept)

	type scored struct {
		p posting.Posting
		r scorer.Result
	}
	results := make([]scored, 0, len(toScore))
	byID := map[string]scorer.Result{}
	for _, p := range toScore {
		r, err := sc.Score(ctx, p)
		if err != nil {
			logger.Warn("scoring failed", "id", p.ID, "error", err.Error())
			continue
		}
		results = append(results, scored{p: p, r: r})
		byID[p.ID] = r
		if err := st.SaveScore(ctx, p, r, "", time.Now()); err != nil {
			logger.Warn("could not save score", "id", p.ID, "error", err.Error())
		}
	}
	// Duplicates share their canonical copy's score; they're stored (so the
	// link is visible) but not alerted on separately.
	for _, p := range kept {
		if canon, ok := duplicateOf[p.ID]; ok {
			if r, ok := byID[canon]; ok {
				r.AI = nil // the cost belongs to the canonical copy's row
				if err := st.SaveScore(ctx, p, r, canon, time.Now()); err != nil {
					logger.Warn("could not save score", "id", p.ID, "error", err.Error())
				}
			}
		}
	}
	// Highest score first.
	sort.SliceStable(results, func(i, j int) bool { return results[i].r.Score > results[j].r.Score })

	// Strong matches = score ≥ alert threshold. Flag the ones new this run.
	alertAt := cfg.Rules.Thresholds.Alert
	var alerts []notify.Alert
	for _, s := range results {
		if s.r.Score < alertAt {
			continue
		}
		alerts = append(alerts, notify.Alert{
			ID:      s.p.ID,
			Company: s.p.Company, Title: s.p.Title, Location: s.p.Location,
			Track: s.r.Track, WorkModel: s.r.WorkModel, Score: s.r.Score,
			Why: s.r.Why, Gap: s.r.Gap, URL: s.p.URL, Flags: s.r.Flags,
			Scorer: s.r.Scorer,
			New:    newIDs[s.p.ID],
		})
	}

	n := &notify.Terminal{W: os.Stdout}
	subject := fmt.Sprintf("job-radar: %d strong match(es) (score ≥ %d)", len(alerts), alertAt)
	if err := n.Send(ctx, subject, alerts); err != nil {
		return err
	}

	fmt.Printf("\nScored %d postings (%d dropped by pre-filter, %d cross-source duplicates). %d strong, %d new this run.\n",
		len(results), len(dropped), len(duplicateOf), len(alerts), len(newIDs))

	stats := store.RunStats{StartedAt: started, FinishedAt: time.Now(), NewPostings: len(newIDs), Scored: len(results)}
	if gate != nil {
		fmt.Printf("AI scorer (%s): %d sent, %d reused from earlier runs, %d below the rules gate, %d skipped by caps, %d fell back to rules.\n",
			gate.AI.Model(), gate.Sent, gate.Cached, gate.NotSent, gate.SkippedCap, gate.Unscored)
		fmt.Printf("AI cost this run: ~$%.4f. This month: ~$%.4f of your $%.2f cap.\n",
			gate.RunCostUSD, gate.SpentThisMonthUSD, cfg.LLM.MonthlySpendCapUSD)
		if gate.SkippedCap > 0 {
			fmt.Println("Some postings kept their free rules score because a cap was reached; they'll be AI-scored on a later run if they're still open.")
		}
		stats.CostEstimate, stats.AISent, stats.AISkipped = gate.RunCostUSD, gate.Sent, gate.SkippedCap
		logger.Info("ai scoring", "sent", gate.Sent, "cached", gate.Cached, "not_sent", gate.NotSent,
			"skipped_cap", gate.SkippedCap, "unscored", gate.Unscored, "cost_usd", gate.RunCostUSD)
	}
	if !*dryRun {
		if _, err := st.RecordRun(ctx, stats); err != nil {
			logger.Warn("could not record run", "error", err.Error())
		}
	}
	logger.Info("run complete", "scored", len(results), "dropped", len(dropped),
		"strong", len(alerts), "new", len(newIDs))
	return nil
}

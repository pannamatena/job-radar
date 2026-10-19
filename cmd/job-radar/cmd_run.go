package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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
	"github.com/pannamatena/job-radar/internal/search"
	"github.com/pannamatena/job-radar/internal/store"
	"github.com/pannamatena/job-radar/internal/version"
)

// cmdRun is the main pipeline: fetch → pre-filter → dedupe/store → match
// cross-source duplicates → score (rules, or rules-then-AI) → notify. Strong
// matches are printed to the terminal and, when email is on, the ones not
// emailed before are sent in one message.
func cmdRun(ctx context.Context, logger *slog.Logger, args []string) (runErr error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	dryRun := fs.Bool("dry-run", false, "don't mark postings as seen or record the run (AI spend is still recorded: it's real)")
	scorerOverride := fs.String("scorer", "", "override the config's scorer for this run: rules or llm")
	noSearch := fs.Bool("no-search", false, "skip web search for this run (search costs money)")
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

	// If the run fails after this point, tell the user (at most once a day).
	defer func() {
		if runErr != nil {
			sendProblem(ctx, logger, cfg, st, []string{"The run failed: " + runErr.Error()}, *dryRun, os.Stdout)
		}
	}()

	var sc scorer.Scorer = scorer.NewRules(cfg.Rules, cfg.Location)
	var gate *agent.Gated
	if mode == config.ScorerLLM {
		if gate, err = newGatedScorer(ctx, cfg, st); err != nil {
			return err
		}
		sc = gate
	}

	if !*dryRun {
		maybeSyncFeedback(ctx, logger, cfg, st, os.Stdout)
	}

	client := httpclient.New(httpclient.Config{UserAgent: version.UserAgent()})
	pre := filter.New(cfg.PreFilter)

	started := time.Now()
	kept, dropped, outcomes := gatherPostings(ctx, cfg, client, pre, logger)
	var problems []string
	if !*dryRun {
		problems = recordSourceHealth(ctx, logger, st, outcomes, started)
	}

	// Jobs found by web search join the watch-list's postings (ADR-0037).
	if cfg.Search.Enabled && !*noSearch {
		searcher, err := search.New(cfg.Search.Provider, cfg.Search.Model)
		if err != nil {
			problems = append(problems, "Web search was skipped: "+err.Error())
		} else {
			sk, sd, sp := searchStage(ctx, logger, cfg, st, pre, searcher, boardFetcher(client), postingReader(client), *dryRun, time.Now(), os.Stdout)
			kept, dropped, problems = append(kept, sk...), append(dropped, sd...), append(problems, sp...)
		}
	}

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
	if _, err := n.Send(ctx, notify.Message{Kind: notify.KindAlert, Subject: subject, Sections: []notify.Section{{Alerts: alerts}}}); err != nil {
		return err
	}
	if len(alerts) > 0 {
		sendAlertEmail(ctx, logger, cfg, st, alerts, *dryRun, os.Stdout)
	}

	maybeSendDigest(ctx, logger, cfg, st, *dryRun, os.Stdout)

	fmt.Printf("\nScored %d postings (%d dropped by pre-filter, %d cross-source duplicates). %d strong, %d new this run.\n",
		len(results), len(dropped), len(duplicateOf), len(alerts), len(newIDs))

	stats := store.RunStats{StartedAt: started, FinishedAt: time.Now(), NewPostings: len(newIDs), Scored: len(results)}
	if gate != nil {
		if notice := aiRunSummary(os.Stdout, logger, cfg, gate, &stats); notice != "" {
			problems = append(problems, notice)
		}
	}
	// The one problem email goes out last, after every part of the run has had
	// the chance to add to problems (it used to go first and miss the AI cap).
	sendProblem(ctx, logger, cfg, st, problems, *dryRun, os.Stdout)
	if !*dryRun {
		if _, err := st.RecordRun(ctx, stats); err != nil {
			logger.Warn("could not record run", "error", err.Error())
		}
	}
	logger.Info("run complete", "scored", len(results), "dropped", len(dropped),
		"strong", len(alerts), "new", len(newIDs))
	return nil
}

// aiRunSummary prints how the AI scorer did, fills in the run's AI stats, and
// returns a notice for the user if the monthly spend cap was reached or is close.
func aiRunSummary(w io.Writer, logger *slog.Logger, cfg *config.Config, gate *agent.Gated, stats *store.RunStats) string {
	fmt.Fprintf(w, "AI scorer (%s): %d sent, %d reused from earlier runs, %d below the rules gate, %d skipped by caps, %d fell back to rules.\n",
		gate.AI.Model(), gate.Sent, gate.Cached, gate.NotSent, gate.SkippedCap, gate.Unscored)
	fmt.Fprintf(w, "AI cost this run: ~$%.4f. This month: ~$%.4f of your $%.2f cap.\n",
		gate.RunCostUSD, gate.SpentThisMonthUSD, cfg.LLM.MonthlySpendCapUSD)
	if gate.SkippedCap > 0 {
		fmt.Fprintln(w, "Some postings kept their free rules score because a cap was reached; they'll be AI-scored on a later run if they're still open.")
	}
	stats.CostEstimate, stats.AISent, stats.AISkipped = gate.RunCostUSD, gate.Sent, gate.SkippedCap
	logger.Info("ai scoring", "sent", gate.Sent, "cached", gate.Cached, "not_sent", gate.NotSent,
		"skipped_cap", gate.SkippedCap, "unscored", gate.Unscored, "cost_usd", gate.RunCostUSD)
	return capNotice("AI", "so new postings keep their free rules score until next month",
		"llm.monthly_spend_cap_usd", gate.SpentThisMonthUSD-gate.RunCostUSD, gate.SpentThisMonthUSD, cfg.LLM.MonthlySpendCapUSD)
}

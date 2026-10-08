package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/filter"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/notify"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
	"github.com/pannamatena/job-radar/internal/version"
)

// cmdRun is the main pipeline: fetch → pre-filter → dedupe/store → score →
// notify. In phase 2b it scores with the free rules scorer and prints strong
// matches to the terminal (email arrives in phase 4).
func cmdRun(ctx context.Context, logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	dryRun := fs.Bool("dry-run", false, "don't record this run or mark postings as seen")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}
	if cfg.Scorer == config.ScorerLLM {
		fmt.Fprintln(os.Stderr, "note: the LLM scorer arrives in phase 3; scoring with the free rules scorer for now.")
	}

	client := httpclient.New(httpclient.Config{UserAgent: version.UserAgent()})
	pre := filter.New(cfg.PreFilter)
	sc := scorer.NewRules(cfg.Rules, cfg.Location)

	started := time.Now()
	kept, dropped := gatherPostings(ctx, cfg, client, pre, logger)

	// Record which postings are new to the user (unless this is a dry run).
	newIDs := map[string]bool{}
	if !*dryRun {
		ids, err := persist(ctx, *home, kept, started, logger)
		if err != nil {
			return err
		}
		for _, id := range ids {
			newIDs[id] = true
		}
	}

	// Score everything the pre-filter kept.
	type scored struct {
		p posting.Posting
		r scorer.Result
	}
	results := make([]scored, 0, len(kept))
	for _, p := range kept {
		r, err := sc.Score(ctx, p)
		if err != nil {
			logger.Warn("scoring failed", "id", p.ID, "error", err.Error())
			continue
		}
		results = append(results, scored{p: p, r: r})
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
			New: newIDs[s.p.ID],
		})
	}

	n := &notify.Terminal{W: os.Stdout}
	subject := fmt.Sprintf("job-radar: %d strong match(es) (score ≥ %d)", len(alerts), alertAt)
	if err := n.Send(ctx, subject, alerts); err != nil {
		return err
	}

	fmt.Printf("\nScored %d postings (%d dropped by pre-filter). %d strong, %d new this run.\n",
		len(results), len(dropped), len(alerts), len(newIDs))
	logger.Info("run complete", "scored", len(results), "dropped", len(dropped),
		"strong", len(alerts), "new", len(newIDs))
	return nil
}

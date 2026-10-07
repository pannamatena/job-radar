package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/scorer"
	"github.com/pannamatena/job-radar/internal/store"
)

// cmdScoreOne scores a single stored posting and prints the breakdown, so you
// can see exactly why it got its score. The posting must already be in the
// database (seen by a previous `run` or `list`).
func cmdScoreOne(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("score-one", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: job-radar score-one <posting_id>  (get ids from `job-radar list`)")
	}
	id := fs.Arg(0)

	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}

	st, err := store.Open(filepath.Join(*home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	p, err := st.GetByID(ctx, id)
	if err == store.ErrNotFound {
		return fmt.Errorf("no posting with id %q in the database yet — run `job-radar run` (or `list`) first, then copy an id from the output", id)
	}
	if err != nil {
		return err
	}

	r, err := scorer.NewRules(cfg.Rules, cfg.Location).Score(ctx, p)
	if err != nil {
		return err
	}

	fmt.Printf("%s — %s\n%s\n\n", p.Company, p.Title, p.URL)
	fmt.Printf("Score:      %d/5  (%s)\n", r.Score, r.Scorer)
	fmt.Printf("Track:      %s\n", r.Track)
	fmt.Printf("Work model: %s\n", r.WorkModel)
	if len(r.Flags) > 0 {
		fmt.Printf("Flags:      %s\n", strings.Join(r.Flags, ", "))
	}
	fmt.Printf("Why:        %s\n", r.Why)
	fmt.Printf("Gap:        %s\n", r.Gap)
	fmt.Printf("\nHow the score was reached:\n")
	for _, step := range r.Breakdown {
		fmt.Printf("  %s\n", step)
	}
	return nil
}

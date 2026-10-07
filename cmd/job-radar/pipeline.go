package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/filter"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/sources"
)

// gatherPostings fetches every configured company through the shared client and
// splits the results into those the pre-filter keeps and those it drops. One
// company failing never stops the rest. Shared by `list` and `run`.
func gatherPostings(ctx context.Context, cfg *config.Config, client *httpclient.Client, pre *filter.Filter, logger *slog.Logger) (kept, dropped []posting.Posting) {
	for _, co := range cfg.Companies {
		src, err := sources.For(client, co)
		if err != nil {
			if errors.Is(err, sources.ErrNoPublicAPI) {
				fmt.Fprintf(os.Stderr, "  · %s: no public API (covered by job-alert emails in phase 5) — skipping\n", co.Name)
			} else {
				fmt.Fprintf(os.Stderr, "  ! %s: %s\n", co.Name, err)
			}
			continue
		}
		postings, err := src.Fetch(ctx)
		if err != nil {
			logger.Warn("source failed", "company", co.Name, "error", err.Error())
			fmt.Fprintf(os.Stderr, "  ! %s: %s\n", co.Name, err)
			continue
		}
		logger.Info("fetched", "company", co.Name, "source", src.Name(), "postings", len(postings))
		for _, p := range postings {
			if pre.Check(p).Keep {
				kept = append(kept, p)
			} else {
				dropped = append(dropped, p)
			}
		}
	}
	return kept, dropped
}

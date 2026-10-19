package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/filter"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/search"
	"github.com/pannamatena/job-radar/internal/sources"
	"github.com/pannamatena/job-radar/internal/store"
)

// searchStage finds jobs by web search (ADR-0037) and returns the postings it
// found in the same kept/dropped shape as the watch-list, so they go through the
// same scoring. Leads from other sites are saved for the digest. Problems come
// back as text for the problem notice; a problem here never stops the run.
//
// Search costs real money, so its spend is recorded even in a dry run.
func searchStage(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, pre *filter.Filter,
	searcher search.Searcher, fetch search.FetchBoard, one search.FetchOne, dryRun bool, now time.Time, w io.Writer) (kept, dropped []posting.Posting, problems []string) {

	active := cfg.Search.ActiveQueries()
	if len(active) == 0 {
		return nil, nil, nil
	}
	cap := cfg.Search.MonthlySpendCapUSD
	spent, err := st.SearchSpendSince(ctx, store.StartOfMonth(now))
	if err != nil {
		return nil, nil, []string{"Search was skipped: " + err.Error()}
	}
	const searchConsequence = "so no new jobs are searched for until next month"
	if spent >= cap {
		fmt.Fprintf(w, "Search: skipped, the monthly search cap ($%.2f) is reached.\n", cap)
		return nil, nil, []string{capNotice("web search", searchConsequence, "search.monthly_spend_cap_usd", spent, spent, cap)}
	}
	start, err := st.SearchQueriesRun(ctx)
	if err != nil {
		return nil, nil, []string{"Search was skipped: " + err.Error()}
	}
	picked := search.PickQueries(active, cfg.Search.QueriesPerRun, start)

	finder := &search.Finder{Searcher: searcher, Cfg: cfg.Search, Price: cfg.LLM.Pricing.Models[cfg.Search.Model]}
	found := finder.Run(ctx, picked, cap-spent)

	if err := st.RecordSearchRun(ctx, store.SearchRun{At: now, Queries: found.Queries, Searches: int(found.Usage.Searches), CostUSD: found.CostUSD}); err != nil {
		logger.Warn("could not record search run", "error", err.Error())
	}
	if len(found.Errors) > 0 && len(found.Errors) == found.Queries {
		problems = append(problems, "Web search failed: "+found.Errors[0])
	} else if len(found.Errors) > 0 {
		logger.Warn("some searches failed", "failed", len(found.Errors), "of", found.Queries, "first", found.Errors[0])
	}

	newLeads := 0
	if !dryRun {
		for _, l := range found.Leads {
			isNew, err := st.SaveLead(ctx, store.Lead{URL: l.URL, Host: l.Host, Title: l.Title, Known: l.Known}, now)
			if err != nil {
				logger.Warn("could not save lead", "error", err.Error())
				continue
			}
			if isNew {
				newLeads++
			}
		}
	}

	postings, rs, resolveProblems := search.Resolve(ctx, fetch, one, found.Refs, cfg.Companies, cfg.Search.IgnoreCompanies)
	for _, p := range resolveProblems {
		logger.Warn("search: " + p)
	}
	for _, p := range postings {
		if pre.Check(p).Keep {
			kept = append(kept, p)
		} else {
			dropped = append(dropped, p)
		}
	}

	monthSpend := spent + found.CostUSD
	if notice := capNotice("web search", searchConsequence, "search.monthly_spend_cap_usd", spent, monthSpend, cap); notice != "" {
		problems = append(problems, notice)
	}
	fmt.Fprintf(w, "Search: %d of %d queries run, %d searches, ~$%.3f (this month ~$%.2f of your $%.2f cap).\n",
		found.Queries, len(active), found.Usage.Searches, found.CostUSD, monthSpend, cap)
	fmt.Fprintf(w, "  %d posting(s) found on known job boards: %d read (%d kept by the pre-filter)", rs.Found, rs.Read, len(kept))
	for _, part := range []struct {
		n    int
		what string
	}{{rs.Closed, "no longer listed"}, {rs.OnWatch, "already on your watch-list"}, {rs.Ignored, "on your ignore list"}, {rs.FailedJobs, "on boards that couldn't be read"}} {
		if part.n > 0 {
			fmt.Fprintf(w, ", %d %s", part.n, part.what)
		}
	}
	fmt.Fprintf(w, "; %d lead(s) from other sites", len(found.Leads))
	if !dryRun {
		fmt.Fprintf(w, " (%d new)", newLeads)
	}
	fmt.Fprintln(w, ".")
	logger.Info("search", "queries", found.Queries, "searches", found.Usage.Searches, "cost_usd", found.CostUSD,
		"postings", len(postings), "leads", len(found.Leads))
	return kept, dropped, problems
}

// boardFetcher reads a company's whole board through the shared, polite client.
func boardFetcher(client *httpclient.Client) search.FetchBoard {
	return func(ctx context.Context, co config.Company) ([]posting.Posting, error) {
		src, err := sources.For(client, co)
		if err != nil {
			return nil, err
		}
		return src.Fetch(ctx)
	}
}

// postingReader reads one posting through the shared, polite client. It reports
// sources.ErrNoSingleRead for a board that can only be read as a whole.
func postingReader(client *httpclient.Client) search.FetchOne {
	return func(ctx context.Context, co config.Company, jobID string) (posting.Posting, bool, error) {
		return sources.ReadOne(ctx, client, co, jobID)
	}
}

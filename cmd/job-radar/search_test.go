package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/filter"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/search"
	"github.com/pannamatena/job-radar/internal/store"
)

const searchJobID = "43dbf072-0fc1-48c9-8c1e-7416db7d4a14"

type stubSearcher struct {
	candidates []search.Candidate
	usage      search.Usage
	err        error
	queries    []string
}

func (s *stubSearcher) Search(_ context.Context, q string, _ search.Options) (search.Result, error) {
	s.queries = append(s.queries, q)
	return search.Result{Candidates: s.candidates, Usage: s.usage}, s.err
}

func searchConfig() *config.Config {
	return &config.Config{
		LLM: config.LLM{Pricing: config.Pricing{Models: map[string]config.ModelPrice{"m": {Input: 1, Output: 1}}}},
		Search: config.Search{
			Enabled: true, Model: "m", Tracks: []string{"manager"},
			Queries:             map[string][]string{"manager": {"q1", "q2", "q3"}},
			QueriesPerRun:       2,
			MaxSearchesPerQuery: 2, MonthlySpendCapUSD: 1, LinkDomains: []string{"linkedin.com"},
		},
	}
}

func runSearchStage(t *testing.T, cfg *config.Config, st *store.Store, s search.Searcher, dry bool) (kept []posting.Posting, problems []string, out string) {
	t.Helper()
	fetch := func(_ context.Context, co config.Company) ([]posting.Posting, error) {
		if co.Slug == "down" {
			return nil, errors.New("503")
		}
		return []posting.Posting{{ID: "ashby:acme/" + searchJobID, Company: "acme", Title: "Engineering Manager", Location: "Manchester"}}, nil
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kept, _, problems = searchStage(context.Background(), logger, cfg, st, filter.New(cfg.PreFilter), s, fetch, nil, dry, time.Now(), &buf)
	return kept, problems, buf.String()
}

func openSearchStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "radar.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSearchStageFindsPostingsSavesLeadsAndRecordsSpend(t *testing.T) {
	st, cfg := openSearchStore(t), searchConfig()
	s := &stubSearcher{usage: search.Usage{Searches: 1}, candidates: []search.Candidate{
		{URL: "https://jobs.ashbyhq.com/acme/" + searchJobID, Title: "EM"},
		{URL: "https://news.example.com/job/1", Title: "A job"},
	}}
	kept, problems, out := runSearchStage(t, cfg, st, s, false)

	if len(kept) != 1 || kept[0].ID != "ashby:acme/"+searchJobID {
		t.Errorf("kept = %+v", kept)
	}
	if len(problems) != 0 {
		t.Errorf("problems = %v", problems)
	}
	if len(s.queries) != 2 {
		t.Errorf("queries_per_run is 2, ran %v", s.queries)
	}
	leads, _ := st.LeadsSince(context.Background(), time.Now().Add(-time.Hour))
	if len(leads) != 1 || leads[0].Host != "news.example.com" || leads[0].Known {
		t.Errorf("leads = %+v", leads)
	}
	if n, _ := st.SearchQueriesRun(context.Background()); n != 2 {
		t.Errorf("the run should be recorded (2 queries), got %d", n)
	}
	if !strings.Contains(out, "2 of 3 queries run") {
		t.Errorf("summary missing: %q", out)
	}

	// The next run starts where this one stopped, so every query gets a turn.
	s2 := &stubSearcher{}
	runSearchStage(t, cfg, st, s2, false)
	if strings.Join(s2.queries, ",") != "q3,q1" {
		t.Errorf("second run should continue the rotation, ran %v", s2.queries)
	}
}

func TestSearchStageDryRunSpendsButSavesNoLeads(t *testing.T) {
	st, cfg := openSearchStore(t), searchConfig()
	s := &stubSearcher{usage: search.Usage{Searches: 1}, candidates: []search.Candidate{{URL: "https://news.example.com/job/1", Title: "A job"}}}
	runSearchStage(t, cfg, st, s, true)
	if leads, _ := st.LeadsSince(context.Background(), time.Now().Add(-time.Hour)); len(leads) != 0 {
		t.Errorf("a dry run must not save leads, got %+v", leads)
	}
	if spend, _ := st.SearchSpendSince(context.Background(), store.StartOfMonth(time.Now())); spend <= 0 {
		t.Error("search money is spent even in a dry run, so it must be recorded")
	}
}

func TestSearchStageStopsAtTheMonthlyCap(t *testing.T) {
	st, cfg := openSearchStore(t), searchConfig()
	if err := st.RecordSearchRun(context.Background(), store.SearchRun{At: time.Now(), Queries: 5, Searches: 5, CostUSD: 1.5}); err != nil {
		t.Fatal(err)
	}
	s := &stubSearcher{}
	_, problems, out := runSearchStage(t, cfg, st, s, false)
	if len(s.queries) != 0 {
		t.Errorf("no search once the cap is reached, ran %v", s.queries)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "monthly spend cap") || !strings.Contains(out, "skipped") {
		t.Errorf("problems=%v out=%q", problems, out)
	}
}

func TestSearchStageReportsWhenEveryQueryFails(t *testing.T) {
	st, cfg := openSearchStore(t), searchConfig()
	s := &stubSearcher{err: errors.New("web search is not enabled")}
	_, problems, _ := runSearchStage(t, cfg, st, s, false)
	if len(problems) != 1 || !strings.Contains(problems[0], "Web search failed") {
		t.Errorf("problems = %v", problems)
	}
}

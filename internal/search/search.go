package search

import (
	"context"

	"github.com/pannamatena/job-radar/internal/config"
)

// Candidate is one link a search returned. It is untrusted text: nothing here
// has been fetched or checked yet.
type Candidate struct {
	URL     string
	Title   string
	PageAge string
}

// Usage is what a search call consumed.
type Usage struct {
	Searches     int64
	InputTokens  int64
	OutputTokens int64
	CacheWrite   int64
	CacheRead    int64
}

// Add returns the sum of two usages.
func (u Usage) Add(o Usage) Usage {
	return Usage{u.Searches + o.Searches, u.InputTokens + o.InputTokens, u.OutputTokens + o.OutputTokens,
		u.CacheWrite + o.CacheWrite, u.CacheRead + o.CacheRead}
}

// CostUSD estimates the cost of u: perSearch dollars for each search (the
// provider's per-search charge, from config) plus tokens at the given
// per-million-token prices. A search that errors is not billed, so this is an
// upper bound.
func (u Usage) CostUSD(p config.ModelPrice, perSearch float64) float64 {
	return float64(u.Searches)*perSearch +
		(float64(u.InputTokens)*p.Input+float64(u.CacheWrite)*p.CacheWrite+float64(u.CacheRead)*p.CacheRead+float64(u.OutputTokens)*p.Output)/1e6
}

// Options limit one query.
type Options struct {
	MaxSearches    int      // at most this many searches for the query
	BlockedDomains []string // sites never to take results from
}

// Result is what one query found.
type Result struct {
	Candidates []Candidate
	Usage      Usage
}

// Searcher runs a web search query and returns the links it found. The
// Anthropic web search is the first implementation (ADR-0037); the rest of
// the package doesn't depend on which provider is behind it.
type Searcher interface {
	Search(ctx context.Context, query string, opt Options) (Result, error)
}

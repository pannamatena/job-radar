package config

import (
	"fmt"
	"strings"
)

// Search configures finding jobs by web search (ADR-0037). A search costs about
// a cent plus tokens, so it is off until switched on, and it only runs the
// queries of the tracks listed in Tracks. Other tracks are matched against the
// free watch-list boards only.
type Search struct {
	// Enabled switches web search on. Default off.
	Enabled bool `yaml:"enabled"`
	// Provider says which search provider to use. Default "anthropic". New
	// providers are added in code (internal/search/provider.go) and named here.
	Provider string `yaml:"provider"`
	// Model is the model that runs the searches, separate from llm.model.
	// Default claude-haiku-5-5. It needs a price under llm.pricing.models.
	Model string `yaml:"model"`
	// PricePerSearchUSD is what the provider charges for each web search, on top
	// of tokens. Default 0.01 (Anthropic: $10 per 1,000 searches). A provider
	// with no per-search charge sets it to 0.
	PricePerSearchUSD *float64 `yaml:"price_per_search_usd"`
	// Tracks lists the tracks to search the web for, for example
	// [manager, lead_ic]. Each must be a track under rules.tracks.
	Tracks []string `yaml:"tracks"`
	// Queries maps a track name to its search queries. Queries restricted to a
	// job-board host (site:jobs.ashbyhq.com …) find postings; general queries
	// mostly return other sites, which are shown as unscored leads.
	Queries map[string][]string `yaml:"queries"`
	// QueriesPerRun limits how many queries one run uses; the rest wait for
	// the next run, round and round, so every query still runs. 0 means all.
	QueriesPerRun int `yaml:"queries_per_run"`
	// MaxSearchesPerQuery caps how many searches the model may make for one
	// query. Default 2.
	MaxSearchesPerQuery int `yaml:"max_searches_per_query"`
	// MonthlySpendCapUSD stops searching for the rest of the calendar month
	// once the estimated spend reaches it. Default 5.
	MonthlySpendCapUSD float64 `yaml:"monthly_spend_cap_usd"`
	// BlockedDomains are sites never to take results from, as bare domains
	// (example.com). Empty by default.
	BlockedDomains []string `yaml:"blocked_domains"`
	// IgnoreCompanies are company names whose postings are dropped, for
	// example your own employer.
	IgnoreCompanies []string `yaml:"ignore_companies"`
	// LinkDomains are well-known sites whose links are shown as normal links
	// in the digest's leads (exact host or its sub-domains). Every other site
	// is shown as text that email programs can't turn into a link, with a
	// warning. Default linkedin.com. job-radar never fetches any of them.
	LinkDomains []string `yaml:"link_domains"`
}

// DefaultSearchProvider is the provider used when none is named.
const DefaultSearchProvider = "anthropic"

// defaultPricePerSearchUSD is Anthropic's price for one web search.
const defaultPricePerSearchUSD = 0.01

// PricePerSearch is the cost of one search, with the default applied.
func (s *Search) PricePerSearch() float64 {
	if s.PricePerSearchUSD == nil {
		return defaultPricePerSearchUSD
	}
	return *s.PricePerSearchUSD
}

func (s *Search) applyDefaults() {
	if strings.TrimSpace(s.Provider) == "" {
		s.Provider = DefaultSearchProvider
	}
	if strings.TrimSpace(s.Model) == "" {
		s.Model = "claude-haiku-5-5"
	}
	if s.MaxSearchesPerQuery == 0 {
		s.MaxSearchesPerQuery = 2
	}
	if s.MonthlySpendCapUSD == 0 {
		s.MonthlySpendCapUSD = 5
	}
	if s.LinkDomains == nil {
		s.LinkDomains = []string{"linkedin.com"}
	}
}

// ActiveQueries is the list of queries for the tracks being searched, in a
// stable order (track order as listed, then query order).
func (s *Search) ActiveQueries() []string {
	var out []string
	for _, t := range s.Tracks {
		out = append(out, s.Queries[t]...)
	}
	return out
}

const maxQueryLength = 300

// validate checks the search block. Only called when search is enabled, so
// users who don't search never see errors about settings they don't use.
func (s Search) validate(c *Config) []string {
	var problems []string
	if len(s.Tracks) == 0 {
		problems = append(problems, "search.tracks: list at least one track to search the web for (for example [manager])")
	}
	for _, t := range s.Tracks {
		if _, ok := c.Rules.Tracks[t]; !ok {
			problems = append(problems, fmt.Sprintf("search.tracks: %q is not a track under rules.tracks (have: %s)", t, strings.Join(c.TrackNames(), ", ")))
			continue
		}
		if len(s.Queries[t]) == 0 {
			problems = append(problems, fmt.Sprintf("search.queries: track %q is in search.tracks but has no queries", t))
		}
	}
	for t, qs := range s.Queries {
		for i, q := range qs {
			switch {
			case strings.TrimSpace(q) == "":
				problems = append(problems, fmt.Sprintf("search.queries.%s[%d]: a query can't be empty", t, i))
			case len(q) > maxQueryLength:
				problems = append(problems, fmt.Sprintf("search.queries.%s[%d]: a query is at most %d characters", t, i, maxQueryLength))
			}
		}
	}
	if s.QueriesPerRun < 0 {
		problems = append(problems, "search.queries_per_run: can't be negative (0 means all queries)")
	}
	if s.MaxSearchesPerQuery < 1 || s.MaxSearchesPerQuery > 5 {
		problems = append(problems, fmt.Sprintf("search.max_searches_per_query: %d is not valid; use 1–5", s.MaxSearchesPerQuery))
	}
	if s.MonthlySpendCapUSD < 0 {
		problems = append(problems, "search.monthly_spend_cap_usd: can't be negative")
	}
	if s.PricePerSearch() < 0 {
		problems = append(problems, "search.price_per_search_usd: can't be negative")
	}
	if _, ok := c.LLM.Pricing.Models[s.Model]; !ok {
		problems = append(problems, fmt.Sprintf("llm.pricing.models has no prices for the search model %q; add them so search costs can be estimated and capped", s.Model))
	}
	for _, list := range []struct {
		name    string
		domains []string
	}{{"search.blocked_domains", s.BlockedDomains}, {"search.link_domains", s.LinkDomains}} {
		for _, d := range list.domains {
			if d == "" || strings.ContainsAny(d, "/:@ ") || !strings.Contains(d, ".") {
				problems = append(problems, fmt.Sprintf("%s: %q should be a bare domain like example.com (no https://, path or port)", list.name, d))
			}
		}
	}
	return problems
}

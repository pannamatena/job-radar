package config

import (
	"os"
	"strings"
	"testing"
)

// exampleWithSearchOn loads the shipped example and switches search on.
func exampleWithSearchOn(t *testing.T) *Config {
	t.Helper()
	data, err := os.ReadFile("../../examples/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("example config: %v", err)
	}
	cfg.Search.Enabled = true
	return cfg
}

func TestSearchExampleIsValidWhenSwitchedOn(t *testing.T) {
	cfg := exampleWithSearchOn(t)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("example search block should be valid: %v", err)
	}
	if got := len(cfg.Search.ActiveQueries()); got != 6 {
		t.Errorf("manager and lead_ic have 3 queries each, got %d active", got)
	}
}

func TestSearchIsOffByDefaultAndDefaultsApply(t *testing.T) {
	cfg, err := Parse([]byte("scorer: rules\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Search.Enabled {
		t.Error("search costs money and must be off by default")
	}
	if cfg.Search.Provider != "anthropic" || cfg.Search.PricePerSearch() != 0.01 {
		t.Errorf("provider and per-search price should default to anthropic and $0.01: %+v", cfg.Search)
	}
	if cfg.Search.Model != "claude-haiku-5-5" || cfg.Search.MaxSearchesPerQuery != 2 ||
		cfg.Search.MonthlySpendCapUSD != 5 || len(cfg.Search.LinkDomains) != 1 || cfg.Search.LinkDomains[0] != "linkedin.com" {
		t.Errorf("unexpected defaults: %+v", cfg.Search)
	}
}

func TestSearchOnlyActiveTracksAreSearched(t *testing.T) {
	cfg := exampleWithSearchOn(t)
	cfg.Search.Tracks = []string{"manager"}
	cfg.Search.Queries["tpm"] = []string{"never run"}
	if got := cfg.Search.ActiveQueries(); len(got) != 3 {
		t.Errorf("only the manager queries should run, got %v", got)
	}
}

func TestSearchValidationReportsProblems(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"no tracks", func(c *Config) { c.Search.Tracks = nil }, "search.tracks: list at least one"},
		{"unknown track", func(c *Config) { c.Search.Tracks = []string{"nope"} }, `"nope" is not a track`},
		{"track without queries", func(c *Config) { c.Search.Tracks = []string{"tpm"} }, "has no queries"},
		{"empty query", func(c *Config) { c.Search.Queries["manager"] = []string{"  "} }, "can't be empty"},
		{"query too long", func(c *Config) { c.Search.Queries["manager"] = []string{strings.Repeat("x", 301)} }, "at most 300"},
		{"negative per run", func(c *Config) { c.Search.QueriesPerRun = -1 }, "queries_per_run"},
		{"too many searches", func(c *Config) { c.Search.MaxSearchesPerQuery = 9 }, "max_searches_per_query"},
		{"model without a price", func(c *Config) { c.Search.Model = "no-such-model" }, "no prices for the search model"},
		{"blocked domain with a scheme", func(c *Config) { c.Search.BlockedDomains = []string{"https://example.com"} }, "bare domain"},
		{"link domain with a path", func(c *Config) { c.Search.LinkDomains = []string{"example.com/jobs"} }, "bare domain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := exampleWithSearchOn(t)
			tt.mutate(cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("want an error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestSearchIsNotValidatedWhenOff(t *testing.T) {
	cfg := exampleWithSearchOn(t)
	cfg.Search.Enabled = false
	cfg.Search.Tracks = []string{"nope"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("a switched-off search block must not produce errors: %v", err)
	}
}

func TestSearchProviderWithNoPerSearchChargeAndNegativePrice(t *testing.T) {
	cfg, err := Parse([]byte("search:\n  provider: other\n  price_per_search_usd: 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Search.Provider != "other" || cfg.Search.PricePerSearch() != 0 {
		t.Errorf("an explicit 0 must stay 0, got provider=%q price=%v", cfg.Search.Provider, cfg.Search.PricePerSearch())
	}
	bad := exampleWithSearchOn(t)
	neg := -1.0
	bad.Search.PricePerSearchUSD = &neg
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "price_per_search_usd") {
		t.Errorf("a negative price should be rejected, got %v", err)
	}
}

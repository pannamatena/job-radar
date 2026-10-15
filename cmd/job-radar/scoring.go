package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/agent"
	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/llm"
	"github.com/pannamatena/job-radar/internal/scorer"
	"github.com/pannamatena/job-radar/internal/store"
)

// Files in the config directory the AI scorer reads.
const (
	profileFile = "profile.md"
	rubricFile  = "rubric.md"
)

// scorerFlagValue resolves a --scorer override against the config.
func scorerFlagValue(cfg *config.Config, override string) (string, error) {
	switch override {
	case "":
		return cfg.Scorer, nil
	case config.ScorerRules, config.ScorerLLM:
		if override == config.ScorerLLM && cfg.Scorer != config.ScorerLLM {
			// The llm block is only validated when scorer is llm; check it now.
			cfg.Scorer = config.ScorerLLM
			if err := cfg.Validate(); err != nil {
				return "", err
			}
		}
		return override, nil
	}
	return "", fmt.Errorf("--scorer must be %q or %q", config.ScorerRules, config.ScorerLLM)
}

// newAgent builds the AI scoring agent: API client, profile and rubric.
func newAgent(cfg *config.Config) (*agent.Agent, error) {
	a, _, err := newAgentAndClient(cfg)
	return a, err
}

// newAgentAndClient is newAgent, but also returns the API client so the batch
// eval can send batches with it.
func newAgentAndClient(cfg *config.Config) (*agent.Agent, *llm.Anthropic, error) {
	client, err := llm.NewAnthropic()
	if err != nil {
		return nil, nil, err
	}
	profile, err := readConfigText(cfg.Home, profileFile)
	if err != nil {
		return nil, nil, err
	}
	rubric, err := readConfigText(cfg.Home, rubricFile)
	if err != nil {
		return nil, nil, err
	}
	for _, w := range agent.CheckRubric(rubric, cfg) {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if cfg.LLM.PricingStale(time.Now()) {
		fmt.Fprintf(os.Stderr, "warning: llm.pricing was last checked %q (over 90 days ago or never); cost estimates and the spend cap may be off. Compare with %s and update last_checked.\n",
			cfg.LLM.Pricing.LastChecked, cfg.LLM.Pricing.Source)
	}
	return agent.New(agent.Options{Client: client, Config: cfg, Profile: profile, Rubric: rubric}), client, nil
}

// newGatedScorer builds the scorer for `scorer: llm`: rules first, then the AI
// within the caps, reusing stored AI scores.
func newGatedScorer(ctx context.Context, cfg *config.Config, st *store.Store) (*agent.Gated, error) {
	a, err := newAgent(cfg)
	if err != nil {
		return nil, err
	}
	spent, err := st.AISpendSince(ctx, store.StartOfMonth(time.Now()))
	if err != nil {
		return nil, err
	}
	return &agent.Gated{
		Rules:             scorer.NewRules(cfg.Rules, cfg.Location),
		AI:                a,
		Cache:             st.CachedAIScore,
		MinRulesScore:     cfg.LLM.MinRulesScore,
		MaxPerRun:         cfg.LLM.MaxPostingsPerRun,
		MonthlyCapUSD:     cfg.LLM.MonthlySpendCapUSD,
		SpentThisMonthUSD: spent,
	}, nil
}

// readConfigText reads profile.md / rubric.md with a helpful error.
func readConfigText(home, name string) (string, error) {
	path := filepath.Join(home, name)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("the AI scorer needs %s — see docs/writing-your-profile.md (examples/%s shows the format)", path, name)
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(b)) == "" {
		return "", fmt.Errorf("%s is empty — the AI scorer can only judge fit against what's written there", path)
	}
	return string(b), nil
}

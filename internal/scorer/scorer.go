// Package scorer turns a Posting into a Result: a 1–5 score with the track it
// fits, the work model, a short honest "why" and "gap", and any flags. Two
// implementations will share this shape (ADR-0006): the free rules scorer here,
// and the optional LLM scorer in phase 3. Because they return the same Result,
// storage, notifications, the digest and the eval don't care which one ran.
package scorer

import (
	"context"

	"github.com/pannamatena/job-radar/internal/posting"
)

// Result is the outcome of scoring one posting. The same shape is produced by
// every scorer.
type Result struct {
	Score     int      // 1–5
	Track     string   // matched track name, or "none"
	WorkModel string   // "remote" | "hybrid" | "onsite" | "not_stated"
	Office    string   // best-known office/location, may be empty
	Why       string   // short, honest reason it fits
	Gap       string   // short, honest biggest gap
	Flags     []string // built-in + user-configured flags that apply
	Breakdown []string // human-readable score steps, shown by `score-one`
	Scorer    string   // which scorer produced this: "rules" or "llm"

	// AI is filled in when the AI scorer was involved (scored it, tried and
	// fell back, or skipped it because of a cap); nil for a plain rules score.
	AI *AIDetails
}

// ScorerLLM is the name recorded on results from the AI scorer.
const ScorerLLM = "llm"

// AI scoring outcomes, recorded on AIDetails.Status.
const (
	AIScored     = "scored"      // the AI scored it; Result holds the AI's score
	AICached     = "cached"      // reused an earlier AI score for the same ad and prompt
	AINotSent    = "not_sent"    // rules score below the gate; kept the rules score
	AISkippedCap = "skipped_cap" // per-run or monthly cap reached; kept the rules score
	AIUnscored   = "unscored"    // turn/token limit hit or API error; kept the rules score
)

// AIDetails records what the AI scorer did and cost for one posting. Plain
// numbers only, so this package doesn't depend on the llm package.
type AIDetails struct {
	Status           string
	Note             string // short human explanation (e.g. why it fell back)
	Model            string
	PromptVersion    string
	Turns            int
	InputTokens      int64
	OutputTokens     int64
	CacheWriteTokens int64
	CacheReadTokens  int64
	CostUSD          float64
	SchemaFailures   int      // invalid record_score calls the model had to retry
	Transcript       []string // tool calls and results, for `score-one`
}

// Scorer scores a single posting. The rules scorer never returns an error, but
// the interface carries ctx and error so the LLM scorer (phase 3) fits the same
// shape without a rewrite.
type Scorer interface {
	Score(ctx context.Context, p posting.Posting) (Result, error)
}

// Built-in flag names (a small generic set; users add their own via red_flags).
const (
	// FlagOutsideHomeArea: a hybrid/onsite role whose office isn't in a home city.
	FlagOutsideHomeArea = "outside_home_area"
	// FlagOutsideEligibleRegion: a remote role open to a region the user isn't
	// (e.g. a US-only remote role for someone who can only work in Europe).
	FlagOutsideEligibleRegion = "outside_eligible_region"
	// FlagWorkModelNotAccepted: a work model the user has ruled out entirely
	// (e.g. onsite: never in allowed_work_models).
	FlagWorkModelNotAccepted = "work_model_not_accepted"
)

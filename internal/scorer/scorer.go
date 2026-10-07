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
	Scorer    string   // which scorer produced this ("rules", later "llm")
}

// Scorer scores a single posting. The rules scorer never returns an error, but
// the interface carries ctx and error so the LLM scorer (phase 3) fits the same
// shape without a rewrite.
type Scorer interface {
	Score(ctx context.Context, p posting.Posting) (Result, error)
}

// Built-in flag names (a small generic set; users add their own via red_flags).
const (
	FlagOutsideHomeArea = "outside_home_area"
)

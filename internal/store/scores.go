package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

// ContentHash identifies the version of an ad that was scored. If the company
// edits the ad, the hash changes and a cached AI score no longer applies.
func ContentHash(p posting.Posting) string {
	h := sha256.New()
	for _, s := range []string{p.Title, p.Location, string(p.Remote), p.DescriptionText} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// SaveScore appends a score row. duplicateOf is the canonical posting id when
// this posting was matched to another source's copy, otherwise "".
func (s *Store) SaveScore(ctx context.Context, p posting.Posting, r scorer.Result, duplicateOf string, now time.Time) error {
	flags, err := json.Marshal(nonNil(r.Flags))
	if err != nil {
		return err
	}
	ai := r.AI
	if ai == nil {
		ai = &scorer.AIDetails{}
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO scores (posting_id, scored_at, scorer, ai_status, ai_note, model, prompt_version, content_hash,
		  score, track, work_model, office, why, gap, flags,
		  turns, input_tokens, output_tokens, cache_write_tokens, cache_read_tokens, cost_usd, schema_failures, duplicate_of)
		VALUES (?,?,?,?,?,?,?,?, ?,?,?,?,?,?,?, ?,?,?,?,?,?,?,?)`,
		p.ID, now.UTC().Format(time.RFC3339), r.Scorer, nullIfEmpty(ai.Status), nullIfEmpty(ai.Note),
		nullIfEmpty(ai.Model), nullIfEmpty(ai.PromptVersion), ContentHash(p),
		r.Score, r.Track, r.WorkModel, nullIfEmpty(r.Office), r.Why, r.Gap, string(flags),
		ai.Turns, ai.InputTokens, ai.OutputTokens, ai.CacheWriteTokens, ai.CacheReadTokens, ai.CostUSD, ai.SchemaFailures,
		nullIfEmpty(duplicateOf))
	if err != nil {
		return fmt.Errorf("store: saving score for %s: %w", p.ID, err)
	}
	return nil
}

// CachedAIScore returns the latest successful AI score for exactly this ad
// content, model and prompt version, so an unchanged posting isn't paid for
// again on the next run.
func (s *Store) CachedAIScore(ctx context.Context, p posting.Posting, model, promptVersion string) (scorer.Result, bool, error) {
	var r scorer.Result
	var office sql.NullString
	var flags string
	err := s.db.QueryRowContext(ctx, `
		SELECT score, track, work_model, office, why, gap, flags
		FROM scores
		WHERE posting_id = ? AND scorer = 'llm' AND ai_status IN ('scored', 'cached')
		  AND model = ? AND prompt_version = ? AND content_hash = ?
		ORDER BY id DESC LIMIT 1`,
		p.ID, model, promptVersion, ContentHash(p)).
		Scan(&r.Score, &r.Track, &r.WorkModel, &office, &r.Why, &r.Gap, &flags)
	if err == sql.ErrNoRows {
		return scorer.Result{}, false, nil
	}
	if err != nil {
		return scorer.Result{}, false, fmt.Errorf("store: cached score for %s: %w", p.ID, err)
	}
	if err := json.Unmarshal([]byte(flags), &r.Flags); err != nil {
		return scorer.Result{}, false, fmt.Errorf("store: flags for %s: %w", p.ID, err)
	}
	r.Office = office.String
	r.Scorer = scorer.ScorerLLM
	r.Breakdown = []string{"AI score reused from an earlier run (same ad, model and prompt) — no new cost"}
	r.AI = &scorer.AIDetails{Status: scorer.AICached, Model: model, PromptVersion: promptVersion}
	return r, true, nil
}

// AISpendSince sums the estimated AI cost of every score since t (used for the
// monthly spend cap).
func (s *Store) AISpendSince(ctx context.Context, t time.Time) (float64, error) {
	var total sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `SELECT SUM(cost_usd) FROM scores WHERE scored_at >= ?`,
		t.UTC().Format(time.RFC3339)).Scan(&total); err != nil {
		return 0, fmt.Errorf("store: summing AI spend: %w", err)
	}
	return total.Float64, nil
}

// StartOfMonth is midnight UTC on the first day of t's month.
func StartOfMonth(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

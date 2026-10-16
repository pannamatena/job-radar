package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/posting"
)

// ScoredPosting is a stored posting with its latest score, for the digest.
type ScoredPosting struct {
	Posting   posting.Posting
	Score     int
	Track     string
	WorkModel string
	Why       string
	Gap       string
	Scorer    string
	Flags     []string
	FirstSeen time.Time
}

const scoredPostingSelect = `
	SELECT p.id, p.source, p.company, p.title, p.location, p.remote, p.url, p.first_seen_at,
	       s.score, s.track, s.work_model, s.why, s.gap, s.scorer, s.flags
	FROM postings p
	JOIN scores s ON s.id = (SELECT MAX(id) FROM scores WHERE posting_id = p.id)
	WHERE s.duplicate_of IS NULL AND s.score >= ?`

// NewScored returns postings first seen at or after since whose latest score is
// at least minScore, best first. These are the "new" part of the digest.
func (s *Store) NewScored(ctx context.Context, since time.Time, minScore int) ([]ScoredPosting, error) {
	return s.queryScored(ctx, scoredPostingSelect+` AND p.first_seen_at >= ? ORDER BY s.score DESC, p.first_seen_at DESC`,
		minScore, since.UTC().Format(time.RFC3339))
}

// StillOpen returns postings first seen before olderThan that were still on the
// company's board at or after seenSince, with a latest score of at least
// minScore. Long-open roles are easy to miss, so the Monday digest lists them.
func (s *Store) StillOpen(ctx context.Context, olderThan, seenSince time.Time, minScore int) ([]ScoredPosting, error) {
	return s.queryScored(ctx, scoredPostingSelect+` AND p.first_seen_at < ? AND p.last_seen_at >= ? ORDER BY s.score DESC, p.first_seen_at ASC`,
		minScore, olderThan.UTC().Format(time.RFC3339), seenSince.UTC().Format(time.RFC3339))
}

// ScoredBetween returns postings first seen at or after since whose latest
// score is between minScore and maxScore (inclusive), highest score first,
// at most limit. It is how the digest finds near-misses and `list --low`
// finds jobs scored lower than you may agree with.
func (s *Store) ScoredBetween(ctx context.Context, since time.Time, minScore, maxScore, limit int) ([]ScoredPosting, error) {
	return s.queryScored(ctx, scoredPostingSelect+` AND s.score <= ? AND p.first_seen_at >= ? ORDER BY s.score DESC, p.first_seen_at DESC LIMIT ?`,
		minScore, maxScore, since.UTC().Format(time.RFC3339), limit)
}

// LatestScore is a posting's most recent score, if it has one.
func (s *Store) LatestScore(ctx context.Context, postingID string) (int, bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT score FROM scores WHERE posting_id = ? ORDER BY id DESC LIMIT 1`, postingID).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("store: latest score: %w", err)
	}
	return n, true, nil
}

func (s *Store) queryScored(ctx context.Context, q string, args ...any) ([]ScoredPosting, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: digest query: %w", err)
	}
	defer rows.Close()
	var out []ScoredPosting
	for rows.Next() {
		var sp ScoredPosting
		var remote, first, flags string
		if err := rows.Scan(&sp.Posting.ID, &sp.Posting.Source, &sp.Posting.Company, &sp.Posting.Title, &sp.Posting.Location,
			&remote, &sp.Posting.URL, &first, &sp.Score, &sp.Track, &sp.WorkModel, &sp.Why, &sp.Gap, &sp.Scorer, &flags); err != nil {
			return nil, fmt.Errorf("store: digest scan: %w", err)
		}
		sp.Posting.Remote = posting.RemoteKind(strings.TrimSpace(remote))
		sp.FirstSeen, _ = time.Parse(time.RFC3339, first)
		if flags != "" {
			_ = json.Unmarshal([]byte(flags), &sp.Flags)
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

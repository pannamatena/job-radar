package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Vote values.
const (
	VoteUp   = "up"
	VoteDown = "down"
)

// Feedback is one recorded vote.
type Feedback struct {
	PostingID      string
	Vote           string
	Source         string // email | terminal
	NotificationID int64  // 0 when not from an email
	ReplyID        string
	At             time.Time
}

// SaveFeedback records a vote. Reading the same reply twice is harmless: a vote
// from the same reply for the same posting is stored once. It reports whether a
// new row was added.
func (s *Store) SaveFeedback(ctx context.Context, f Feedback) (bool, error) {
	var notif any
	if f.NotificationID != 0 {
		notif = f.NotificationID
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO feedback (posting_id, vote, source, notification_id, reply_id, created_at)
		VALUES (?,?,?,?,?,?)`,
		f.PostingID, f.Vote, f.Source, notif, nullIfEmpty(f.ReplyID), f.At.UTC().Format(time.RFC3339))
	if err != nil {
		return false, fmt.Errorf("store: saving feedback: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// LatestVote is the most recent vote on a posting, if there is one.
func (s *Store) LatestVote(ctx context.Context, postingID string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT vote FROM feedback WHERE posting_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, postingID).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("store: latest vote: %w", err)
	}
	return v, true, nil
}

// RecentFeedback lists votes since the given time, newest first, with the
// posting's company and title.
type FeedbackRow struct {
	Feedback
	Company, Title string
}

func (s *Store) RecentFeedback(ctx context.Context, since time.Time) ([]FeedbackRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.posting_id, f.vote, f.source, f.created_at, p.company, p.title
		FROM feedback f JOIN postings p ON p.id = f.posting_id
		WHERE f.created_at >= ? ORDER BY f.created_at DESC, f.id DESC`, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("store: feedback query: %w", err)
	}
	defer rows.Close()
	var out []FeedbackRow
	for rows.Next() {
		var r FeedbackRow
		var at string
		if err := rows.Scan(&r.PostingID, &r.Vote, &r.Source, &at, &r.Company, &r.Title); err != nil {
			return nil, err
		}
		r.At, _ = time.Parse(time.RFC3339, at)
		out = append(out, r)
	}
	return out, rows.Err()
}

// NotificationByMessageID finds a sent email by its Message-ID header, and the
// posting shown as each number in it (index n-1 = number n).
func (s *Store) NotificationByMessageID(ctx context.Context, messageID string) (int64, []string, bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM notifications WHERE message_id = ?`, messageID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, fmt.Errorf("store: notification lookup: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT posting_id FROM notification_items WHERE notification_id = ? ORDER BY number`, id)
	if err != nil {
		return 0, nil, false, fmt.Errorf("store: notification items: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			return 0, nil, false, err
		}
		ids = append(ids, pid)
	}
	return id, ids, true, rows.Err()
}

// RecordSourceResult updates a company's failure streak and returns the new
// count of consecutive failed runs (0 after a success).
func (s *Store) RecordSourceResult(ctx context.Context, company string, failed bool, errText string, now time.Time) (int, error) {
	ts := now.UTC().Format(time.RFC3339)
	if !failed {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO source_status (company, consecutive_failures, last_error, updated_at) VALUES (?,0,NULL,?)
			ON CONFLICT(company) DO UPDATE SET consecutive_failures = 0, last_error = NULL, updated_at = excluded.updated_at`, company, ts)
		return 0, err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO source_status (company, consecutive_failures, last_error, updated_at) VALUES (?,1,?,?)
		ON CONFLICT(company) DO UPDATE SET consecutive_failures = consecutive_failures + 1, last_error = excluded.last_error, updated_at = excluded.updated_at`,
		company, errText, ts)
	if err != nil {
		return 0, err
	}
	var n int
	err = s.db.QueryRowContext(ctx, `SELECT consecutive_failures FROM source_status WHERE company = ?`, company).Scan(&n)
	return n, err
}

// Summary is the numbers behind `job-radar report`.
type Summary struct {
	Runs          int
	NewPostings   int
	Scored        int
	AICostUSD     float64
	EmailsByKind  map[string]int
	StrongMatches int // distinct postings listed in alert emails
}

// Summarise totals the activity since the given time.
func (s *Store) Summarise(ctx context.Context, since time.Time) (Summary, error) {
	ts := since.UTC().Format(time.RFC3339)
	sum := Summary{EmailsByKind: map[string]int{}}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(new_postings),0), COALESCE(SUM(scored),0), COALESCE(SUM(cost_estimate),0)
		FROM runs WHERE started_at >= ?`, ts).Scan(&sum.Runs, &sum.NewPostings, &sum.Scored, &sum.AICostUSD); err != nil {
		return sum, fmt.Errorf("store: summary: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT kind, COUNT(*) FROM notifications WHERE sent_at >= ? GROUP BY kind`, ts)
	if err != nil {
		return sum, fmt.Errorf("store: summary: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return sum, err
		}
		sum.EmailsByKind[kind] = n
	}
	if err := rows.Err(); err != nil {
		return sum, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT i.posting_id) FROM notification_items i
		JOIN notifications n ON n.id = i.notification_id
		WHERE n.kind = ? AND n.sent_at >= ?`, KindAlert, ts).Scan(&sum.StrongMatches); err != nil {
		return sum, fmt.Errorf("store: summary: %w", err)
	}
	return sum, nil
}

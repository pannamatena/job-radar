package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Notification kinds.
const (
	KindAlert   = "alert"
	KindDigest  = "digest"
	KindProblem = "problem"
)

// Notification is one email that was sent.
type Notification struct {
	Channel    string
	Kind       string
	SentAt     time.Time
	Subject    string
	MessageID  string
	Recipients []string
	// PostingIDs lists the jobs in the email; the job shown as number n is
	// PostingIDs[n-1].
	PostingIDs []string
}

// RecordNotification stores a sent email and its numbered jobs in one
// transaction.
func (s *Store) RecordNotification(ctx context.Context, n Notification) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("store: begin: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO notifications (channel, kind, sent_at, subject, message_id, recipients)
		VALUES (?,?,?,?,?,?)`,
		n.Channel, n.Kind, n.SentAt.UTC().Format(time.RFC3339), n.Subject,
		nullIfEmpty(n.MessageID), strings.Join(n.Recipients, ","))
	if err != nil {
		return 0, fmt.Errorf("store: insert notification: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for i, pid := range n.PostingIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO notification_items (notification_id, number, posting_id) VALUES (?,?,?)`,
			id, i+1, pid); err != nil {
			return 0, fmt.Errorf("store: insert notification item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("store: commit: %w", err)
	}
	return id, nil
}

// AlreadyAlerted returns which of the given posting ids have already been
// listed in an alert email on the channel, so none is sent twice.
func (s *Store) AlreadyAlerted(ctx context.Context, channel string, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, id := range ids {
		var n int
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM notification_items i
			JOIN notifications n ON n.id = i.notification_id
			WHERE n.channel = ? AND n.kind = ? AND i.posting_id = ?`,
			channel, KindAlert, id).Scan(&n)
		if err != nil {
			return nil, fmt.Errorf("store: alerted lookup: %w", err)
		}
		if n > 0 {
			out[id] = true
		}
	}
	return out, nil
}

// LastNotification returns when the last email of this kind was sent on the
// channel, and false if none was.
func (s *Store) LastNotification(ctx context.Context, channel, kind string) (time.Time, bool, error) {
	var ts string
	err := s.db.QueryRowContext(ctx, `
		SELECT sent_at FROM notifications WHERE channel = ? AND kind = ?
		ORDER BY sent_at DESC LIMIT 1`, channel, kind).Scan(&ts)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("store: last notification: %w", err)
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("store: bad sent_at %q: %w", ts, err)
	}
	return t, true, nil
}

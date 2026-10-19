package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SearchRun is one run's web-search activity.
type SearchRun struct {
	At       time.Time
	Queries  int
	Searches int
	CostUSD  float64
}

// RecordSearchRun stores what a run's searching used. It is recorded even in a
// dry run, because the money is spent either way.
func (s *Store) RecordSearchRun(ctx context.Context, r SearchRun) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO search_runs (ran_at, queries, searches, cost_usd) VALUES (?, ?, ?, ?)`,
		r.At.UTC().Format(time.RFC3339), r.Queries, r.Searches, r.CostUSD)
	if err != nil {
		return fmt.Errorf("store: recording search run: %w", err)
	}
	return nil
}

// SearchSpendSince is the estimated search spend since t.
func (s *Store) SearchSpendSince(ctx context.Context, t time.Time) (float64, error) {
	var total sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `SELECT SUM(cost_usd) FROM search_runs WHERE ran_at >= ?`,
		t.UTC().Format(time.RFC3339)).Scan(&total); err != nil {
		return 0, fmt.Errorf("store: summing search spend: %w", err)
	}
	return total.Float64, nil
}

// SearchQueriesRun is how many queries have been run in total. The next run
// starts that far into the (rotating) query list.
func (s *Store) SearchQueriesRun(ctx context.Context) (int, error) {
	var n sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT SUM(queries) FROM search_runs`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: counting search queries: %w", err)
	}
	return int(n.Int64), nil
}

// Lead is a search result that is not a posting on a known job board.
type Lead struct {
	URL       string
	Host      string
	Title     string
	Known     bool // a well-known host, shown as a normal link
	FirstSeen time.Time
}

// SaveLead stores a lead once. It returns true if it is new.
func (s *Store) SaveLead(ctx context.Context, l Lead, now time.Time) (bool, error) {
	known := 0
	if l.Known {
		known = 1
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO leads (url, host, title, known, first_seen_at) VALUES (?, ?, ?, ?, ?)`,
		l.URL, l.Host, l.Title, known, now.UTC().Format(time.RFC3339))
	if err != nil {
		return false, fmt.Errorf("store: saving lead: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// LeadsSince returns leads first seen at or after since, oldest first.
func (s *Store) LeadsSince(ctx context.Context, since time.Time) ([]Lead, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT url, host, title, known, first_seen_at FROM leads WHERE first_seen_at >= ? ORDER BY first_seen_at, url`,
		since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("store: listing leads: %w", err)
	}
	defer rows.Close()
	var out []Lead
	for rows.Next() {
		var l Lead
		var known int
		var first string
		if err := rows.Scan(&l.URL, &l.Host, &l.Title, &known, &first); err != nil {
			return nil, fmt.Errorf("store: scanning lead: %w", err)
		}
		l.Known = known == 1
		l.FirstSeen, _ = time.Parse(time.RFC3339, first)
		out = append(out, l)
	}
	return out, rows.Err()
}

// NewLeadSites returns hosts whose first lead was seen at or after since: sites
// the user has not been told about before.
func (s *Store) NewLeadSites(ctx context.Context, since time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT host FROM leads GROUP BY host HAVING MIN(first_seen_at) >= ? ORDER BY host`,
		since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("store: listing new sites: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, fmt.Errorf("store: scanning site: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

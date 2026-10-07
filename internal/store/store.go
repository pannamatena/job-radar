// Package store is job-radar's memory: a single SQLite database file that
// persists between runs, so the tool knows which postings it has already seen
// (dedupe) and can log each run. SQLite is used through the pure-Go
// modernc.org/sqlite driver — no C compiler needed, so it cross-compiles and
// runs in CI with nothing to install (ADR-0016). The schema is created and
// upgraded by hand-written, numbered SQL migrations embedded in the binary
// (ADR-0017).
//
// Go note: the standard library's database/sql package is the common interface
// to every SQL database; importing the driver with a blank name
// (_ "modernc.org/sqlite") registers it so sql.Open("sqlite", …) works. We
// write plain SQL with ? placeholders rather than using an ORM.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pannamatena/job-radar/internal/posting"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store wraps the database handle.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens (creating if needed) the SQLite database at path and brings its
// schema up to date, backing the file up to <path>.bak before applying any new
// migration.
func Open(path string) (*Store, error) {
	// busy_timeout makes concurrent access wait briefly rather than erroring.
	dsn := path + "?_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: opening %s: %w", path, err)
	}
	// SQLite allows only one writer; serialise access to avoid "database is
	// locked" errors in this single-process tool.
	db.SetMaxOpenConns(1)

	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// migration is one embedded SQL file.
type migration struct {
	version int
	name    string
	sql     string
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("store: creating schema_migrations: %w", err)
	}

	current, err := s.currentVersion()
	if err != nil {
		return err
	}

	all, err := loadMigrations()
	if err != nil {
		return err
	}

	var pending []migration
	for _, m := range all {
		if m.version > current {
			pending = append(pending, m)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	// Back up an existing, non-empty database before changing its schema.
	if err := s.backup(); err != nil {
		return err
	}

	for _, m := range pending {
		if err := s.applyMigration(m); err != nil {
			return fmt.Errorf("store: applying migration %04d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

func (s *Store) currentVersion() (int, error) {
	var v sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, fmt.Errorf("store: reading schema version: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

// applyMigration runs one migration and records it, all in a transaction so a
// failure leaves the schema untouched.
func (s *Store) applyMigration(m migration) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after a successful Commit

	if _, err := tx.Exec(m.sql); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		m.version, nowRFC3339()); err != nil {
		return err
	}
	return tx.Commit()
}

// backup copies the database file to <path>.bak if it exists and is non-empty.
func (s *Store) backup() error {
	info, err := os.Stat(s.path)
	if err != nil || info.Size() == 0 {
		return nil // nothing to back up (fresh database)
	}
	src, err := os.Open(s.path)
	if err != nil {
		return fmt.Errorf("store: opening db for backup: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(s.path + ".bak")
	if err != nil {
		return fmt.Errorf("store: creating backup: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("store: writing backup: %w", err)
	}
	return nil
}

// loadMigrations reads and sorts the embedded migration files.
func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("store: reading migrations: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// Filenames look like "0001_init.sql"; the numeric prefix is the version.
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("store: migration %q must be named <version>_<name>.sql", e.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("store: migration %q has a non-numeric version prefix", e.Name())
		}
		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: version, name: e.Name(), sql: string(data)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// SaveFetched records the postings seen in a run. For each posting it either
// inserts a new row (and reports the id as "new to the user", ADR-0025) or
// refreshes the existing row's mutable fields and last_seen_at. It returns the
// ids that were new, in the order given. The whole batch runs in one
// transaction.
func (s *Store) SaveFetched(ctx context.Context, postings []posting.Posting, now time.Time) ([]string, error) {
	ts := now.UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("store: begin: %w", err)
	}
	defer tx.Rollback()

	var newIDs []string
	for _, p := range postings {
		var existing string
		err := tx.QueryRowContext(ctx, `SELECT id FROM postings WHERE id = ?`, p.ID).Scan(&existing)
		switch {
		case err == sql.ErrNoRows:
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO postings
				  (id, source, company, title, location, remote, url, description_text, posted_at, first_seen_at, last_seen_at, raw_json)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
				p.ID, p.Source, p.Company, p.Title, p.Location, string(p.Remote), p.URL,
				p.DescriptionText, postedAt(p), ts, ts, string(p.RawJSON)); err != nil {
				return nil, fmt.Errorf("store: insert %s: %w", p.ID, err)
			}
			newIDs = append(newIDs, p.ID)
		case err != nil:
			return nil, fmt.Errorf("store: lookup %s: %w", p.ID, err)
		default:
			// Already seen: refresh fields that can change and the last_seen time.
			if _, err := tx.ExecContext(ctx, `
				UPDATE postings SET
				  title = ?, location = ?, remote = ?, url = ?, description_text = ?, posted_at = ?, last_seen_at = ?, raw_json = ?
				WHERE id = ?`,
				p.Title, p.Location, string(p.Remote), p.URL, p.DescriptionText, postedAt(p), ts, string(p.RawJSON), p.ID); err != nil {
				return nil, fmt.Errorf("store: update %s: %w", p.ID, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: commit: %w", err)
	}
	return newIDs, nil
}

// CountCompanyPostings returns how many postings we have ever recorded for a
// company. Zero means this is the first time we are seeing that company, which
// later phases use to send a first-run batch summary instead of many alerts.
func (s *Store) CountCompanyPostings(ctx context.Context, company string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM postings WHERE company = ?`, company).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: counting postings for %q: %w", company, err)
	}
	return n, nil
}

// RunStats is what we record about a run.
type RunStats struct {
	StartedAt    time.Time
	FinishedAt   time.Time
	NewPostings  int
	Scored       int
	CostEstimate float64
	Errors       string
}

// RecordRun writes a row to the runs log and returns its id.
func (s *Store) RecordRun(ctx context.Context, r RunStats) (int64, error) {
	var finished any
	if !r.FinishedAt.IsZero() {
		finished = r.FinishedAt.UTC().Format(time.RFC3339)
	}
	var errs any
	if r.Errors != "" {
		errs = r.Errors
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (started_at, finished_at, new_postings, scored, cost_estimate, errors)
		VALUES (?,?,?,?,?,?)`,
		r.StartedAt.UTC().Format(time.RFC3339), finished, r.NewPostings, r.Scored, r.CostEstimate, errs)
	if err != nil {
		return 0, fmt.Errorf("store: recording run: %w", err)
	}
	return res.LastInsertId()
}

func postedAt(p posting.Posting) any {
	if p.PostedAt == nil {
		return nil
	}
	return p.PostedAt.UTC().Format(time.RFC3339)
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

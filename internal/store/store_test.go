package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/posting"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "radar.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func mkPosting(id, title string) posting.Posting {
	return posting.Posting{
		ID: id, Source: "greenhouse", Company: "Acme", Title: title,
		Location: "Manchester", Remote: posting.RemoteUnknown, URL: "http://x/" + id,
	}
}

func TestMigrationsRunAndAreIdempotent(t *testing.T) {
	_, path := openTemp(t)
	// Re-open the same file: migrations must not re-run or error.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer s2.Close()
	v, err := s2.currentVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Errorf("schema version = %d, want 1", v)
	}
}

func TestSaveFetched_DedupeAcrossRuns(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	now := time.Now()

	// First run: two postings, both new.
	batch := []posting.Posting{mkPosting("greenhouse:a/1", "Role One"), mkPosting("greenhouse:a/2", "Role Two")}
	newIDs, err := s.SaveFetched(ctx, batch, now)
	if err != nil {
		t.Fatalf("SaveFetched run 1: %v", err)
	}
	if len(newIDs) != 2 {
		t.Fatalf("run 1 new = %d, want 2", len(newIDs))
	}

	// Second run: same two, plus one genuinely new. Only the new one is "new".
	batch2 := append(batch, mkPosting("greenhouse:a/3", "Role Three"))
	newIDs, err = s.SaveFetched(ctx, batch2, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("SaveFetched run 2: %v", err)
	}
	if len(newIDs) != 1 || newIDs[0] != "greenhouse:a/3" {
		t.Fatalf("run 2 new = %v, want [greenhouse:a/3]", newIDs)
	}

	// Third run: nothing new (the phase-2 acceptance criterion).
	newIDs, err = s.SaveFetched(ctx, batch2, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("SaveFetched run 3: %v", err)
	}
	if len(newIDs) != 0 {
		t.Fatalf("run 3 new = %v, want none", newIDs)
	}
}

func TestSaveFetched_FirstSeenIsStable(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

	p := mkPosting("greenhouse:a/1", "Original Title")
	if _, err := s.SaveFetched(ctx, []posting.Posting{p}, t0); err != nil {
		t.Fatal(err)
	}
	// Re-save later with a changed title.
	p.Title = "Updated Title"
	if _, err := s.SaveFetched(ctx, []posting.Posting{p}, t0.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	var first, last, title string
	err := s.db.QueryRow(`SELECT first_seen_at, last_seen_at, title FROM postings WHERE id = ?`, p.ID).
		Scan(&first, &last, &title)
	if err != nil {
		t.Fatal(err)
	}
	if first != t0.Format(time.RFC3339) {
		t.Errorf("first_seen_at = %q, want it unchanged at %q", first, t0.Format(time.RFC3339))
	}
	if last == first {
		t.Error("last_seen_at should advance on re-save")
	}
	if title != "Updated Title" {
		t.Errorf("mutable field not refreshed: title = %q", title)
	}
}

func TestCountCompanyPostings(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	if n, _ := s.CountCompanyPostings(ctx, "Acme"); n != 0 {
		t.Fatalf("new company count = %d, want 0", n)
	}
	_, _ = s.SaveFetched(ctx, []posting.Posting{mkPosting("greenhouse:a/1", "Role")}, time.Now())
	if n, _ := s.CountCompanyPostings(ctx, "Acme"); n != 1 {
		t.Fatalf("count after save = %d, want 1", n)
	}
}

func TestRecordRun(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	id, err := s.RecordRun(ctx, RunStats{
		StartedAt: time.Now(), FinishedAt: time.Now(), NewPostings: 3,
	})
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if id <= 0 {
		t.Errorf("run id = %d, want > 0", id)
	}
}

func TestBackup(t *testing.T) {
	s, path := openTemp(t)
	ctx := context.Background()
	if _, err := s.SaveFetched(ctx, []posting.Posting{mkPosting("greenhouse:a/1", "Role")}, time.Now()); err != nil {
		t.Fatal(err)
	}

	// A non-empty database is copied to <path>.bak.
	if err := s.backup(); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if info, err := os.Stat(path + ".bak"); err != nil || info.Size() == 0 {
		t.Fatalf("expected a non-empty %s.bak: err=%v", path, err)
	}

	// A missing database is a no-op (fresh install), not an error.
	missing := &Store{path: filepath.Join(t.TempDir(), "nope.db")}
	if err := missing.backup(); err != nil {
		t.Errorf("backup of a missing db should be a no-op, got: %v", err)
	}
	if _, err := os.Stat(missing.path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("no backup should be created for a missing db")
	}
}

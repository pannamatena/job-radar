package store

import (
	"context"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
)

func TestDigestQueries(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC) // a Monday
	old := mkPosting("greenhouse:a/old", "Old EM")
	fresh := mkPosting("greenhouse:a/new", "New EM")
	low := mkPosting("greenhouse:a/low", "Low")
	gone := mkPosting("greenhouse:a/gone", "Gone")

	// old and gone were first seen last week; old is still on the board.
	if _, err := s.SaveFetched(ctx, []posting.Posting{old, gone}, now.AddDate(0, 0, -7)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveFetched(ctx, []posting.Posting{old, fresh, low}, now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	score := func(p posting.Posting, n int, track string) {
		r := scorer.Result{Score: n, Track: track, WorkModel: "remote", Why: "w", Gap: "g", Scorer: scorer.ScorerRules}
		if err := s.SaveScore(ctx, p, r, "", now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	score(old, 4, "manager")
	score(gone, 4, "manager")
	score(fresh, 5, "lead_ic")
	score(low, 2, "manager")
	// A re-score replaces the earlier one: only the latest counts.
	score(fresh, 4, "lead_ic")

	got, err := s.NewScored(ctx, now.Add(-24*time.Hour), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Posting.ID != fresh.ID || got[0].Score != 4 || got[0].Track != "lead_ic" {
		t.Errorf("NewScored = %+v, want only the new posting at its latest score 4", got)
	}
	open, err := s.StillOpen(ctx, now.Add(-24*time.Hour), now.Add(-36*time.Hour), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].Posting.ID != old.ID {
		t.Errorf("StillOpen = %+v, want only the old posting that is still listed", open)
	}
}

package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
	"github.com/pannamatena/job-radar/internal/store"
)

func TestDigestDue(t *testing.T) {
	day := func(h, m int) time.Time { return time.Date(2026, 10, 17, h, m, 0, 0, time.Local) }
	cases := []struct {
		name string
		now  time.Time
		last time.Time
		have bool
		want bool
	}{
		{"before digest time", day(7, 59), time.Time{}, false, false},
		{"just after, never sent", day(8, 0), time.Time{}, false, true},
		{"afternoon, last sent yesterday", day(15, 0), day(8, 0).AddDate(0, 0, -1), true, true},
		{"already sent this morning", day(15, 0), day(8, 30), true, false},
		{"sent today before the set time", day(9, 0), day(7, 0), true, true},
	}
	for _, c := range cases {
		if got := digestDue(c.now, c.last, c.have, "08:00"); got != c.want {
			t.Errorf("%s: digestDue = %v, want %v", c.name, got, c.want)
		}
	}
	if digestDue(day(12, 0), time.Time{}, false, "not a time") {
		t.Error("a bad time should never be due")
	}
}

func TestTrackTitle(t *testing.T) {
	tracks := map[string]config.Track{"manager": {Label: "Manager roles"}, "lead_ic": {}, "tpm": {}, "staff_engineer": {}}
	for name, want := range map[string]string{
		"manager": "Manager roles", "lead_ic": "Lead IC", "tpm": "TPM", "staff_engineer": "Staff Engineer", "none": "Other", "": "Other",
	} {
		if got := trackTitle(name, tracks); got != want {
			t.Errorf("trackTitle(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestBuildDigest_GroupsByTrackAndAddsMondaySection(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "radar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := &config.Config{Rules: config.Rules{Thresholds: config.Thresholds{Alert: 4, Digest: 3},
		Tracks: map[string]config.Track{"manager": {Label: "Manager roles"}, "lead_ic": {}}}}

	// 2026-10-19 is a Monday; 2026-10-20 a Tuesday.
	monday := time.Date(2026, 10, 19, 9, 0, 0, 0, time.Local)
	mk := func(id, co, title string) posting.Posting {
		return posting.Posting{ID: id, Source: "greenhouse", Company: co, Title: title, URL: "https://example.com/" + id}
	}
	em, tl, old := mk("greenhouse:a/1", "Acme", "Engineering Manager"), mk("greenhouse:a/2", "Globex", "Tech Lead"), mk("greenhouse:a/3", "Initech", "Old EM")
	if _, err := st.SaveFetched(ctx, []posting.Posting{old}, monday.AddDate(0, 0, -10)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveFetched(ctx, []posting.Posting{old, em, tl}, monday.Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	for p, sc := range map[string]struct {
		post  posting.Posting
		score int
		track string
	}{"1": {em, 5, "manager"}, "2": {tl, 3, "lead_ic"}, "3": {old, 4, "manager"}} {
		_ = p
		r := scorer.Result{Score: sc.score, Track: sc.track, WorkModel: "remote", Why: "w", Gap: "g", Scorer: scorer.ScorerRules}
		if err := st.SaveScore(ctx, sc.post, r, "", monday.Add(-2*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}

	msg, n, err := buildDigest(ctx, cfg, st, monday, monday.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || len(msg.Sections) != 3 {
		t.Fatalf("want 3 jobs in 3 sections (2 tracks + still open), got %d in %d: %+v", n, len(msg.Sections), msg.Sections)
	}
	if msg.Sections[0].Title != "Lead IC" || msg.Sections[1].Title != "Manager roles" || msg.Sections[2].Title != "Still open from earlier weeks" {
		t.Errorf("section titles: %q %q %q", msg.Sections[0].Title, msg.Sections[1].Title, msg.Sections[2].Title)
	}
	if !strings.Contains(msg.Subject, "2 new roles scoring 3 or higher") {
		t.Errorf("subject: %q", msg.Subject)
	}

	// Votes recorded since the last digest are listed in the intro.
	if strings.Contains(msg.Intro, "Votes recorded") {
		t.Errorf("no votes yet, intro should not mention them: %q", msg.Intro)
	}
	if _, err := st.SaveFeedback(ctx, store.Feedback{PostingID: em.ID, Vote: store.VoteDown, Source: "terminal", At: monday.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	msg, _, err = buildDigest(ctx, cfg, st, monday, monday.Add(-24*time.Hour))
	if err != nil || !strings.Contains(msg.Intro, "Votes recorded since then: 👎 Acme — Engineering Manager.") {
		t.Errorf("intro should list the vote, got %q (err %v)", msg.Intro, err)
	}

	// The same data on a Tuesday has no "still open" section.
	tue := monday.AddDate(0, 0, 1)
	msg, n, err = buildDigest(ctx, cfg, st, tue, monday.Add(-24*time.Hour))
	if err != nil || n != 2 || len(msg.Sections) != 2 {
		t.Errorf("Tuesday: n=%d sections=%d err=%v", n, len(msg.Sections), err)
	}
	// Nothing in the window means nothing to send.
	if _, n, _ := buildDigest(ctx, cfg, st, tue, tue); n != 0 {
		t.Errorf("empty window should give 0 jobs, got %d", n)
	}
}

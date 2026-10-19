package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/eval"
	"github.com/pannamatena/job-radar/internal/notify"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
	"github.com/pannamatena/job-radar/internal/store"
)

// notMonday returns t, or the day before if t is a Monday. On Mondays the digest
// adds a "still open from earlier weeks" section, which tests about other parts
// of the digest should not depend on.
func notMonday(t time.Time) time.Time {
	if t.In(time.Local).Weekday() == time.Monday {
		return t.AddDate(0, 0, -1)
	}
	return t
}

// lowFixture stores three scored jobs: 5 (alerted), 3 (digest) and 2 (just below).
func lowFixture(t *testing.T, home string, now time.Time) *store.Store {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(filepath.Join(home, "radar.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	for i, sc := range []int{5, 3, 2} {
		p := posting.Posting{ID: "greenhouse:x/" + string(rune('1'+i)), Source: "greenhouse", Company: "Globex", Title: "Role " + string(rune('A'+i)),
			Location: "Dublin", URL: "https://example.com/" + string(rune('1'+i)), DescriptionText: "The job text."}
		if _, err := st.SaveFetched(ctx, []posting.Posting{p}, now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		r := scorer.Result{Score: sc, Track: "none", WorkModel: "remote", Why: "w", Gap: "gap " + p.Title, Scorer: scorer.ScorerRules}
		if err := st.SaveScore(ctx, p, r, "", now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

func TestListLowShowsOnlyBelowTheDigestLine(t *testing.T) {
	home, now := t.TempDir(), time.Now()
	st := lowFixture(t, home, now)
	cfg := &config.Config{Rules: config.Rules{Thresholds: config.Thresholds{Alert: 4, Digest: 3}}}
	if _, err := st.SaveFeedback(context.Background(), store.Feedback{PostingID: "greenhouse:x/3", Vote: store.VoteUp, Source: "terminal", At: now}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := printLowScored(context.Background(), &out, cfg, home, 14, 30, now); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Role C") || strings.Contains(got, "Role A") || strings.Contains(got, "Role B") {
		t.Errorf("only the score-2 job belongs here:\n%s", got)
	}
	if !strings.Contains(got, "greenhouse:x/3") || !strings.Contains(got, "👍") {
		t.Errorf("id and current vote should show:\n%s", got)
	}
}

func TestDigestAddsNearMissesButNeverAloneAndCanBeOff(t *testing.T) {
	ctx, now := context.Background(), notMonday(time.Now())
	st := lowFixture(t, t.TempDir(), now)
	cfg := &config.Config{Rules: config.Rules{Thresholds: config.Thresholds{Alert: 4, Digest: 3}}}
	msg, n, err := buildDigest(ctx, cfg, st, now, now.Add(-24*time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	last := msg.Sections[len(msg.Sections)-1]
	if !strings.HasPrefix(last.Title, "Just below the line") || len(last.Alerts) != 1 || last.Alerts[0].Score != 2 {
		t.Fatalf("want a near-miss section with the score-2 job, got %+v", msg.Sections)
	}
	if got := len(notify.Numbered(msg)); got != 3 {
		t.Errorf("near-misses are numbered so they can be voted on; numbered=%d", got)
	}
	zero := 0
	cfg.Email.NearMisses = &zero
	if msg, _, _ = buildDigest(ctx, cfg, st, now, now.Add(-24*time.Hour)); len(msg.Sections) != 1 {
		t.Errorf("near_misses: 0 should turn the section off, got %d sections", len(msg.Sections))
	}
	// Only a near-miss and nothing else: no digest.
	cfg.Email.NearMisses = nil
	cfg.Rules.Thresholds.Digest = 3
	if _, n, _ := buildDigest(ctx, cfg, st, now, now.Add(30*time.Minute)); n != 0 {
		t.Errorf("an empty window must not send a digest, n=%d", n)
	}
}

func TestEvalDraftWritesBlankDraftsAndFindsDisagreements(t *testing.T) {
	ctx, now := context.Background(), time.Now()
	home := t.TempDir()
	st := lowFixture(t, home, now)
	for id, v := range map[string]string{"greenhouse:x/3": store.VoteUp, "greenhouse:x/1": store.VoteUp, "greenhouse:x/2": store.VoteDown} {
		if _, err := st.SaveFeedback(ctx, store.Feedback{PostingID: id, Vote: v, Source: "terminal", At: now}); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := disagreements(ctx, st, 4)
	if err != nil || len(ids) != 1 || ids[0] != "greenhouse:x/3" {
		// x/1: 👍 on a 5 agrees. x/2: 👎 on a 3 (below alert) agrees. x/3: 👍 on a 2 disagrees.
		t.Fatalf("want only x/3, got %v (err %v)", ids, err)
	}
	dir := filepath.Join(home, "evals", "cases")
	name, created, err := draftCase(ctx, st, dir, "greenhouse:x/3")
	if err != nil || !created || name != "globex-role-c" {
		t.Fatalf("name=%q created=%v err=%v", name, created, err)
	}
	if _, created, _ := draftCase(ctx, st, dir, "greenhouse:x/3"); created {
		t.Error("a second draft must not overwrite the first")
	}
	ad, _ := os.ReadFile(filepath.Join(dir, name, "ad.txt"))
	if !strings.HasPrefix(string(ad), "Role C — Globex (Dublin)\n") || !strings.Contains(string(ad), "The job text.") {
		t.Errorf("ad.txt = %q", ad)
	}
	// The draft loads as a draft, never as a confirmed (measured) case, and carries no score of ours as a label.
	res, err := eval.LoadCases(dir, []string{"lead_ic"}, nil)
	if err != nil || len(res.Confirmed) != 0 || len(res.Drafts) != 1 {
		t.Fatalf("confirmed=%d drafts=%v err=%v", len(res.Confirmed), res.Drafts, err)
	}
	label, _ := os.ReadFile(filepath.Join(dir, name, "label.yaml"))
	if !strings.Contains(string(label), "status: draft") || !strings.Contains(string(label), "score:              #") {
		t.Errorf("label must be a draft with a blank score:\n%s", label)
	}
}

func TestCaseName(t *testing.T) {
	if got := caseName("Acme, Inc.", "Staff Engineer (Platform) – EMEA"); got != "acme-inc-staff-engineer-platform-emea" {
		t.Errorf("got %q", got)
	}
	if got := caseName("", "…"); got != "case" {
		t.Errorf("got %q", got)
	}
}

func TestListScoredShowsJobsAtOrAboveMin(t *testing.T) {
	home, now := t.TempDir(), time.Now()
	lowFixture(t, home, now)
	var out bytes.Buffer
	if err := printScored(context.Background(), &out, home, 3, 14, 30, now); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Role A") || !strings.Contains(got, "Role B") || strings.Contains(got, "Role C") {
		t.Errorf("want scores 5 and 3 only:\n%s", got)
	}
	if strings.Index(got, "Role A") > strings.Index(got, "Role B") {
		t.Errorf("best score should come first:\n%s", got)
	}
	if err := printScored(context.Background(), &out, home, 6, 14, 30, now); err == nil {
		t.Error("--min 6 should be rejected")
	}
}

func TestShowPrintsScoreAndReasons(t *testing.T) {
	ctx, now := context.Background(), time.Now()
	st := lowFixture(t, t.TempDir(), now)
	var out bytes.Buffer
	if err := printShow(ctx, &out, st, "greenhouse:x/2"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Globex — Role B", "Score:  3", "rules scorer", "Gap:    gap Role B"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if err := printShow(ctx, &out, st, "greenhouse:x/99"); err == nil || !strings.Contains(err.Error(), "no posting") {
		t.Errorf("unknown id should say so, got %v", err)
	}
}

func TestClosedJobsAreMarked(t *testing.T) {
	ctx, now := context.Background(), time.Now()
	st := lowFixture(t, t.TempDir(), now)
	// A later read of the same board that still lists only Role B (…/2).
	still := posting.Posting{ID: "greenhouse:x/2", Source: "greenhouse", Company: "Globex", Title: "Role B",
		Location: "Dublin", URL: "https://example.com/2", DescriptionText: "The job text."}
	if _, err := st.SaveFetched(ctx, []posting.Posting{still}, now); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := printShow(ctx, &out, st, "greenhouse:x/1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Status: closed") {
		t.Errorf("Role A is gone from the board and should show as closed:\n%s", out.String())
	}
	out.Reset()
	if err := printShow(ctx, &out, st, "greenhouse:x/2"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "closed") {
		t.Errorf("Role B is still listed and must not show as closed:\n%s", out.String())
	}
	out.Reset()
	if err := printScored(ctx, &out, t.TempDir(), 1, 14, 30, now); err != nil {
		t.Fatal(err)
	}
}

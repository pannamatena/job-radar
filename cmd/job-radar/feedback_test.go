package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/mailbox"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/store"
)

type fakeMailbox struct{ replies []mailbox.Reply }

func (f fakeMailbox) Replies(context.Context, time.Time, []string) ([]mailbox.Reply, error) {
	return f.replies, nil
}

func TestSyncFeedback(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "radar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	var ps []posting.Posting
	for _, id := range []string{"a/1", "a/2", "a/3"} {
		ps = append(ps, posting.Posting{ID: "greenhouse:" + id, Source: "greenhouse", Company: "Acme", Title: "Role " + id, URL: "https://example.com/" + id})
	}
	if _, err := st.SaveFetched(ctx, ps, now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordNotification(ctx, store.Notification{Channel: emailChannel, Kind: store.KindAlert, SentAt: now,
		Subject: "s", MessageID: "<sent1@example.com>", Recipients: []string{"me@example.com"},
		PostingIDs: []string{ps[0].ID, ps[1].ID, ps[2].ID}}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Email: config.Email{To: []string{"me@example.com"}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	good := mailbox.Reply{MessageID: "<r1@x>", Refs: []string{"<sent1@example.com>"}, From: "me@example.com", Body: "1 👍, 3 down\n\n> 2 up"}
	rd := fakeMailbox{replies: []mailbox.Reply{
		good,
		{MessageID: "<r2@x>", Refs: []string{"<sent1@example.com>"}, From: "stranger@example.com", Body: "2 up"}, // not you
		{MessageID: "<r3@x>", Refs: []string{"<unrelated@x>"}, From: "me@example.com", Body: "2 up"},             // not our email
		{MessageID: "<r4@x>", Refs: []string{"<sent1@example.com>"}, From: "me@example.com", Body: "thanks!"},    // no votes
		{MessageID: "<r5@x>", Refs: []string{"<sent1@example.com>"}, From: "me@example.com", Body: "9 up"},       // out of range
	}}
	var out bytes.Buffer
	res, err := syncFeedback(ctx, logger, cfg, st, rd, now, &out)
	if err != nil || res.Added != 2 || res.Unreadable != 2 {
		t.Fatalf("res=%+v err=%v out=%q", res, err, out.String())
	}
	if !strings.Contains(out.String(), "Recorded 👍 Acme — Role a/1") {
		t.Errorf("sync should say what it recorded, got %q", out.String())
	}
	// Reading the same replies again adds nothing.
	if r, _ := syncFeedback(ctx, logger, cfg, st, rd, now, &out); r.Added != 0 {
		t.Errorf("second sync added %d vote(s), want 0", r.Added)
	}
	rows, err := st.RecentFeedback(ctx, now.Add(-time.Hour))
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.PostingID] = r.Vote
	}
	if got[ps[0].ID] != "up" || got[ps[2].ID] != "down" || len(got) != 2 {
		t.Errorf("votes = %v", got)
	}
}

func TestCurrentVotes(t *testing.T) {
	at := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	row := func(id, vote string, mins int) store.FeedbackRow {
		return store.FeedbackRow{Feedback: store.Feedback{PostingID: id, Vote: vote, At: at.Add(time.Duration(mins) * time.Minute)}}
	}
	// newest first, as RecentFeedback returns them
	got := currentVotes([]store.FeedbackRow{row("a", "down", 5), row("b", "up", 4), row("a", "up", 1)})
	if len(got) != 2 || got[0].PostingID != "a" || got[0].Vote != "down" || got[0].ChangedFrom != "up" {
		t.Fatalf("a should show 👎 changed from 👍, got %+v", got)
	}
	if got[1].PostingID != "b" || got[1].ChangedFrom != "" {
		t.Errorf("b unchanged, got %+v", got[1])
	}
}

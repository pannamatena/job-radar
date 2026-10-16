package store

import (
	"context"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/posting"
)

func TestNotifications_NoDuplicateAlerts(t *testing.T) {
	s, _ := openTemp(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
	a, b := mkPosting("greenhouse:a/1", "Engineering Manager"), mkPosting("greenhouse:a/2", "Tech Lead")
	if _, err := s.SaveFetched(ctx, []posting.Posting{a, b}, now); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := s.LastNotification(ctx, "email", KindDigest); err != nil || ok {
		t.Fatalf("no digest yet: ok=%v err=%v", ok, err)
	}
	_, err := s.RecordNotification(ctx, Notification{
		Channel: "email", Kind: KindAlert, SentAt: now, Subject: "s", MessageID: "<m1@x>",
		Recipients: []string{"me@example.com"}, PostingIDs: []string{a.ID},
	})
	if err != nil {
		t.Fatalf("RecordNotification: %v", err)
	}
	got, err := s.AlreadyAlerted(ctx, "email", []string{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !got[a.ID] || got[b.ID] {
		t.Errorf("AlreadyAlerted = %v, want only %s", got, a.ID)
	}
	// A digest listing b is not an alert about b, and another channel is separate.
	if _, err := s.RecordNotification(ctx, Notification{Channel: "email", Kind: KindDigest, SentAt: now.Add(time.Hour),
		Subject: "d", Recipients: []string{"me@example.com"}, PostingIDs: []string{b.ID}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.AlreadyAlerted(ctx, "email", []string{b.ID}); got[b.ID] {
		t.Error("a digest must not count as an alert")
	}
	if got, _ := s.AlreadyAlerted(ctx, "slack", []string{a.ID}); got[a.ID] {
		t.Error("alerts are tracked per channel")
	}
	last, ok, err := s.LastNotification(ctx, "email", KindDigest)
	if err != nil || !ok || !last.Equal(now.Add(time.Hour)) {
		t.Errorf("LastNotification = %v, %v, %v", last, ok, err)
	}
}

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
	"github.com/pannamatena/job-radar/internal/notify"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/store"
)

func TestAlertSubject(t *testing.T) {
	one := alertSubject([]notify.Alert{{Company: "Acme"}})
	if one != "job-radar: 1 strong match – Acme" {
		t.Errorf("one: %q", one)
	}
	many := alertSubject([]notify.Alert{{Company: "A"}, {Company: "A"}, {Company: "B"}, {Company: "C"}, {Company: "D"}, {Company: "E"}})
	if many != "job-radar: 6 strong matches – A, B, C +2 more" {
		t.Errorf("many: %q", many)
	}
}

func TestSendAlertEmail_DryRunSkipsAlreadyEmailedAndRecordsNothing(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "radar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	a := posting.Posting{ID: "greenhouse:acme/1", Source: "greenhouse", Company: "Acme", Title: "EM", URL: "https://example.com/1"}
	b := posting.Posting{ID: "greenhouse:acme/2", Source: "greenhouse", Company: "Acme", Title: "TL", URL: "https://example.com/2"}
	if _, err := st.SaveFetched(ctx, []posting.Posting{a, b}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordNotification(ctx, store.Notification{Channel: emailChannel, Kind: store.KindAlert, SentAt: now,
		Subject: "old", Recipients: []string{"me@example.com"}, PostingIDs: []string{a.ID}}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Email: config.Email{To: []string{"me@example.com"}}}
	alerts := []notify.Alert{{ID: a.ID, Company: "Acme"}, {ID: b.ID, Company: "Acme"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var out bytes.Buffer
	sendAlertEmail(ctx, logger, cfg, st, alerts, true, &out)
	if !strings.Contains(out.String(), "would email 1 strong match") {
		t.Errorf("only the not-yet-emailed match should be listed: %q", out.String())
	}
	if done, _ := st.AlreadyAlerted(ctx, emailChannel, []string{b.ID}); done[b.ID] {
		t.Error("a dry run must not record the notification")
	}

	// Email switched off, or alerts switched off: nothing is said or sent.
	out.Reset()
	sendAlertEmail(ctx, logger, &config.Config{}, st, alerts, true, &out)
	off := false
	sendAlertEmail(ctx, logger, &config.Config{Email: config.Email{To: []string{"me@example.com"}, MatchAlerts: &off}}, st, alerts, true, &out)
	if out.Len() != 0 {
		t.Errorf("expected silence when email or alerts are off, got %q", out.String())
	}
}

func TestNewEmailNotifier_MissingSecretsNamesThemWithoutValues(t *testing.T) {
	t.Setenv(envEmailAddress, "")
	t.Setenv(envEmailPassword, "")
	_, err := newEmailNotifier(&config.Config{Email: config.Email{To: []string{"me@example.com"}}})
	if err == nil || !strings.Contains(err.Error(), envEmailAddress) || !strings.Contains(err.Error(), envEmailPassword) {
		t.Errorf("error should name both variables: %v", err)
	}
	t.Setenv(envEmailAddress, "bot@example.com")
	t.Setenv(envEmailPassword, "abcd efgh ijkl mnop")
	n, err := newEmailNotifier(&config.Config{Email: config.Email{To: []string{"me@example.com"}, SMTPHost: "h", SMTPPort: 587}})
	if err != nil || n.Password != "abcdefghijklmnop" {
		t.Errorf("spaces in an app password should be dropped: %v %v", n, err)
	}
}

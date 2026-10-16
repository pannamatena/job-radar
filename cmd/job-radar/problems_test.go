package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/store"
)

func TestRecordSourceHealth_NoticeOnlyAfterSeveralFailuresInARow(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "radar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Now()
	bad := []sourceOutcome{{Company: "Acme", Err: errors.New("503")}, {Company: "Globex"}}
	for i := 1; i <= 2; i++ {
		if lines := recordSourceHealth(ctx, logger, st, bad, now); len(lines) != 0 {
			t.Fatalf("run %d: too early for a notice: %v", i, lines)
		}
	}
	lines := recordSourceHealth(ctx, logger, st, bad, now)
	if len(lines) != 1 || !strings.Contains(lines[0], "Acme") || !strings.Contains(lines[0], "3 runs in a row") {
		t.Fatalf("third failure should give one notice about Acme: %v", lines)
	}
	// A success resets the streak.
	recordSourceHealth(ctx, logger, st, []sourceOutcome{{Company: "Acme"}}, now)
	if lines := recordSourceHealth(ctx, logger, st, bad, now); len(lines) != 0 {
		t.Errorf("streak should have reset: %v", lines)
	}
}

func TestSendProblem_DryRunAndOff(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "radar.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	on := &config.Config{Email: config.Email{To: []string{"me@example.com"}}}

	var out bytes.Buffer
	sendProblem(ctx, logger, on, st, []string{"something broke"}, true, &out)
	if !strings.Contains(out.String(), "would email a problem notice (1 issue") {
		t.Errorf("dry run output: %q", out.String())
	}
	out.Reset()
	sendProblem(ctx, logger, &config.Config{}, st, []string{"x"}, true, &out)
	sendProblem(ctx, logger, on, st, nil, true, &out)
	if out.Len() != 0 {
		t.Errorf("email off or nothing wrong should be silent, got %q", out.String())
	}
	if _, ok, _ := st.LastNotification(ctx, emailChannel, store.KindProblem); ok {
		t.Error("a dry run must not record a notice")
	}
}

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

	"github.com/pannamatena/job-radar/internal/agent"
	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/scorer"
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

func TestCapNotice(t *testing.T) {
	tests := []struct {
		name          string
		before, after float64
		cap           float64
		want          string // "" = no notice; otherwise a phrase the notice must contain
	}{
		{"well under the cap", 1, 2, 5, ""},
		{"just under the warning line", 3.0, 3.9, 5, ""},
		{"this run crosses 80%", 3.5, 4.2, 5, "over 80% of your $5.00 cap"},
		{"already past 80% before this run: no repeat warning", 4.2, 4.6, 5, ""},
		{"this run reaches the cap", 4.5, 5.1, 5, "is reached"},
		{"already over the cap", 6, 6, 5, "is reached"},
		{"no cap set", 1, 100, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capNotice("AI", "so things stop", "llm.monthly_spend_cap_usd", tt.before, tt.after, tt.cap)
			if tt.want == "" && got != "" || tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("capNotice = %q, want containing %q", got, tt.want)
			}
		})
	}
	if got := capNotice("AI", "so things stop", "llm.monthly_spend_cap_usd", 1, 6, 5); !strings.Contains(got, "llm.monthly_spend_cap_usd") {
		t.Errorf("the notice should name the setting to change: %q", got)
	}
}

type stubAI struct{}

func (stubAI) Score(context.Context, posting.Posting) scorer.Result { return scorer.Result{} }
func (stubAI) Model() string                                        { return "m" }
func (stubAI) PromptVersion() string                                { return "p" }

// The AI cap notice used to be built after the problem email had gone out, so
// it never reached the user (issue #17). The summary must hand it back.
func TestAIRunSummaryReturnsTheCapNoticeAndFillsTheStats(t *testing.T) {
	cfg := &config.Config{LLM: config.LLM{MonthlySpendCapUSD: 5}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var out bytes.Buffer

	gate := &agent.Gated{AI: stubAI{}, SpentThisMonthUSD: 5.2, RunCostUSD: 0.3, Sent: 7, SkippedCap: 2}
	var stats store.RunStats
	notice := aiRunSummary(&out, logger, cfg, gate, &stats)
	if !strings.Contains(notice, "AI monthly spend cap ($5.00) is reached") {
		t.Errorf("notice = %q", notice)
	}
	if stats.AISent != 7 || stats.AISkipped != 2 || stats.CostEstimate != 0.3 {
		t.Errorf("stats = %+v", stats)
	}
	if !strings.Contains(out.String(), "7 sent") || !strings.Contains(out.String(), "of your $5.00 cap") {
		t.Errorf("summary text: %q", out.String())
	}

	quiet := &agent.Gated{AI: stubAI{}, SpentThisMonthUSD: 0.5, RunCostUSD: 0.1}
	if n := aiRunSummary(&out, logger, cfg, quiet, &stats); n != "" {
		t.Errorf("far from the cap there is nothing to say, got %q", n)
	}
}

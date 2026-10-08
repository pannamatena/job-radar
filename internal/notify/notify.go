// Package notify delivers match alerts. V1 ships only a terminal notifier (for
// development) and, in phase 4, email; the Notifier interface keeps other
// channels (Slack, Telegram…) addable in V2 without touching the pipeline
// (ADR-0024). The notifier is deliberately decoupled from the scorer and
// posting types — callers build plain Alert values — so this package has no
// dependency on how a score was produced.
package notify

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Alert is one posting worth telling the user about, flattened to display
// fields so notifiers don't depend on internal types.
type Alert struct {
	ID        string // stable posting id, e.g. for `score-one`
	Company   string
	Title     string
	Location  string
	Track     string
	WorkModel string
	Score     int
	Why       string
	Gap       string
	URL       string
	Flags     []string
	New       bool // first time the user is seeing this posting
}

// Notifier delivers a batch of alerts under a short subject. One call per run.
type Notifier interface {
	Send(ctx context.Context, subject string, alerts []Alert) error
}

// Terminal prints alerts to a writer. It's the temporary V1 notifier used
// before email lands in phase 4.
type Terminal struct {
	W io.Writer
}

// Send writes a readable, grouped summary of the alerts.
func (t *Terminal) Send(_ context.Context, subject string, alerts []Alert) error {
	if len(alerts) == 0 {
		fmt.Fprintln(t.W, "No strong matches this run.")
		return nil
	}
	fmt.Fprintf(t.W, "\n%s\n%s\n", subject, strings.Repeat("─", len(subject)))
	for i, a := range alerts {
		marker := " "
		if a.New {
			marker = "★" // new to you
		}
		fmt.Fprintf(t.W, "\n%s %d. [%d] %s — %s\n", marker, i+1, a.Score, a.Company, a.Title)
		fmt.Fprintf(t.W, "     %s · %s", orNone(a.Track), orNone(a.WorkModel))
		if a.Location != "" {
			fmt.Fprintf(t.W, " · %s", a.Location)
		}
		fmt.Fprintln(t.W)
		if a.Why != "" {
			fmt.Fprintf(t.W, "     why: %s\n", a.Why)
		}
		if a.Gap != "" {
			fmt.Fprintf(t.W, "     gap: %s\n", a.Gap)
		}
		if a.ID != "" {
			fmt.Fprintf(t.W, "     id:  %s\n", a.ID)
		}
		if a.URL != "" {
			fmt.Fprintf(t.W, "     %s\n", a.URL)
		}
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

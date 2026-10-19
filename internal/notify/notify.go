// Package notify delivers match alerts. V1 ships a terminal notifier (used for
// dry runs) and email; the Notifier interface keeps other
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
	Scorer    string // "rules" or "llm", shown so the user knows who judged it
	New       bool   // first time the user is seeing this posting
}

// Message kinds.
const (
	KindAlert   = "alert"   // new strong matches at the end of a run
	KindDigest  = "digest"  // the daily summary
	KindProblem = "problem" // something needs the user's attention
)

// Section is a titled group of alerts (the digest groups them by track).
type Section struct {
	Title  string
	Alerts []Alert
}

// Message is one notification: a subject, an optional intro line, and zero or
// more groups of numbered jobs. A problem notice has only an intro.
type Message struct {
	Kind     string
	Subject  string
	Intro    string
	Sections []Section
	// Leads are unscored search results from other sites, shown after the jobs.
	// They are not numbered and can't be voted on. NewSites lists sites seen for
	// the first time, so the reader can sign up for their job alerts.
	Leads    []Lead
	NewSites []string
}

// Sent is what a notifier reports back, so the caller can record it.
type Sent struct {
	// MessageID identifies the email, so a reply can be matched to it.
	MessageID string
	// Numbered lists the posting ids in the order they were numbered (the job
	// shown as number n is Numbered[n-1]).
	Numbered []string
}

// Numbered returns the posting ids of a message in display order. Notifiers and
// the code that records a send use it, so they always agree on the numbering.
func Numbered(m Message) []string {
	var ids []string
	for _, s := range m.Sections {
		for _, a := range s.Alerts {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

// Notifier delivers one message.
type Notifier interface {
	Send(ctx context.Context, m Message) (Sent, error)
}

// Terminal prints a message to a writer. It's the dry-run notifier and the
// fallback when email is off.
type Terminal struct {
	W io.Writer
}

// Send writes a readable summary of the message.
func (t *Terminal) Send(_ context.Context, m Message) (Sent, error) {
	sent := Sent{Numbered: Numbered(m)}
	if len(sent.Numbered) == 0 && m.Intro == "" {
		fmt.Fprintln(t.W, "No strong matches this run.")
		return sent, nil
	}
	fmt.Fprintf(t.W, "\n%s\n%s\n", m.Subject, strings.Repeat("─", len(m.Subject)))
	if m.Intro != "" {
		fmt.Fprintf(t.W, "%s\n", m.Intro)
	}
	n := 0
	for _, sec := range m.Sections {
		if sec.Title != "" {
			fmt.Fprintf(t.W, "\n== %s ==\n", sec.Title)
		}
		for _, a := range sec.Alerts {
			n++
			t.writeAlert(n, a)
		}
	}
	return sent, nil
}

func (t *Terminal) writeAlert(i int, a Alert) {
	marker := " "
	if a.New {
		marker = "★" // new to you
	}
	by := ""
	if a.Scorer == "llm" {
		by = " (AI)"
	}
	fmt.Fprintf(t.W, "\n%s %d. [%d%s] %s — %s\n", marker, i, a.Score, by, a.Company, a.Title)
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

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

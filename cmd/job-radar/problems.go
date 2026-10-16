package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/notify"
	"github.com/pannamatena/job-radar/internal/store"
)

// failuresBeforeNotice is how many runs in a row a job board must fail before
// it's worth an email. One failed run is usually a blip.
const failuresBeforeNotice = 3

// problemInterval is the least time between two problem emails.
const problemInterval = 24 * time.Hour

// recordSourceHealth updates each company's failure streak and returns a
// sentence for every board that has now failed several runs in a row.
func recordSourceHealth(ctx context.Context, logger *slog.Logger, st *store.Store, outcomes []sourceOutcome, now time.Time) []string {
	var lines []string
	for _, o := range outcomes {
		errText := ""
		if o.Err != nil {
			errText = o.Err.Error()
		}
		n, err := st.RecordSourceResult(ctx, o.Company, o.Err != nil, errText, now)
		if err != nil {
			logger.Warn("could not record source health", "company", o.Company, "error", err.Error())
			continue
		}
		if n >= failuresBeforeNotice {
			lines = append(lines, fmt.Sprintf("%s's job board has failed %d runs in a row (last error: %s).", o.Company, n, errText))
		}
	}
	return lines
}

// sendProblem emails what's wrong, at most once a day however many problems
// there are. Failing to send it is only logged: it must never hide the run's own
// result.
func sendProblem(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, problems []string, dryRun bool, out io.Writer) {
	if len(problems) == 0 || !cfg.Email.Enabled() {
		return
	}
	msg := notify.Message{
		Kind:    notify.KindProblem,
		Subject: "job-radar: needs your attention",
		Intro:   "job-radar noticed:\n\n- " + strings.Join(problems, "\n- ") + "\n\nThis is the only problem email you'll get today.",
	}
	if dryRun {
		fmt.Fprintf(out, "Dry run: would email a problem notice (%d issue(s)).\n", len(problems))
		return
	}
	last, have, err := st.LastNotification(ctx, emailChannel, store.KindProblem)
	if err == nil && have && time.Since(last) < problemInterval {
		logger.Info("problem notice skipped: one was sent in the last day", "issues", len(problems))
		return
	}
	n, err := newEmailNotifier(cfg)
	if err != nil {
		logger.Warn("cannot send problem notice", "error", err.Error())
		return
	}
	sent, err := n.Send(ctx, msg)
	if err != nil {
		logger.Warn("email failed", "kind", notify.KindProblem, "error", err.Error())
		return
	}
	if _, err := st.RecordNotification(ctx, store.Notification{
		Channel: emailChannel, Kind: store.KindProblem, SentAt: time.Now(), Subject: msg.Subject,
		MessageID: sent.MessageID, Recipients: cfg.Email.To,
	}); err != nil {
		logger.Warn("could not record notification", "error", err.Error())
	}
	fmt.Fprintf(out, "Emailed a problem notice (%d issue(s)) to %s.\n", len(problems), strings.Join(cfg.Email.To, ", "))
}

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

// capWarnFraction is how much of a monthly spend cap is used when the user gets
// an early warning (once, on the run that crosses it).
const capWarnFraction = 0.8

// capNotice words what to tell the user about a monthly spend cap, or returns ""
// if there is nothing to say. before and after are the estimated spend this
// month at the start and end of the run. It says so when the cap is reached
// (every run until the month ends, but a problem email goes out at most once a
// day), and warns once, on the run that takes spend past 80% of the cap.
func capNotice(what, consequence, setting string, before, after, cap float64) string {
	if cap <= 0 {
		return ""
	}
	if after >= cap {
		return fmt.Sprintf("The %s monthly spend cap ($%.2f) is reached, %s (or until you raise %s).", what, cap, consequence, setting)
	}
	if after >= cap*capWarnFraction && before < cap*capWarnFraction {
		return fmt.Sprintf("Your %s spend this month is about $%.2f, over %d%% of your $%.2f cap. It stops at the cap; raise %s if you want more.",
			what, after, int(capWarnFraction*100), cap, setting)
	}
	return ""
}

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

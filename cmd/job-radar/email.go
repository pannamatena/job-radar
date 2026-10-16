package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/notify"
	"github.com/pannamatena/job-radar/internal/store"
)

// Environment variable names for the sending account (see docs/email-setup.md).
const (
	envEmailAddress  = "RADAR_EMAIL_ADDRESS"
	envEmailPassword = "RADAR_EMAIL_APP_PASSWORD"
)

// emailChannel is the channel name recorded in the notifications table.
const emailChannel = "email"

// newEmailNotifier builds the SMTP sender from the config and the environment.
// The secrets are read from the environment (usually loaded from .env) and are
// never printed.
func newEmailNotifier(cfg *config.Config) (*notify.Email, error) {
	from, pass := strings.TrimSpace(os.Getenv(envEmailAddress)), os.Getenv(envEmailPassword)
	var missing []string
	if from == "" {
		missing = append(missing, envEmailAddress)
	}
	if strings.TrimSpace(pass) == "" {
		missing = append(missing, envEmailPassword)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("email is on (email.to is set) but %s is missing; put it in your .env file (see docs/email-setup.md)", strings.Join(missing, " and "))
	}
	return &notify.Email{
		Host: cfg.Email.SMTPHost, Port: cfg.Email.SMTPPort,
		From: from, Password: strings.ReplaceAll(pass, " ", ""), To: cfg.Email.To,
	}, nil
}

// alertSubject is a short subject that says what's inside, e.g.
// "job-radar: 2 strong matches – Acme, Globex".
func alertSubject(alerts []notify.Alert) string {
	var names []string
	seen := map[string]bool{}
	for _, a := range alerts {
		if !seen[a.Company] {
			seen[a.Company] = true
			names = append(names, a.Company)
		}
	}
	more := ""
	if len(names) > 3 {
		more = fmt.Sprintf(" +%d more", len(names)-3)
		names = names[:3]
	}
	noun := "strong matches"
	if len(alerts) == 1 {
		noun = "strong match"
	}
	return fmt.Sprintf("job-radar: %d %s – %s%s", len(alerts), noun, strings.Join(names, ", "), more)
}

// sendAlertEmail emails the strong matches that haven't been emailed before.
// An email failure is reported but never fails the run: the scores are already
// saved, and the postings stay un-alerted so the next run tries again.
func sendAlertEmail(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, alerts []notify.Alert, dryRun bool, out io.Writer) {
	if !cfg.Email.Enabled() || !cfg.Email.AlertsOn() {
		return
	}
	ids := make([]string, len(alerts))
	for i, a := range alerts {
		ids[i] = a.ID
	}
	done, err := st.AlreadyAlerted(ctx, emailChannel, ids)
	if err != nil {
		fmt.Fprintf(out, "! Couldn't check which matches were already emailed: %s\n", err)
		return
	}
	var fresh []notify.Alert
	for _, a := range alerts {
		if !done[a.ID] {
			fresh = append(fresh, a)
		}
	}
	if len(fresh) == 0 {
		fmt.Fprintln(out, "No new strong matches to email.")
		return
	}
	msg := notify.Message{Kind: notify.KindAlert, Subject: alertSubject(fresh), Sections: []notify.Section{{Alerts: fresh}}}
	if dryRun {
		fmt.Fprintf(out, "Dry run: would email %d strong match(es) to %s.\n", len(fresh), strings.Join(cfg.Email.To, ", "))
		return
	}
	n, err := newEmailNotifier(cfg)
	if err != nil {
		fmt.Fprintf(out, "! %s\n", err)
		return
	}
	sent, err := n.Send(ctx, msg)
	if err != nil {
		logger.Warn("email failed", "kind", notify.KindAlert, "error", err.Error())
		fmt.Fprintf(out, "! Couldn't send the alert email: %s\n  The matches will be tried again on the next run.\n", err)
		return
	}
	if _, err := st.RecordNotification(ctx, store.Notification{
		Channel: emailChannel, Kind: store.KindAlert, SentAt: time.Now(), Subject: msg.Subject,
		MessageID: sent.MessageID, Recipients: cfg.Email.To, PostingIDs: sent.Numbered,
	}); err != nil {
		logger.Warn("could not record notification", "error", err.Error())
	}
	fmt.Fprintf(out, "Emailed %d strong match(es) to %s.\n", len(fresh), strings.Join(cfg.Email.To, ", "))
}

// cmdEmailTest sends one short, clearly-labelled test email, so you can check
// the setup (sender account, app password, recipient, spam folder) before
// relying on it.
func cmdEmailTest(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("email-test", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}
	if !cfg.Email.Enabled() {
		return fmt.Errorf("email is off: add your address under email.to in %s/config.yaml (see docs/email-setup.md)", *home)
	}
	n, err := newEmailNotifier(cfg)
	if err != nil {
		return err
	}
	msg := notify.Message{
		Kind:    notify.KindProblem,
		Subject: "job-radar: test email",
		Intro:   "This is a test email from job-radar. If you can read it in your main inbox (not spam), alerts will reach you. Nothing else to do.",
	}
	if _, err := n.Send(ctx, msg); err != nil {
		return err
	}
	fmt.Printf("Sent a test email to %s. Check your inbox (and spam, once).\n", strings.Join(cfg.Email.To, ", "))
	return nil
}

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/notify"
	"github.com/pannamatena/job-radar/internal/store"
)

// maxDigestWindow stops a long gap (a laptop that was off for a week) turning
// the next digest into a flood.
const maxDigestWindow = 72 * time.Hour

// digestDue reports whether the daily digest should go out now: it's past
// today's digest time and none has been sent since that time.
func digestDue(now, lastSent time.Time, haveLast bool, hhmm string) bool {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return false
	}
	local := now.In(time.Local)
	at := time.Date(local.Year(), local.Month(), local.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
	if local.Before(at) {
		return false
	}
	return !haveLast || lastSent.Before(at)
}

// digestWindowStart is where "new since the last digest" begins.
func digestWindowStart(now, lastSent time.Time, haveLast bool) time.Time {
	if haveLast && now.Sub(lastSent) <= maxDigestWindow {
		return lastSent
	}
	return now.Add(-24 * time.Hour)
}

// trackTitle is the digest heading for a track: its label if it has one,
// otherwise its name tidied up ("lead_ic" → "Lead IC").
func trackTitle(name string, tracks map[string]config.Track) string {
	if name == "" || name == "none" {
		return "Other"
	}
	if l := strings.TrimSpace(tracks[name].Label); l != "" {
		return l
	}
	words := strings.Fields(strings.ReplaceAll(name, "_", " "))
	for i, w := range words {
		if len(w) <= 3 {
			words[i] = strings.ToUpper(w)
		} else {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func toAlert(sp store.ScoredPosting, windowStart time.Time) notify.Alert {
	return notify.Alert{
		ID: sp.Posting.ID, Company: sp.Posting.Company, Title: sp.Posting.Title, Location: sp.Posting.Location,
		Track: sp.Track, WorkModel: sp.WorkModel, Score: sp.Score, Why: sp.Why, Gap: sp.Gap,
		URL: sp.Posting.URL, Flags: sp.Flags, Scorer: sp.Scorer, New: !sp.FirstSeen.Before(windowStart),
	}
}

// buildDigest groups the new scored postings by track (in track-name order), and
// on a Monday adds the roles that are still open from earlier. It returns the
// message and how many jobs it lists; zero means there's nothing to send.
func buildDigest(ctx context.Context, cfg *config.Config, st *store.Store, now, windowStart time.Time) (notify.Message, int, error) {
	min := cfg.Rules.Thresholds.Digest
	fresh, err := st.NewScored(ctx, windowStart, min)
	if err != nil {
		return notify.Message{}, 0, err
	}
	byTrack := map[string][]notify.Alert{}
	for _, sp := range fresh {
		byTrack[sp.Track] = append(byTrack[sp.Track], toAlert(sp, windowStart))
	}
	var names []string
	for name := range byTrack {
		names = append(names, name)
	}
	// Configured tracks alphabetically, then "none"/unknown last.
	sort.Slice(names, func(i, j int) bool {
		ai, aj := trackTitle(names[i], cfg.Rules.Tracks) == "Other", trackTitle(names[j], cfg.Rules.Tracks) == "Other"
		if ai != aj {
			return aj
		}
		return names[i] < names[j]
	})
	var msg notify.Message
	count := 0
	for _, name := range names {
		msg.Sections = append(msg.Sections, notify.Section{Title: trackTitle(name, cfg.Rules.Tracks), Alerts: byTrack[name]})
		count += len(byTrack[name])
	}
	newCount := count

	if now.In(time.Local).Weekday() == time.Monday {
		open, err := st.StillOpen(ctx, windowStart, now.Add(-48*time.Hour), min)
		if err != nil {
			return notify.Message{}, 0, err
		}
		var alerts []notify.Alert
		for _, sp := range open {
			alerts = append(alerts, toAlert(sp, windowStart))
		}
		if len(alerts) > 0 {
			msg.Sections = append(msg.Sections, notify.Section{Title: "Still open from earlier weeks", Alerts: alerts})
			count += len(alerts)
		}
	}
	if count == 0 {
		return notify.Message{}, 0, nil
	}
	// Jobs scored just under the digest line, so a wrong low score can be
	// corrected with a 👍 reply. Never the reason for a digest on its own.
	if n := cfg.Email.NearMissCount(); n > 0 && min > 1 {
		near, err := st.ScoredBetween(ctx, windowStart, min-1, min-1, n)
		if err != nil {
			return notify.Message{}, 0, err
		}
		var alerts []notify.Alert
		for _, sp := range near {
			alerts = append(alerts, toAlert(sp, windowStart))
		}
		if len(alerts) > 0 {
			msg.Sections = append(msg.Sections, notify.Section{
				Title:  "Just below the line (reply 👍 to any that should have scored higher)",
				Alerts: alerts,
			})
		}
	}
	noun := "roles"
	if newCount == 1 {
		noun = "role"
	}
	msg.Kind = notify.KindDigest
	msg.Subject = fmt.Sprintf("job-radar digest: %d new %s scoring %d or higher – %s", newCount, noun, min, now.In(time.Local).Format("Mon 2 Jan"))
	msg.Intro = fmt.Sprintf("New since your last digest, scoring %d or higher, grouped by track.", min)
	if votes, err := st.RecentFeedback(ctx, windowStart); err == nil && len(votes) > 0 {
		var parts []string
		for _, v := range currentVotes(votes) {
			parts = append(parts, voteMark(v.Vote)+" "+v.Company+" — "+v.Title)
		}
		msg.Intro += " Votes recorded since then: " + strings.Join(parts, "; ") + "."
	}
	return msg, count, nil
}

// sendDigest builds and (unless dryRun) emails the digest, then records it.
func sendDigest(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, now time.Time, hours int, dryRun bool, out io.Writer) error {
	last, have, err := st.LastNotification(ctx, emailChannel, store.KindDigest)
	if err != nil {
		return err
	}
	start := digestWindowStart(now, last, have)
	if hours > 0 {
		start = now.Add(-time.Duration(hours) * time.Hour)
	}
	msg, count, err := buildDigest(ctx, cfg, st, now, start)
	if err != nil {
		return err
	}
	if count == 0 {
		fmt.Fprintln(out, "Nothing new to put in a digest.")
		return nil
	}
	if dryRun {
		fmt.Fprintf(out, "Dry run: the digest would list %d job(s) and go to %s.\n", count, strings.Join(cfg.Email.To, ", "))
		_, err := (&notify.Terminal{W: out}).Send(ctx, msg)
		return err
	}
	n, err := newEmailNotifier(cfg)
	if err != nil {
		return err
	}
	sent, err := n.Send(ctx, msg)
	if err != nil {
		logger.Warn("email failed", "kind", notify.KindDigest, "error", err.Error())
		return fmt.Errorf("couldn't send the digest: %w", err)
	}
	if _, err := st.RecordNotification(ctx, store.Notification{
		Channel: emailChannel, Kind: store.KindDigest, SentAt: now, Subject: msg.Subject,
		MessageID: sent.MessageID, Recipients: cfg.Email.To, PostingIDs: sent.Numbered,
	}); err != nil {
		logger.Warn("could not record notification", "error", err.Error())
	}
	fmt.Fprintf(out, "Emailed the digest (%d job(s)) to %s.\n", count, strings.Join(cfg.Email.To, ", "))
	return nil
}

// maybeSendDigest is called at the end of a run: it sends the digest only when
// email and the digest are on and it's due. Failures are reported, not fatal.
func maybeSendDigest(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, dryRun bool, out io.Writer) {
	if !cfg.Email.Enabled() || !cfg.Email.DigestOn() {
		return
	}
	now := time.Now()
	last, have, err := st.LastNotification(ctx, emailChannel, store.KindDigest)
	if err != nil {
		fmt.Fprintf(out, "! Couldn't check the last digest: %s\n", err)
		return
	}
	if !digestDue(now, last, have, cfg.Email.DigestTime) {
		return
	}
	if err := sendDigest(ctx, logger, cfg, st, now, 0, dryRun, out); err != nil {
		fmt.Fprintf(out, "! %s\n", err)
	}
}

// cmdDigest sends the digest on demand, ignoring the time of day.
func cmdDigest(ctx context.Context, logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("digest", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	dryRun := fs.Bool("dry-run", false, "print the digest instead of emailing it")
	hours := fs.Int("hours", 0, "look back this many hours instead of since the last digest")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}
	if !cfg.Email.Enabled() && !*dryRun {
		return fmt.Errorf("email is off: add your address under email.to in %s/config.yaml (see docs/email-setup.md)", *home)
	}
	st, err := store.Open(filepath.Join(*home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	return sendDigest(ctx, logger, cfg, st, time.Now(), *hours, *dryRun, os.Stdout)
}

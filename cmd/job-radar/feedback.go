package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/feedback"
	"github.com/pannamatena/job-radar/internal/mailbox"
	"github.com/pannamatena/job-radar/internal/store"
)

// replyLookback is how far back replies are searched. Older replies are ignored.
const replyLookback = 14 * 24 * time.Hour

// newMailbox reads replies from the job-radar account (same login as sending).
func newMailbox(cfg *config.Config) (mailbox.Reader, error) {
	user, pass := strings.TrimSpace(os.Getenv(envEmailAddress)), os.Getenv(envEmailPassword)
	if user == "" || strings.TrimSpace(pass) == "" {
		return nil, fmt.Errorf("reading replies needs %s and %s in your .env file", envEmailAddress, envEmailPassword)
	}
	return &mailbox.IMAP{Host: cfg.Email.IMAPHost, Port: cfg.Email.IMAPPort, User: user, Password: pass}, nil
}

// syncResult says what reading replies did.
type syncResult struct {
	Added      int // new votes recorded
	Unreadable int // replies to our emails that held no vote we could read
}

// syncFeedback reads replies to job-radar's emails and records the 👍/👎 votes in
// them, telling the user what it recorded. Replies that aren't from one
// of your own addresses, don't answer a job-radar email, or contain no
// recognisable vote are skipped; nothing in a reply is ever acted on beyond
// recording a vote.
func syncFeedback(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, rd mailbox.Reader, now time.Time, out io.Writer) (syncResult, error) {
	allowed := map[string]bool{}
	var from []string
	for _, a := range cfg.Email.To {
		allowed[strings.ToLower(strings.TrimSpace(a))] = true
		from = append(from, strings.TrimSpace(a))
	}
	replies, err := rd.Replies(ctx, now.Add(-replyLookback), from)
	if err != nil {
		return syncResult{}, err
	}
	var res syncResult
	for _, r := range replies {
		if !allowed[r.From] {
			logger.Info("reply skipped: not from a configured recipient", "from_domain", domainOf(r.From))
			continue
		}
		var notifID int64
		var ids []string
		for _, ref := range r.Refs {
			id, items, ok, err := st.NotificationByMessageID(ctx, ref)
			if err != nil {
				return res, err
			}
			if ok {
				notifID, ids = id, items
				break
			}
		}
		if notifID == 0 {
			continue // a reply to something that isn't a job-radar email
		}
		votes := feedback.Parse(r.Body, len(ids))
		if len(votes) == 0 {
			logger.Info("reply had no recognisable votes", "message_id", r.MessageID)
			res.Unreadable++
			continue
		}
		for _, v := range votes {
			vote := store.VoteDown
			if v.Up {
				vote = store.VoteUp
			}
			postingID := ids[v.Number-1]
			before, hadBefore, err := st.LatestVote(ctx, postingID)
			if err != nil {
				return res, err
			}
			ok, err := st.SaveFeedback(ctx, store.Feedback{
				PostingID: postingID, Vote: vote, Source: "email",
				NotificationID: notifID, ReplyID: r.MessageID, At: now,
			})
			if err != nil {
				return res, err
			}
			if ok {
				res.Added++
				fmt.Fprintf(out, "Recorded %s\n", describeVote(ctx, st, postingID, vote, before, hadBefore))
			}
		}
	}
	return res, nil
}

// voteMark is the emoji for a vote.
func voteMark(vote string) string {
	if vote == store.VoteDown {
		return "👎"
	}
	return "👍"
}

// describeVote words one recorded vote, e.g. "👎 Acme — Staff Engineer (was 👍)".
func describeVote(ctx context.Context, st *store.Store, postingID, vote, before string, hadBefore bool) string {
	label := postingID
	if p, err := st.GetByID(ctx, postingID); err == nil {
		label = p.Company + " — " + p.Title
	}
	s := voteMark(vote) + " " + label
	switch {
	case hadBefore && before != vote:
		s += " (changed from " + voteMark(before) + ")"
	case hadBefore:
		s += " (same as before)"
	}
	return s
}

func domainOf(addr string) string {
	if _, d, ok := strings.Cut(addr, "@"); ok {
		return d
	}
	return ""
}

// maybeSyncFeedback runs at the start of `run`. A problem is reported, never fatal.
func maybeSyncFeedback(ctx context.Context, logger *slog.Logger, cfg *config.Config, st *store.Store, out io.Writer) {
	if !cfg.Email.Enabled() {
		return
	}
	rd, err := newMailbox(cfg)
	if err != nil {
		return // sending reports a missing login; stay quiet here
	}
	if _, err := syncFeedback(ctx, logger, cfg, st, rd, time.Now(), out); err != nil {
		logger.Warn("could not read replies", "error", err.Error())
		fmt.Fprintf(out, "! Couldn't read your email replies this time: %s\n", err)
	}
}

const feedbackUsage = `usage:
  job-radar feedback <posting-id> up|down    record a vote yourself
  job-radar feedback sync                    read votes from your email replies now`

// cmdFeedback records a vote from the terminal, or reads replies now.
func cmdFeedback(ctx context.Context, logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("feedback", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(*home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	switch {
	case fs.NArg() == 1 && fs.Arg(0) == "sync":
		if !cfg.Email.Enabled() {
			return fmt.Errorf("email is off: add your address under email.to in %s/config.yaml", *home)
		}
		rd, err := newMailbox(cfg)
		if err != nil {
			return err
		}
		res, err := syncFeedback(ctx, logger, cfg, st, rd, time.Now(), os.Stdout)
		if err != nil {
			return err
		}
		if res.Added == 0 {
			fmt.Println("No new votes in your replies.")
		}
		if res.Unreadable > 0 {
			fmt.Printf("%d repl%s to your job-radar emails had no vote I could read. Write the job's number and 👍 or 👎, e.g. \"1 👍, 3 👎\".\n",
				res.Unreadable, map[bool]string{true: "y", false: "ies"}[res.Unreadable == 1])
		}
		return nil
	case fs.NArg() == 2 && (fs.Arg(1) == "up" || fs.Arg(1) == "down"):
		if _, err := st.GetByID(ctx, fs.Arg(0)); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("no posting with id %q (run `job-radar list` to find ids)", fs.Arg(0))
			}
			return err
		}
		before, hadBefore, err := st.LatestVote(ctx, fs.Arg(0))
		if err != nil {
			return err
		}
		if _, err := st.SaveFeedback(ctx, store.Feedback{PostingID: fs.Arg(0), Vote: fs.Arg(1), Source: "terminal", At: time.Now()}); err != nil {
			return err
		}
		fmt.Println("Recorded", describeVote(ctx, st, fs.Arg(0), fs.Arg(1), before, hadBefore))
		return nil
	}
	return fmt.Errorf("%s", feedbackUsage)
}

// cmdReport prints a short summary of recent activity, including your votes.
func cmdReport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	days := fs.Int("days", 7, "how many days back to report")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *days < 1 {
		return fmt.Errorf("--days must be at least 1")
	}
	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(*home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	since := time.Now().Add(-time.Duration(*days) * 24 * time.Hour)
	sum, err := st.Summarise(ctx, since)
	if err != nil {
		return err
	}
	fmt.Printf("job-radar report — the last %d day(s)\n\n", *days)
	fmt.Printf("Runs: %d · new postings: %d · scored: %d\n", sum.Runs, sum.NewPostings, sum.Scored)
	fmt.Printf("Emails sent: %d alert(s), %d digest(s), %d problem notice(s) · %d strong match(es) emailed\n",
		sum.EmailsByKind[store.KindAlert], sum.EmailsByKind[store.KindDigest], sum.EmailsByKind[store.KindProblem], sum.StrongMatches)
	if cfg.Scorer == config.ScorerLLM || sum.AICostUSD > 0 {
		spent, err := st.AISpendSince(ctx, store.StartOfMonth(time.Now()))
		if err == nil {
			fmt.Printf("AI cost: ~$%.4f in this period · ~$%.4f this month of your $%.2f cap\n", sum.AICostUSD, spent, cfg.LLM.MonthlySpendCapUSD)
		}
	}
	rows, err := st.RecentFeedback(ctx, since)
	if err != nil {
		return err
	}
	fmt.Println()
	if len(rows) == 0 {
		fmt.Println("Your votes: none yet. Reply to an alert with e.g. \"1 👍, 3 👎\".")
		return nil
	}
	fmt.Println("Your votes (latest per job):")
	for _, v := range currentVotes(rows) {
		note := ""
		if v.ChangedFrom != "" {
			note = ", changed from " + voteMark(v.ChangedFrom)
		}
		fmt.Printf("  %s  %s — %s  (%s, %s%s)\n", voteMark(v.Vote), v.Company, v.Title, v.Source, v.At.Local().Format("2 Jan 15:04"), note)
	}
	return nil
}

// currentVote is a job's latest vote, and what it replaced if it was changed.
type currentVote struct {
	store.FeedbackRow
	ChangedFrom string
}

// currentVotes keeps one vote per job from rows listed newest first.
func currentVotes(rows []store.FeedbackRow) []currentVote {
	var out []currentVote
	index := map[string]int{}
	for _, r := range rows {
		i, seen := index[r.PostingID]
		if !seen {
			index[r.PostingID] = len(out)
			out = append(out, currentVote{FeedbackRow: r})
			continue
		}
		if out[i].ChangedFrom == "" && r.Vote != out[i].Vote {
			out[i].ChangedFrom = r.Vote // the vote just before the current one
		}
	}
	return out
}

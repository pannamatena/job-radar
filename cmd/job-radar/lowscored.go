package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/store"
)

// printLowScored lists recently seen jobs that scored below the digest line,
// from the database (no network), so a score that looks too low can be
// corrected with `job-radar feedback <id> up`.
func printLowScored(ctx context.Context, w io.Writer, cfg *config.Config, home string, days, limit int, now time.Time) error {
	st, err := store.Open(filepath.Join(home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	line := cfg.Rules.Thresholds.Digest
	rows, err := st.ScoredBetween(ctx, now.AddDate(0, 0, -days), 1, line-1, limit)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Fprintf(w, "No jobs scored below %d in the last %d day(s).\n", line, days)
		return nil
	}
	printScoredTable(ctx, w, st, rows, "WHY IT SCORED LOW")
	fmt.Fprintf(w, "\n%d job(s) first seen in the last %d day(s), scored below %d (best first; --limit and --days change this).\n", len(rows), days, line)
	fmt.Fprintln(w, "Think one deserved more? Run: job-radar feedback <ID> up")
	fmt.Fprintln(w, "To turn it into a test case for the scorer: job-radar eval draft <ID>")
	return nil
}

// printScored lists stored jobs scoring minScore or more (from the database,
// no network), best first, so scores of 3 and up can be seen without waiting
// for the digest.
func printScored(ctx context.Context, w io.Writer, home string, minScore, days, limit int, now time.Time) error {
	if minScore < 1 || minScore > 5 {
		return fmt.Errorf("--min must be between 1 and 5")
	}
	st, err := store.Open(filepath.Join(home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	rows, err := st.ScoredBetween(ctx, now.AddDate(0, 0, -days), minScore, 5, limit)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Fprintf(w, "No jobs scored %d or more in the last %d day(s).\n", minScore, days)
		return nil
	}
	printScoredTable(ctx, w, st, rows, "WHY / GAP")
	fmt.Fprintf(w, "\n%d job(s) first seen in the last %d day(s), scored %d or more (best first; --min, --limit and --days change this).\n", len(rows), days, minScore)
	fmt.Fprintln(w, "See one in full: job-radar show <ID>")
	return nil
}

// printScoredTable prints the shared table. The last column is the gap line.
func printScoredTable(ctx context.Context, w io.Writer, st *store.Store, rows []store.ScoredPosting, lastHeading string) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SCORE\tBY\tVOTE\tID\tCOMPANY\tTITLE\t"+lastHeading)
	for _, sp := range rows {
		vote := ""
		if v, ok, err := st.LatestVote(ctx, sp.Posting.ID); err == nil && ok {
			vote = voteMark(v)
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", sp.Score, sp.Scorer, vote, sp.Posting.ID, sp.Posting.Company, sp.Posting.Title+closedMark(sp), sp.Gap)
	}
	tw.Flush()
}

// closedMark flags a job that has gone from its company's board.
func closedMark(sp store.ScoredPosting) string {
	if sp.Closed {
		return " [closed]"
	}
	return ""
}

// cmdShow prints one stored job's score and the reasons behind it.
func cmdShow(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: job-radar show <posting-id>   (find ids with `job-radar list --scored --min 1`)")
	}
	st, err := store.Open(filepath.Join(*home, "radar.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	return printShow(ctx, os.Stdout, st, fs.Arg(0))
}

func printShow(ctx context.Context, w io.Writer, st *store.Store, id string) error {
	sp, ok, err := st.ScoredByID(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		p, err := st.GetByID(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("no posting with id %q (run `job-radar list --scored --min 1` to find ids)", id)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s — %s\n%s\nNot scored yet.\n", p.Company, p.Title, p.URL)
		return nil
	}
	p := sp.Posting
	fmt.Fprintf(w, "%s — %s\n%s · %s · %s\n%s\n\n", p.Company, p.Title, p.Location, p.Remote, sp.FirstSeen.Local().Format("first seen 2 Jan"), p.URL)
	if sp.Closed {
		fmt.Fprintln(w, "Status: closed (no longer on the company's board)")
	}
	fmt.Fprintf(w, "Score:  %d (by the %s scorer)\nTrack:  %s · work model: %s\nWhy:    %s\nGap:    %s\n", sp.Score, sp.Scorer, sp.Track, sp.WorkModel, sp.Why, sp.Gap)
	if len(sp.Flags) > 0 {
		fmt.Fprintf(w, "Flags:  %s\n", strings.Join(sp.Flags, ", "))
	}
	if v, ok, err := st.LatestVote(ctx, id); err == nil && ok {
		fmt.Fprintf(w, "Your vote: %s\n", voteMark(v))
	}
	return nil
}

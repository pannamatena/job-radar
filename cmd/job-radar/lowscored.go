package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
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
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "SCORE\tVOTE\tID\tCOMPANY\tTITLE\tWHY IT SCORED LOW")
	for _, sp := range rows {
		vote := ""
		if v, ok, err := st.LatestVote(ctx, sp.Posting.ID); err == nil && ok {
			vote = voteMark(v)
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n", sp.Score, vote, sp.Posting.ID, sp.Posting.Company, sp.Posting.Title, sp.Gap)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d job(s) first seen in the last %d day(s), scored below %d (best first; --limit and --days change this).\n", len(rows), days, line)
	fmt.Fprintln(w, "Think one deserved more? Run: job-radar feedback <ID> up")
	fmt.Fprintln(w, "To turn it into a test case for the scorer: job-radar eval draft <ID>")
	return nil
}

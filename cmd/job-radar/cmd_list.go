package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/filter"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/sources"
	"github.com/pannamatena/job-radar/internal/store"
	"github.com/pannamatena/job-radar/internal/version"
)

// cmdList fetches current postings for the configured companies, runs the
// pre-filter, records them in the store (so re-runs can tell new from seen),
// and prints the kept ones. With --companies it just lists the watch-list.
func cmdList(ctx context.Context, logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	companiesOnly := fs.Bool("companies", false, "list the watch-list companies instead of their postings")
	showAll := fs.Bool("all", false, "also show postings the pre-filter dropped")
	noStore := fs.Bool("no-store", false, "don't record this run in the database (read-only)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*home)
	if err != nil {
		return err
	}
	if *companiesOnly {
		printCompanies(cfg)
		return nil
	}

	client := httpclient.New(httpclient.Config{UserAgent: version.UserAgent()})
	pre := filter.New(cfg.PreFilter)

	var kept, dropped []posting.Posting
	started := time.Now()
	for _, co := range cfg.Companies {
		src, err := sources.For(client, co)
		if err != nil {
			if errors.Is(err, sources.ErrNoPublicAPI) {
				fmt.Fprintf(os.Stderr, "  · %s: no public API (covered by job-alert emails in phase 5) — skipping\n", co.Name)
			} else {
				fmt.Fprintf(os.Stderr, "  ! %s: %s\n", co.Name, err)
			}
			continue
		}
		postings, err := src.Fetch(ctx)
		if err != nil {
			logger.Warn("source failed", "company", co.Name, "error", err.Error())
			fmt.Fprintf(os.Stderr, "  ! %s: %s\n", co.Name, err)
			continue
		}
		logger.Info("fetched", "company", co.Name, "source", src.Name(), "postings", len(postings))
		for _, p := range postings {
			if pre.Check(p).Keep {
				kept = append(kept, p)
			} else {
				dropped = append(dropped, p)
			}
		}
	}

	// Record what we kept so future runs can distinguish new from already-seen.
	newIDs := map[string]bool{}
	if !*noStore {
		ids, err := persist(ctx, *home, kept, started, logger)
		if err != nil {
			return err
		}
		for _, id := range ids {
			newIDs[id] = true
		}
	}

	toShow := kept
	if *showAll {
		toShow = append(append([]posting.Posting{}, kept...), dropped...)
	}
	printPostings(toShow, newIDs)

	total, perHost := client.Stats()
	fmt.Printf("\nFetched %d kept, %d dropped by pre-filter", len(kept), len(dropped))
	if !*noStore {
		fmt.Printf(" · %d new, %d already seen", len(newIDs), len(kept)-len(newIDs))
	}
	fmt.Println(".")
	logger.Info("run complete", "kept", len(kept), "dropped", len(dropped), "new", len(newIDs), "requests", total, "hosts", len(perHost))
	return nil
}

// persist opens the store in the config dir, saves the kept postings and logs
// the run, returning the ids that were new to the user.
func persist(ctx context.Context, home string, kept []posting.Posting, started time.Time, logger *slog.Logger) ([]string, error) {
	st, err := store.Open(filepath.Join(home, "radar.db"))
	if err != nil {
		return nil, err
	}
	defer st.Close()

	newIDs, err := st.SaveFetched(ctx, kept, time.Now())
	if err != nil {
		return nil, err
	}
	if _, err := st.RecordRun(ctx, store.RunStats{
		StartedAt:   started,
		FinishedAt:  time.Now(),
		NewPostings: len(newIDs),
	}); err != nil {
		logger.Warn("could not record run", "error", err.Error())
	}
	return newIDs, nil
}

func printCompanies(cfg *config.Config) {
	if len(cfg.Companies) == 0 {
		fmt.Println("Your watch-list is empty. Add companies to", config.ConfigPath(cfg.Home))
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "COMPANY\tATS\tIDENTIFIER\tNOTES")
	for _, co := range cfg.Companies {
		id := co.Slug
		notes := ""
		if co.ATS == config.ATSNone {
			id = co.CareersURL
			notes = "no public API — job-alert emails (phase 5)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", co.Name, co.ATS, id, notes)
	}
	tw.Flush()
}

func printPostings(ps []posting.Posting, newIDs map[string]bool) {
	if len(ps) == 0 {
		fmt.Println("No postings found. (Check your company slugs and filters.)")
		return
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].Company != ps[j].Company {
			return ps[i].Company < ps[j].Company
		}
		return ps[i].Title < ps[j].Title
	})

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "\tCOMPANY\tTITLE\tLOCATION\tREMOTE")
	for _, p := range ps {
		marker := ""
		if newIDs[p.ID] {
			marker = "NEW"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", marker, p.Company, p.Title, p.Location, p.Remote)
	}
	tw.Flush()
}

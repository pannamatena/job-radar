package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/sources"
	"github.com/pannamatena/job-radar/internal/version"
)

// cmdList fetches and prints current postings for the configured companies.
// With --companies it just lists the watch-list, including companies with no
// public API so the coverage gap is visible.
func cmdList(ctx context.Context, logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	companiesOnly := fs.Bool("companies", false, "list the watch-list companies instead of their postings")
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

	var all []posting.Posting
	for _, co := range cfg.Companies {
		switch co.ATS {
		case config.ATSGreenhouse:
			src := sources.NewGreenhouse(client, co)
			postings, err := src.Fetch(ctx)
			if err != nil {
				// One company failing shouldn't stop the rest.
				logger.Warn("source failed", "company", co.Name, "error", err.Error())
				fmt.Fprintf(os.Stderr, "  ! %s: %s\n", co.Name, err)
				continue
			}
			logger.Info("fetched", "company", co.Name, "source", src.Name(), "postings", len(postings))
			all = append(all, postings...)
		case config.ATSLever, config.ATSAshby:
			fmt.Fprintf(os.Stderr, "  · %s: %s sources arrive in phase 2 — skipping for now\n", co.Name, co.ATS)
		case config.ATSNone:
			fmt.Fprintf(os.Stderr, "  · %s: no public API (covered by job-alert emails in phase 5) — skipping\n", co.Name)
		}
	}

	printPostings(all)
	total, perHost := client.Stats()
	logger.Info("run complete", "postings", len(all), "requests", total, "hosts", len(perHost))
	return nil
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

func printPostings(ps []posting.Posting) {
	if len(ps) == 0 {
		fmt.Println("No postings found. (Check your company slugs and filters.)")
		return
	}
	// Stable order: company, then title.
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].Company != ps[j].Company {
			return ps[i].Company < ps[j].Company
		}
		return ps[i].Title < ps[j].Title
	})

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "COMPANY\tTITLE\tLOCATION\tREMOTE\tURL")
	for _, p := range ps {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Company, p.Title, p.Location, p.Remote, p.URL)
	}
	tw.Flush()
	fmt.Printf("\n%d postings.\n", len(ps))
}

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/sources"
	"github.com/pannamatena/job-radar/internal/version"
)

// cmdDiscover takes a company's careers-page URL, fetches it, works out which
// job board (ATS) it uses and prints a ready-to-paste config entry. If the
// company uses an ATS with no public feed, it says so.
func cmdDiscover(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	name := fs.String("name", "", "company name to put in the printed config entry")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: job-radar discover [--name \"Company\"] <careers-page-url>")
	}
	pageURL := fs.Arg(0)

	client := httpclient.New(httpclient.Config{UserAgent: version.UserAgent()})
	resp, err := client.Get(ctx, pageURL)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", pageURL, err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("fetching %s: HTTP %d", pageURL, resp.StatusCode)
	}

	det := sources.Detect(pageURL, resp.Body)
	switch {
	case det.Found():
		company := *name
		if company == "" {
			company = "Company Name"
		}
		verifyBoard(ctx, client, det)
		fmt.Printf("\nAdd this to the `companies:` list in your config.yaml:\n\n")
		fmt.Print(configEntry(company, det))
	case det.Unsupported != "":
		fmt.Printf("This looks like a %s board, which has no public feed job-radar can read.\n", det.Unsupported)
		fmt.Printf("Add it as `ats: none` and cover it with that company's own job-alert emails (phase 5):\n\n")
		fmt.Printf("  - name: %s\n    ats: none\n    careers_url: \"%s\"\n", orDefault(*name, "Company Name"), pageURL)
	default:
		fmt.Fprintf(os.Stderr, "Couldn't detect a supported job board on %s.\n", pageURL)
		fmt.Fprintln(os.Stderr, "The page may load its jobs with JavaScript. Try the direct board URL")
		fmt.Fprintln(os.Stderr, "(e.g. the boards.greenhouse.io / jobs.lever.co / jobs.ashbyhq.com link), or")
		fmt.Fprintln(os.Stderr, "add the company as `ats: none` with its careers_url.")
		return fmt.Errorf("no supported board detected")
	}
	return nil
}

// verifyBoard fetches the detected board once to confirm the slug works and
// reports how many postings it currently has.
func verifyBoard(ctx context.Context, client *httpclient.Client, det sources.Detection) {
	src, err := sources.For(client, config.Company{Name: "verify", ATS: det.ATS, Slug: det.Slug})
	if err != nil {
		return
	}
	ps, err := src.Fetch(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  (detected %s/%s, but couldn't verify it: %s)\n", det.ATS, det.Slug, err)
		return
	}
	fmt.Printf("✓ Detected the %s board \"%s\" with %d current postings.\n", det.ATS, det.Slug, len(ps))
}

func configEntry(company string, det sources.Detection) string {
	slug := det.Slug
	// Quote slugs that contain a space (e.g. an Ashby org "Acme Labs").
	quoted := slug
	if containsSpace(slug) {
		quoted = "\"" + slug + "\""
	}
	return fmt.Sprintf("  - name: %s\n    ats: %s\n    slug: %s\n", company, det.ATS, quoted)
}

func containsSpace(s string) bool {
	for _, r := range s {
		if r == ' ' {
			return true
		}
	}
	return false
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

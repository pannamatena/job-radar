package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	jobradar "github.com/pannamatena/job-radar"
	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/version"
)

// cmdInit creates the user's config directory from the bundled example persona,
// after showing a summary of the security-and-privacy notice and asking the
// user to confirm they've read it (BUILD_PLAN.md §9a point 7).
func cmdInit(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to create")
	yes := fs.Bool("yes", false, "confirm you have read the security-and-privacy notice (skips the prompt)")
	force := fs.Bool("force", false, "overwrite an existing config directory (careful: this is your personal data)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	configPath := config.ConfigPath(*home)
	if _, err := os.Stat(configPath); err == nil && !*force {
		return fmt.Errorf("a config already exists at %s\n\nNothing was changed. Use --force to overwrite it (this replaces your personal settings).", configPath)
	}

	// Show the notice and get confirmation before creating anything.
	if !*yes {
		confirmed, err := confirmNotice(os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("\nNo problem — nothing was created. Re-run `job-radar init` when you're ready.")
			return nil
		}
	} else {
		printNoticeSummary(os.Stdout)
		fmt.Println("\n(You passed --yes, confirming you've read the notice above.)")
	}

	if err := copyExamples(*home); err != nil {
		return fmt.Errorf("creating your config in %s: %w", *home, err)
	}

	fmt.Printf(`
✓ Created your config in %s

What's there:
  config.yaml   your companies and location rules — edit this first
  profile.md    your career profile (currently the example persona)
  rubric.md     how roles should be scored for you
  evals/        example test cases for later

Next steps:
  1. Open %s and replace the example companies with ones you want to watch.
  2. Replace profile.md and rubric.md with your own details.
  3. Run: job-radar list

Secrets (email app password, AI API key) never go in these files — they go in a
.env file or environment variables. See docs/security-and-privacy.md.
`, *home, configPath)
	return nil
}

// confirmNotice prints the notice summary and waits for the user to type "y".
func confirmNotice(in *os.File, out *os.File) (bool, error) {
	printNoticeSummary(out)
	fmt.Fprint(out, "\nHave you read the full notice and do you want to continue? [y/N]: ")

	// If stdin isn't interactive, we can't prompt; tell the user to use --yes.
	info, _ := in.Stat()
	if info != nil && (info.Mode()&os.ModeCharDevice) == 0 {
		return false, fmt.Errorf("init needs confirmation but stdin isn't interactive; re-run with --yes once you've read %s",
			noticeURL())
	}

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func printNoticeSummary(out *os.File) {
	fmt.Fprintf(out, "Before you set up job-radar, please read the security & privacy notice:\n  %s\n\n", noticeURL())
	fmt.Fprint(out, shortVersion(jobradar.SecurityNotice))
}

func noticeURL() string {
	return version.RepoURL + "/blob/main/docs/security-and-privacy.md"
}

// shortVersion extracts the "## The short version" section from the notice so
// what we show always matches the doc. Falls back to a minimal message.
func shortVersion(notice string) string {
	const marker = "## The short version"
	i := strings.Index(notice, marker)
	if i < 0 {
		return "Your data stays in your own files and accounts; job-radar sends data only to services you switch on.\n"
	}
	rest := notice[i+len(marker):]
	// The section ends at the next horizontal rule.
	if end := strings.Index(rest, "\n---"); end >= 0 {
		rest = rest[:end]
	}
	return "The short version:\n" + strings.Trim(rest, "\n ") + "\n"
}

// copyExamples writes the embedded example persona into the home directory,
// marking the security notice as accepted in the created config.yaml.
func copyExamples(home string) error {
	return fs.WalkDir(jobradar.ExamplesFS, "examples", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("examples", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(home, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0o700)
		}

		data, err := jobradar.ExamplesFS.ReadFile(path)
		if err != nil {
			return err
		}
		if rel == "config.yaml" {
			data = acceptNotice(data)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o600)
	})
}

// acceptNotice flips the example config's notice.accepted to true and records
// the notice version, preserving the file's comments.
func acceptNotice(configYAML []byte) []byte {
	s := string(configYAML)
	accepted := "  accepted: true\n  version: \"" + jobradar.NoticeVersion + "\""
	if strings.Contains(s, "  accepted: false") {
		s = strings.Replace(s, "  accepted: false", accepted, 1)
	} else {
		s += "\nnotice:\n" + accepted + "\n"
	}
	return []byte(s)
}

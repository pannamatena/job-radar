package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/store"
)

const evalDraftUsage = `usage:
  job-radar eval draft <posting-id>...    make a draft eval case from a stored job
  job-radar eval draft --from-votes       make drafts for every job where your vote disagrees with its score`

// cmdEvalDraft copies jobs from the database into draft eval cases. The label
// is left blank on purpose: only the owner writes labels.
func cmdEvalDraft(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("eval draft", flag.ContinueOnError)
	home := fs.String("home", config.Home(), "config directory to read")
	fromVotes := fs.Bool("from-votes", false, "draft a case for every job where your latest vote disagrees with its score")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *fromVotes == (fs.NArg() > 0) {
		return fmt.Errorf("%s", evalDraftUsage)
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

	ids := fs.Args()
	if *fromVotes {
		if ids, err = disagreements(ctx, st, cfg.Rules.Thresholds.Alert); err != nil {
			return err
		}
		if len(ids) == 0 {
			fmt.Println("No votes disagree with a score right now. (A 👍 on a job below the alert line, or a 👎 on one at or above it, counts.)")
			return nil
		}
	}
	casesDir := filepath.Join(*home, "evals", "cases")
	made := 0
	for _, id := range ids {
		name, created, err := draftCase(ctx, st, casesDir, id)
		switch {
		case errors.Is(err, store.ErrNotFound):
			fmt.Printf("  skipped %s: no such job (run `job-radar list --low` or `job-radar report` to find ids)\n", id)
		case err != nil:
			return err
		case !created:
			fmt.Printf("  already have a case for %s (%s)\n", id, name)
		default:
			fmt.Printf("  drafted %s\n", filepath.Join(casesDir, name))
			made++
		}
	}
	if made > 0 {
		fmt.Printf("\n%d draft case(s) made. Open each label.yaml, fill in YOUR judgement (score, track, work_model …),\nthen change status: draft to status: confirmed. Delete a folder if you don't want that case.\n", made)
	}
	return nil
}

// disagreements finds jobs whose latest vote goes against their latest score:
// a 👍 below the alert line (it deserved more) or a 👎 at or above it.
func disagreements(ctx context.Context, st *store.Store, alert int) ([]string, error) {
	rows, err := st.RecentFeedback(ctx, time.Time{})
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, v := range currentVotes(rows) {
		score, ok, err := st.LatestScore(ctx, v.PostingID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if (v.Vote == store.VoteUp && score < alert) || (v.Vote == store.VoteDown && score >= alert) {
			ids = append(ids, v.PostingID)
		}
	}
	return ids, nil
}

var slugRun = regexp.MustCompile(`[^a-z0-9]+`)

// caseName makes a folder name like "acme-staff-engineer".
func caseName(company, title string) string {
	s := strings.Trim(slugRun.ReplaceAllString(strings.ToLower(company+" "+title), "-"), "-")
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	if s == "" {
		s = "case"
	}
	return s
}

// draftCase writes one draft case folder for a stored posting. It reports false
// when a case for that posting already exists (nothing is overwritten).
func draftCase(ctx context.Context, st *store.Store, casesDir, postingID string) (string, bool, error) {
	p, err := st.GetByID(ctx, postingID)
	if err != nil {
		return "", false, err
	}
	name := caseName(p.Company, p.Title)
	dir := filepath.Join(casesDir, name)
	if _, err := os.Stat(dir); err == nil {
		return name, false, nil
	}
	score, hasScore, _ := st.LatestScore(ctx, postingID)
	vote, hasVote, _ := st.LatestVote(ctx, postingID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(filepath.Join(dir, "ad.txt"), []byte(adText(p)), 0o644); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(filepath.Join(dir, "label.yaml"), []byte(draftLabel(p, score, hasScore, vote, hasVote)), 0o644); err != nil {
		return "", false, err
	}
	return name, true, nil
}

// adText is the ad in the form eval cases use: a "Title — Company (Location)"
// first line, then the text.
func adText(p posting.Posting) string {
	first := p.Title + " — " + p.Company
	if p.Location != "" {
		first += " (" + p.Location + ")"
	}
	return first + "\n\n" + strings.TrimSpace(p.DescriptionText) + "\n"
}

func quote(s string) string {
	b, _ := json.Marshal(s) // a JSON string is also a valid YAML string
	return string(b)
}

// draftLabel is the label.yaml for a draft: facts about the job filled in,
// every judgement left for the owner.
func draftLabel(p posting.Posting, score int, hasScore bool, vote string, hasVote bool) string {
	var b strings.Builder
	b.WriteString("# DRAFT. Fill in YOUR judgement below, then change status to confirmed.\n")
	b.WriteString("# job-radar never writes a label for you.\n")
	if hasScore {
		fmt.Fprintf(&b, "# For reference: job-radar scored this %d/5", score)
		if hasVote {
			fmt.Fprintf(&b, " and your latest vote on it was %s", voteMark(vote))
		}
		b.WriteString(". That is not your label.\n")
	}
	b.WriteString("status: draft\n")
	fmt.Fprintf(&b, "company: %s\ntitle: %s\n", quote(p.Company), quote(p.Title))
	if p.URL != "" {
		fmt.Fprintf(&b, "source: %s\n", quote(p.URL))
	}
	b.WriteString(`score:              # 1-5, how good a fit it is for you
track:              # one of your tracks from config.yaml, or none
work_model:         # remote | hybrid | onsite | not_stated
office: ""
flags: []
note: >
  One line on why you gave this score.
why_tricky: ""
`)
	return b.String()
}

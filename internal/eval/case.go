// Package eval measures how well a scorer agrees with the owner's own
// judgement. A "case" is one job ad plus the human's hand label. The runner
// scores every confirmed case, and the metrics say how often the scorer was
// right, with the most weight on the alert threshold (ADR-0011).
//
// Labels are always the human's. Nothing in this package writes or edits a
// label.
package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pannamatena/job-radar/internal/posting"
)

// Case statuses. Only confirmed cases count towards the metrics.
const (
	StatusDraft     = "draft"
	StatusConfirmed = "confirmed"
)

// Label is the human's judgement of one ad (the label.yaml file).
type Label struct {
	Status     string   `yaml:"status"`
	Source     string   `yaml:"source"`
	Company    string   `yaml:"company"`
	Title      string   `yaml:"title"`
	Score      int      `yaml:"score"`
	Track      string   `yaml:"track"`
	WorkModel  string   `yaml:"work_model"`
	Office     string   `yaml:"office"`
	Flags      []string `yaml:"flags"`
	Note       string   `yaml:"note"`
	WhyTricky  string   `yaml:"why_tricky"`
	AdLocation string   `yaml:"ad_location"` // optional: overrides the location read from the ad's first line
}

// Case is one folder: ad.txt plus label.yaml.
type Case struct {
	Name  string // the folder name
	Label Label
	Ad    string // the ad text
}

// LoadResult is what was found in a cases folder.
type LoadResult struct {
	Confirmed []Case
	Drafts    []string // names of cases skipped because they are drafts
}

var validWorkModels = map[string]bool{"remote": true, "hybrid": true, "onsite": true, "not_stated": true}

// LoadCases reads every case folder under dir. Drafts are listed but not
// returned as cases. A confirmed case with an incomplete label is an error
// that names the case, so a half-filled label can't silently skew the numbers.
func LoadCases(dir string, tracks, flags []string) (LoadResult, error) {
	var out LoadResult
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, fmt.Errorf("no eval cases folder at %s — create one folder per ad, each with ad.txt and label.yaml (see docs/evaluation.md)", dir)
		}
		return out, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := loadCase(filepath.Join(dir, e.Name()))
		if err != nil {
			return out, fmt.Errorf("case %q: %w", e.Name(), err)
		}
		if c.Label.Status != StatusConfirmed {
			out.Drafts = append(out.Drafts, c.Name)
			continue
		}
		if err := c.Label.validate(tracks, flags); err != nil {
			return out, fmt.Errorf("case %q is confirmed but its label is incomplete: %w", c.Name, err)
		}
		out.Confirmed = append(out.Confirmed, c)
	}
	sort.Strings(out.Drafts)
	sort.Slice(out.Confirmed, func(i, j int) bool { return out.Confirmed[i].Name < out.Confirmed[j].Name })
	return out, nil
}

func loadCase(dir string) (Case, error) {
	c := Case{Name: filepath.Base(dir)}
	raw, err := os.ReadFile(filepath.Join(dir, "label.yaml"))
	if err != nil {
		return c, fmt.Errorf("missing label.yaml: %w", err)
	}
	if err := yaml.Unmarshal(raw, &c.Label); err != nil {
		return c, fmt.Errorf("label.yaml is not valid YAML: %w", err)
	}
	ad, err := os.ReadFile(filepath.Join(dir, "ad.txt"))
	if err != nil {
		return c, fmt.Errorf("missing ad.txt: %w", err)
	}
	c.Ad = string(ad)
	return c, nil
}

func (l Label) validate(tracks, flags []string) error {
	var problems []string
	if l.Score < 1 || l.Score > 5 {
		problems = append(problems, "score must be 1–5")
	}
	if l.Track != "none" && !contains(tracks, l.Track) {
		problems = append(problems, fmt.Sprintf("track %q must be one of %s or none", l.Track, strings.Join(tracks, ", ")))
	}
	for _, f := range l.Flags {
		if !contains(flags, f) {
			problems = append(problems, fmt.Sprintf("flag %q is not a known flag; add it under rules.red_flags in config.yaml (an empty list is fine) or use one of: %s", f, strings.Join(flags, ", ")))
		}
	}
	if !validWorkModels[l.WorkModel] {
		problems = append(problems, "work_model must be remote, hybrid, onsite or not_stated")
	}
	if strings.TrimSpace(l.Company) == "" || strings.TrimSpace(l.Title) == "" {
		problems = append(problems, "company and title are required")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

// Posting turns a case into the posting the scorers see. The ad's first line
// is expected to look like "Title — Company (Location)", which is how the
// boards present it; the location in the brackets is all the scorers get, just
// as with a real posting. The human's label is never part of the input.
func (c Case) Posting() posting.Posting {
	body := c.Ad
	loc := c.Label.AdLocation
	if first, rest, ok := strings.Cut(c.Ad, "\n"); ok {
		if l := locationInHeader(first); l != "" {
			if loc == "" {
				loc = l
			}
			body = strings.TrimLeft(rest, "\n")
		}
	}
	return posting.Posting{
		ID:              "eval:" + c.Name,
		Source:          "eval",
		Company:         c.Label.Company,
		Title:           c.Label.Title,
		Location:        loc,
		Remote:          remoteFromLocation(loc),
		URL:             c.Label.Source,
		DescriptionText: strings.TrimSpace(body),
	}
}

// locationInHeader returns the text in the last pair of brackets of a header
// line like "Senior Engineer — Acme (Dublin, hybrid)", or "" if the line has
// no such shape.
func locationInHeader(line string) string {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, " — ") || !strings.HasSuffix(line, ")") {
		return ""
	}
	// Walk back from the final ")" to its matching "(", so nested brackets
	// like "(Ireland (Hybrid))" give "Ireland (Hybrid)".
	depth := 0
	for i := len(line) - 1; i >= 0; i-- {
		switch line[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return strings.TrimSpace(line[i+1 : len(line)-1])
			}
		}
	}
	return ""
}

// remoteFromLocation reads the work model off a board-style location string.
func remoteFromLocation(loc string) posting.RemoteKind {
	l := strings.ToLower(loc)
	switch {
	case strings.Contains(l, "hybrid"):
		return posting.RemoteHybrid
	case strings.Contains(l, "remote"):
		return posting.RemoteYes
	}
	return posting.RemoteUnknown
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

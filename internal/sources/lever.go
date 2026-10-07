package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/posting"
)

// LeverBaseURL is the public Lever postings API. Verified against the live API
// on 2026-10-07: GET /v0/postings/{slug}?mode=json returns a JSON array of
// postings, each with text (title), categories, workplaceType and plain-text
// description fields.
const LeverBaseURL = "https://api.lever.co/v0/postings"

const sourceLever = "lever"

// Lever fetches postings from one company's Lever board.
type Lever struct {
	client  *httpclient.Client
	company config.Company
	baseURL string // overridable in tests
}

// NewLever builds a Lever source for a company.
func NewLever(client *httpclient.Client, company config.Company) *Lever {
	return &Lever{client: client, company: company}
}

// Name identifies the source, e.g. "lever:gopuff".
func (l *Lever) Name() string { return sourceLever + ":" + l.company.Slug }

func (l *Lever) base() string {
	if l.baseURL != "" {
		return l.baseURL
	}
	return LeverBaseURL
}

// leverJob mirrors the parts of a Lever posting we use.
type leverJob struct {
	ID               string          `json:"id"`
	Text             string          `json:"text"` // the title
	HostedURL        string          `json:"hostedUrl"`
	WorkplaceType    string          `json:"workplaceType"` // remote | hybrid | onsite
	Country          string          `json:"country"`
	CreatedAt        int64           `json:"createdAt"` // epoch milliseconds
	Categories       leverCategories `json:"categories"`
	DescriptionPlain string          `json:"descriptionPlain"`
	AdditionalPlain  string          `json:"additionalPlain"`
	Lists            []leverList     `json:"lists"`
}

type leverCategories struct {
	Commitment   string   `json:"commitment"`
	Department   string   `json:"department"`
	Team         string   `json:"team"`
	Location     string   `json:"location"`
	AllLocations []string `json:"allLocations"`
}

type leverList struct {
	Text    string `json:"text"`    // section heading, e.g. "Responsibilities"
	Content string `json:"content"` // HTML list markup
}

// Fetch retrieves the board, applies filters and normalises matching jobs.
func (l *Lever) Fetch(ctx context.Context) ([]posting.Posting, error) {
	if l.company.Slug == "" {
		return nil, fmt.Errorf("lever: company %q has no slug", l.company.Name)
	}
	endpoint := fmt.Sprintf("%s/%s?mode=json", l.base(), url.PathEscape(l.company.Slug))

	resp, err := l.client.Get(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("lever %q: %w", l.company.Name, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("lever %q: unexpected HTTP %d (is the slug %q correct?)", l.company.Name, resp.StatusCode, l.company.Slug)
	}

	var jobs []leverJob
	if err := json.Unmarshal(resp.Body, &jobs); err != nil {
		return nil, fmt.Errorf("lever %q: parsing response: %w", l.company.Name, err)
	}

	out := make([]posting.Posting, 0, len(jobs))
	for i := range jobs {
		job := jobs[i]
		if !passesFilters(l.filterMeta(job), l.company.Filters) {
			continue
		}
		out = append(out, l.normalise(job))
	}
	return out, nil
}

func (l *Lever) filterMeta(job leverJob) filterMeta {
	// Department and team both count as "departments" for matching.
	depts := []string{job.Categories.Department}
	if job.Categories.Team != "" {
		depts = append(depts, job.Categories.Team)
	}
	return filterMeta{
		Departments: depts,
		Location:    leverLocation(job),
		Remote:      remoteFromWorkplaceType(job.WorkplaceType),
	}
}

func (l *Lever) normalise(job leverJob) posting.Posting {
	raw, _ := json.Marshal(job)
	return posting.Posting{
		ID:              posting.MakeID(sourceLever, l.company.Slug+"/"+job.ID),
		Source:          sourceLever,
		Company:         l.company.Name,
		Title:           strings.TrimSpace(job.Text),
		Location:        leverLocation(job),
		Remote:          remoteFromWorkplaceType(job.WorkplaceType),
		URL:             job.HostedURL,
		DescriptionText: leverDescription(job),
		PostedAt:        epochMillisToTime(job.CreatedAt),
		RawJSON:         raw,
	}
}

// leverLocation prefers the single location, falling back to all locations.
func leverLocation(job leverJob) string {
	if loc := strings.TrimSpace(job.Categories.Location); loc != "" {
		return loc
	}
	return strings.Join(job.Categories.AllLocations, "; ")
}

// leverDescription assembles the full ad text from Lever's parts: the plain
// description, each list section (heading + its HTML turned to text), and any
// additional plain text.
func leverDescription(job leverJob) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(job.DescriptionPlain))
	for _, lst := range job.Lists {
		b.WriteString("\n\n")
		if lst.Text != "" {
			b.WriteString(lst.Text)
			b.WriteString("\n")
		}
		b.WriteString(posting.HTMLToText(lst.Content))
	}
	if add := strings.TrimSpace(job.AdditionalPlain); add != "" {
		b.WriteString("\n\n")
		b.WriteString(add)
	}
	return strings.TrimSpace(b.String())
}

// remoteFromWorkplaceType maps Lever/Ashby-style workplace labels to RemoteKind.
func remoteFromWorkplaceType(wt string) posting.RemoteKind {
	switch strings.ToLower(strings.TrimSpace(wt)) {
	case "remote":
		return posting.RemoteYes
	case "hybrid":
		return posting.RemoteHybrid
	case "onsite", "on-site", "on site":
		return posting.RemoteNo
	default:
		return posting.RemoteUnknown
	}
}

func epochMillisToTime(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

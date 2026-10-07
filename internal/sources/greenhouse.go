package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/posting"
)

// GreenhouseBaseURL is the public Greenhouse job-board API. The board token
// (the company's "slug") is substituted in. Verified against the live API on
// 2026-10-04: GET /v1/boards/{token}/jobs?content=true returns {jobs:[...]}
// with per-job location, departments, offices and HTML-escaped content.
const GreenhouseBaseURL = "https://boards-api.greenhouse.io/v1/boards"

const sourceGreenhouse = "greenhouse"

// Greenhouse fetches postings from one company's Greenhouse board.
type Greenhouse struct {
	client  *httpclient.Client
	company config.Company
	baseURL string // overridable in tests; empty means GreenhouseBaseURL
}

// NewGreenhouse builds a Greenhouse source for a company. The company's ATS
// must be "greenhouse" and it must have a slug (its board token).
func NewGreenhouse(client *httpclient.Client, company config.Company) *Greenhouse {
	return &Greenhouse{client: client, company: company}
}

// Name identifies the source, e.g. "greenhouse:anthropic".
func (g *Greenhouse) Name() string {
	return sourceGreenhouse + ":" + g.company.Slug
}

func (g *Greenhouse) base() string {
	if g.baseURL != "" {
		return g.baseURL
	}
	return GreenhouseBaseURL
}

// ghResponse / ghJob mirror the parts of the Greenhouse API we use.
type ghResponse struct {
	Jobs []ghJob `json:"jobs"`
}

type ghJob struct {
	ID             int64      `json:"id"`
	Title          string     `json:"title"`
	AbsoluteURL    string     `json:"absolute_url"`
	Location       ghLocation `json:"location"`
	Content        string     `json:"content"` // HTML, entity-escaped
	FirstPublished string     `json:"first_published"`
	UpdatedAt      string     `json:"updated_at"`
	Departments    []ghDept   `json:"departments"`
	Offices        []ghOffice `json:"offices"`
}

type ghLocation struct {
	Name string `json:"name"`
}

type ghDept struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ghOffice struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Fetch retrieves the board, applies the company's filters and normalises the
// matching jobs into Postings.
func (g *Greenhouse) Fetch(ctx context.Context) ([]posting.Posting, error) {
	if g.company.Slug == "" {
		return nil, fmt.Errorf("greenhouse: company %q has no slug (board token)", g.company.Name)
	}
	endpoint := fmt.Sprintf("%s/%s/jobs?content=true", g.base(), url.PathEscape(g.company.Slug))

	resp, err := g.client.Get(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("greenhouse %q: %w", g.company.Name, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("greenhouse %q: unexpected HTTP %d (is the slug %q correct?)", g.company.Name, resp.StatusCode, g.company.Slug)
	}

	var parsed ghResponse
	if err := json.Unmarshal(resp.Body, &parsed); err != nil {
		return nil, fmt.Errorf("greenhouse %q: parsing response: %w", g.company.Name, err)
	}

	out := make([]posting.Posting, 0, len(parsed.Jobs))
	for i := range parsed.Jobs {
		job := parsed.Jobs[i]
		if !passesFilters(g.filterMeta(job), g.company.Filters) {
			continue
		}
		out = append(out, g.normalise(job))
	}
	return out, nil
}

// filterMeta extracts the facts the shared company filters act on.
func (g *Greenhouse) filterMeta(job ghJob) filterMeta {
	depts := make([]string, 0, len(job.Departments))
	for _, d := range job.Departments {
		depts = append(depts, d.Name)
	}
	offices := make([]int64, 0, len(job.Offices))
	for _, o := range job.Offices {
		offices = append(offices, o.ID)
	}
	return filterMeta{
		Departments: depts,
		OfficeIDs:   offices,
		Location:    job.Location.Name,
		Remote:      remoteFromLocation(job.Location.Name),
	}
}

// normalise converts a Greenhouse job into a Posting.
func (g *Greenhouse) normalise(job ghJob) posting.Posting {
	raw, _ := json.Marshal(job)

	p := posting.Posting{
		ID:              posting.MakeID(sourceGreenhouse, fmt.Sprintf("%s/%d", g.company.Slug, job.ID)),
		Source:          sourceGreenhouse,
		Company:         g.company.Name,
		Title:           strings.TrimSpace(job.Title),
		Location:        strings.TrimSpace(job.Location.Name),
		Remote:          remoteFromLocation(job.Location.Name),
		URL:             job.AbsoluteURL,
		DescriptionText: posting.HTMLToText(html.UnescapeString(job.Content)),
		PostedAt:        parseGreenhouseTime(job.FirstPublished),
		RawJSON:         raw,
	}
	return p
}

// remoteFromLocation makes a best-effort guess from the location text. Work
// model detection is refined by the rules scorer in a later phase.
func remoteFromLocation(locName string) posting.RemoteKind {
	l := strings.ToLower(locName)
	switch {
	case strings.Contains(l, "hybrid"):
		return posting.RemoteHybrid
	case isRemoteText(l):
		return posting.RemoteYes
	default:
		return posting.RemoteUnknown
	}
}

func isRemoteText(lowerLoc string) bool {
	return strings.Contains(lowerLoc, "remote")
}

func parseGreenhouseTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	return nil
}

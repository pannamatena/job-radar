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

// AshbyBaseURL is the public Ashby job-board API. Verified against the live API
// on 2026-10-07: GET /posting-api/job-board/{org}?includeCompensation=true
// returns {jobs:[...]}, each with title, department, team, location,
// workplaceType, isListed, publishedAt, jobUrl and a plain-text description.
const AshbyBaseURL = "https://api.ashbyhq.com/posting-api/job-board"

const sourceAshby = "ashby"

// Ashby fetches postings from one company's Ashby board.
type Ashby struct {
	client  *httpclient.Client
	company config.Company
	baseURL string // overridable in tests
}

// NewAshby builds an Ashby source for a company.
func NewAshby(client *httpclient.Client, company config.Company) *Ashby {
	return &Ashby{client: client, company: company}
}

// Name identifies the source, e.g. "ashby:posthog".
func (a *Ashby) Name() string { return sourceAshby + ":" + a.company.Slug }

func (a *Ashby) base() string {
	if a.baseURL != "" {
		return a.baseURL
	}
	return AshbyBaseURL
}

type ashbyResponse struct {
	Jobs []ashbyJob `json:"jobs"`
}

// ashbyJob mirrors the parts of an Ashby posting we use.
type ashbyJob struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Department         string `json:"department"`
	Team               string `json:"team"`
	Location           string `json:"location"`
	SecondaryLocations []struct {
		Location string `json:"location"`
	} `json:"secondaryLocations"`
	WorkplaceType    string `json:"workplaceType"` // Remote | Hybrid | OnSite
	IsRemote         bool   `json:"isRemote"`
	IsListed         bool   `json:"isListed"`
	PublishedAt      string `json:"publishedAt"`
	JobURL           string `json:"jobUrl"`
	DescriptionPlain string `json:"descriptionPlain"`
}

// Fetch retrieves the board, keeps listed jobs, applies filters and normalises.
func (a *Ashby) Fetch(ctx context.Context) ([]posting.Posting, error) {
	if a.company.Slug == "" {
		return nil, fmt.Errorf("ashby: company %q has no slug", a.company.Name)
	}
	// The org slug can contain spaces (e.g. "Acme Labs"); PathEscape encodes them.
	endpoint := fmt.Sprintf("%s/%s?includeCompensation=true", a.base(), url.PathEscape(a.company.Slug))

	resp, err := a.client.Get(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("ashby %q: %w", a.company.Name, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ashby %q: unexpected HTTP %d (is the slug %q correct?)", a.company.Name, resp.StatusCode, a.company.Slug)
	}

	var parsed ashbyResponse
	if err := json.Unmarshal(resp.Body, &parsed); err != nil {
		return nil, fmt.Errorf("ashby %q: parsing response: %w", a.company.Name, err)
	}

	out := make([]posting.Posting, 0, len(parsed.Jobs))
	for i := range parsed.Jobs {
		job := parsed.Jobs[i]
		if !job.IsListed {
			continue // unlisted/hidden postings are not public openings
		}
		if !passesFilters(a.filterMeta(job), a.company.Filters) {
			continue
		}
		out = append(out, a.normalise(job))
	}
	return out, nil
}

func (a *Ashby) filterMeta(job ashbyJob) filterMeta {
	depts := []string{job.Department}
	if job.Team != "" {
		depts = append(depts, job.Team)
	}
	return filterMeta{
		Departments: depts,
		Location:    ashbyLocation(job),
		Remote:      a.remote(job),
	}
}

func (a *Ashby) normalise(job ashbyJob) posting.Posting {
	raw, _ := json.Marshal(job)
	return posting.Posting{
		ID:              posting.MakeID(sourceAshby, a.company.Slug+"/"+job.ID),
		Source:          sourceAshby,
		Company:         a.company.Name,
		Title:           strings.TrimSpace(job.Title),
		Location:        ashbyLocation(job),
		Remote:          a.remote(job),
		URL:             job.JobURL,
		DescriptionText: strings.TrimSpace(job.DescriptionPlain),
		PostedAt:        parseRFC3339(job.PublishedAt),
		RawJSON:         raw,
	}
}

// remote uses the structured workplaceType, falling back to the isRemote flag.
func (a *Ashby) remote(job ashbyJob) posting.RemoteKind {
	if r := remoteFromWorkplaceType(job.WorkplaceType); r != posting.RemoteUnknown {
		return r
	}
	if job.IsRemote {
		return posting.RemoteYes
	}
	return posting.RemoteUnknown
}

func ashbyLocation(job ashbyJob) string {
	parts := []string{strings.TrimSpace(job.Location)}
	for _, s := range job.SecondaryLocations {
		if sl := strings.TrimSpace(s.Location); sl != "" {
			parts = append(parts, sl)
		}
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "; ")
}

func parseRFC3339(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		u := t.UTC()
		return &u
	}
	return nil
}

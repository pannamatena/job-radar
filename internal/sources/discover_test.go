package sources

import "testing"

func TestDetect(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		body            string
		wantATS         string
		wantSlug        string
		wantUnsupported string
	}{
		{
			name:    "greenhouse embed in body",
			url:     "https://example.com/careers",
			body:    `<script src="https://boards.greenhouse.io/embed/job_board/js?for=acme"></script>`,
			wantATS: "greenhouse", wantSlug: "acme",
		},
		{
			name:    "greenhouse board link",
			url:     "https://example.com/careers",
			body:    `<a href="https://job-boards.greenhouse.io/acmecorp">Open roles</a>`,
			wantATS: "greenhouse", wantSlug: "acmecorp",
		},
		{
			name:    "lever in body",
			url:     "https://example.com/jobs",
			body:    `<iframe src="https://jobs.lever.co/gopuff"></iframe>`,
			wantATS: "lever", wantSlug: "gopuff",
		},
		{
			name:    "ashby from the pasted URL itself",
			url:     "https://jobs.ashbyhq.com/posthog",
			body:    "",
			wantATS: "ashby", wantSlug: "posthog",
		},
		{
			name:    "ashby slug with encoded space",
			url:     "https://jobs.ashbyhq.com/Acme%20Labs",
			body:    "",
			wantATS: "ashby", wantSlug: "Acme Labs",
		},
		{
			name:            "unsupported smartrecruiters",
			url:             "https://careers.etsy.com",
			body:            `<a href="https://api.smartrecruiters.com/v1/companies/etsy/postings">jobs</a>`,
			wantUnsupported: "SmartRecruiters",
		},
		{
			name: "nothing detected",
			url:  "https://example.com/careers",
			body: `<p>We have no public job board.</p>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := Detect(tc.url, []byte(tc.body))
			if d.ATS != tc.wantATS || d.Slug != tc.wantSlug {
				t.Errorf("got ats=%q slug=%q, want ats=%q slug=%q", d.ATS, d.Slug, tc.wantATS, tc.wantSlug)
			}
			if d.Unsupported != tc.wantUnsupported {
				t.Errorf("got unsupported=%q, want %q", d.Unsupported, tc.wantUnsupported)
			}
		})
	}
}

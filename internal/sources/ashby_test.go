package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/httpclient"
)

func newAshbyServer(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("../../testdata/ashby_posthog.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func newTestAshby(t *testing.T, srv *httptest.Server, slug string, filters config.CompanyFilters) *Ashby {
	t.Helper()
	client := httpclient.New(httpclient.Config{UserAgent: "test", MinHostInterval: time.Millisecond})
	a := NewAshby(client, config.Company{Name: "PostHog", ATS: config.ATSAshby, Slug: slug, Filters: filters})
	a.baseURL = srv.URL
	return a
}

func TestAshby_FetchNoFilters(t *testing.T) {
	srv := newAshbyServer(t)
	defer srv.Close()
	ps, err := newTestAshby(t, srv, "posthog", config.CompanyFilters{}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(ps) != 9 {
		t.Fatalf("got %d postings, want 9", len(ps))
	}
	for _, p := range ps {
		if p.Source != "ashby" || !strings.HasPrefix(p.ID, "ashby:posthog/") {
			t.Errorf("bad ID/source: %q %q", p.Source, p.ID)
		}
	}
	// A known OnSite role should normalise to remote=no.
	for _, p := range ps {
		if strings.Contains(p.Title, "Site Reliability Engineer") {
			if p.Remote != "no" {
				t.Errorf("SRE (OnSite) Remote = %q, want no", p.Remote)
			}
			if p.PostedAt == nil {
				t.Error("PostedAt should parse from publishedAt")
			}
		}
	}
}

func TestAshby_DepartmentFilter(t *testing.T) {
	srv := newAshbyServer(t)
	defer srv.Close()
	ps, err := newTestAshby(t, srv, "posthog", config.CompanyFilters{Departments: []string{"Engineering"}}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Fixture has 5 Engineering roles.
	if len(ps) != 5 {
		t.Fatalf("Engineering filter kept %d, want 5: %v", len(ps), titles(ps))
	}
	for _, p := range ps {
		if strings.Contains(p.Title, "Finance") || strings.Contains(p.Title, "Customer Success") {
			t.Errorf("non-engineering role leaked: %q", p.Title)
		}
	}
}

func TestAshby_SlugWithSpaceIsURLEncoded(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Write([]byte(`{"jobs":[]}`))
	}))
	defer srv.Close()
	a := newTestAshby(t, srv, "Acme Labs", config.CompanyFilters{})
	if _, err := a.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(gotPath, "Acme%20Labs") {
		t.Errorf("slug with space not URL-encoded; path = %q", gotPath)
	}
}

func TestAshby_SkipsUnlisted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jobs":[
			{"id":"1","title":"Listed Role","isListed":true,"workplaceType":"Remote","jobUrl":"http://x/1"},
			{"id":"2","title":"Hidden Role","isListed":false,"workplaceType":"Remote","jobUrl":"http://x/2"}
		]}`))
	}))
	defer srv.Close()
	ps, err := newTestAshby(t, srv, "posthog", config.CompanyFilters{}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Title != "Listed Role" {
		t.Fatalf("expected only the listed role, got %v", titles(ps))
	}
}

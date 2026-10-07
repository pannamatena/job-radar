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
	"github.com/pannamatena/job-radar/internal/posting"
)

// newFixtureServer serves the recorded Anthropic board at /{slug}/jobs.
func newFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("../../testdata/greenhouse_anthropic.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/jobs") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("content") != "true" {
			t.Errorf("expected content=true query, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func newTestGreenhouse(t *testing.T, srv *httptest.Server, filters config.CompanyFilters) *Greenhouse {
	t.Helper()
	client := httpclient.New(httpclient.Config{
		UserAgent:       "test",
		MinHostInterval: time.Millisecond,
	})
	g := NewGreenhouse(client, config.Company{
		Name:    "Anthropic",
		ATS:     config.ATSGreenhouse,
		Slug:    "anthropic",
		Filters: filters,
	})
	g.baseURL = srv.URL
	return g
}

func idSet(ps []posting.Posting) map[string]bool {
	m := make(map[string]bool, len(ps))
	for _, p := range ps {
		m[p.ID] = true
	}
	return m
}

func TestGreenhouse_FetchNoFilters(t *testing.T) {
	srv := newFixtureServer(t)
	defer srv.Close()
	g := newTestGreenhouse(t, srv, config.CompanyFilters{})

	ps, err := g.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(ps) != 8 {
		t.Fatalf("got %d postings, want 8 (all fixture jobs)", len(ps))
	}

	// Check the Staff Software Engineer (Dublin) is normalised correctly.
	var swe *posting.Posting
	for i := range ps {
		if ps[i].ID == "greenhouse:anthropic/5101169008" {
			swe = &ps[i]
		}
	}
	if swe == nil {
		t.Fatal("expected the Staff Software Engineer posting (id 5101169008)")
	}
	if swe.Source != "greenhouse" {
		t.Errorf("Source = %q", swe.Source)
	}
	if swe.Company != "Anthropic" {
		t.Errorf("Company = %q", swe.Company)
	}
	if !strings.Contains(swe.Title, "Staff Software Engineer") {
		t.Errorf("Title = %q", swe.Title)
	}
	if swe.Location != "Dublin, IE" {
		t.Errorf("Location = %q, want Dublin, IE", swe.Location)
	}
	if swe.URL == "" || !strings.HasPrefix(swe.URL, "http") {
		t.Errorf("URL = %q", swe.URL)
	}
	if swe.PostedAt == nil {
		t.Errorf("PostedAt should be parsed from first_published")
	}
	// Content should be real text with tags and entity-escaping removed.
	if swe.DescriptionText == "" {
		t.Error("DescriptionText is empty")
	}
	if strings.Contains(swe.DescriptionText, "&lt;") || strings.Contains(swe.DescriptionText, "<div") {
		t.Errorf("DescriptionText still contains markup/escaping:\n%.200s", swe.DescriptionText)
	}
	if len(swe.RawJSON) == 0 {
		t.Error("RawJSON should be preserved")
	}
}

func TestGreenhouse_RemoteDetection(t *testing.T) {
	srv := newFixtureServer(t)
	defer srv.Close()
	g := newTestGreenhouse(t, srv, config.CompanyFilters{})
	ps, err := g.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]posting.Posting{}
	for _, p := range ps {
		byID[p.ID] = p
	}
	// 5205495008 is listed in several locations incl. "Remote-Friendly".
	if got := byID["greenhouse:anthropic/5205495008"].Remote; got != posting.RemoteYes {
		t.Errorf("multi-location remote role Remote = %q, want yes", got)
	}
	// 5101169008 is Dublin only.
	if got := byID["greenhouse:anthropic/5101169008"].Remote; got != posting.RemoteUnknown {
		t.Errorf("Dublin-only role Remote = %q, want unknown", got)
	}
}

func TestGreenhouse_OfficeIDFilter(t *testing.T) {
	srv := newFixtureServer(t)
	defer srv.Close()
	// 4006509008 = Anthropic's Dublin, IE office (from the fixture).
	g := newTestGreenhouse(t, srv, config.CompanyFilters{OfficeIDs: []int64{4006509008}})

	ps, err := g.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := idSet(ps)
	want := []string{
		"greenhouse:anthropic/5101169008",
		"greenhouse:anthropic/5391313008",
		"greenhouse:anthropic/5393268008",
	}
	if len(ps) != len(want) {
		t.Fatalf("office filter kept %d, want %d: %v", len(ps), len(want), ids)
	}
	for _, id := range want {
		if !ids[id] {
			t.Errorf("expected %s to pass the Dublin-office filter", id)
		}
	}
}

func TestGreenhouse_DepartmentFilter(t *testing.T) {
	srv := newFixtureServer(t)
	defer srv.Close()
	g := newTestGreenhouse(t, srv, config.CompanyFilters{Departments: []string{"Engineering"}})

	ps, err := g.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := idSet(ps)
	// "Software Engineering - Infrastructure" and "AI Research & Engineering".
	if len(ps) != 2 || !ids["greenhouse:anthropic/5101169008"] || !ids["greenhouse:anthropic/5183044008"] {
		t.Fatalf("department filter kept %d: %v", len(ps), ids)
	}
}

func TestGreenhouse_CombinedOfficeAndDepartmentFilter(t *testing.T) {
	srv := newFixtureServer(t)
	defer srv.Close()
	g := newTestGreenhouse(t, srv, config.CompanyFilters{
		OfficeIDs:   []int64{4006509008},
		Departments: []string{"Software Engineering"},
	})

	ps, err := g.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].ID != "greenhouse:anthropic/5101169008" {
		t.Fatalf("combined filter should keep only the Dublin Staff SWE, got %d: %v", len(ps), idSet(ps))
	}
}

func TestGreenhouse_BadSlugStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	g := newTestGreenhouse(t, srv, config.CompanyFilters{})
	if _, err := g.Fetch(context.Background()); err == nil {
		t.Fatal("expected an error for a 404 board")
	}
}

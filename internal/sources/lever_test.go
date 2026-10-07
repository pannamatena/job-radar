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

func newLeverServer(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("../../testdata/lever_gopuff.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("mode") != "json" {
			t.Errorf("expected mode=json, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func newTestLever(t *testing.T, srv *httptest.Server, filters config.CompanyFilters) *Lever {
	t.Helper()
	client := httpclient.New(httpclient.Config{UserAgent: "test", MinHostInterval: time.Millisecond})
	l := NewLever(client, config.Company{Name: "Gopuff", ATS: config.ATSLever, Slug: "gopuff", Filters: filters})
	l.baseURL = srv.URL
	return l
}

func TestLever_FetchNoFilters(t *testing.T) {
	srv := newLeverServer(t)
	defer srv.Close()
	ps, err := newTestLever(t, srv, config.CompanyFilters{}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(ps) != 8 {
		t.Fatalf("got %d postings, want 8", len(ps))
	}
	p := ps[0] // "Analytics Engineer", Tech/Product/Design, hybrid, Philadelphia
	if p.Source != "lever" || !strings.HasPrefix(p.ID, "lever:gopuff/") {
		t.Errorf("bad ID/source: %q %q", p.Source, p.ID)
	}
	if p.Title != "Analytics Engineer" {
		t.Errorf("Title = %q", p.Title)
	}
	if p.Remote != "hybrid" {
		t.Errorf("Remote = %q, want hybrid", p.Remote)
	}
	if !strings.Contains(p.Location, "Philadelphia") {
		t.Errorf("Location = %q", p.Location)
	}
	if p.URL == "" || p.PostedAt == nil {
		t.Errorf("URL/PostedAt missing: %q %v", p.URL, p.PostedAt)
	}
	if p.DescriptionText == "" {
		t.Error("DescriptionText empty")
	}
}

func TestLever_DepartmentFilter(t *testing.T) {
	srv := newLeverServer(t)
	defer srv.Close()
	// "Tech, Product, & Design" is the engineering-ish department in the fixture.
	ps, err := newTestLever(t, srv, config.CompanyFilters{Departments: []string{"Tech, Product"}}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Title != "Analytics Engineer" {
		t.Fatalf("department filter kept %d: %v", len(ps), titles(ps))
	}
}

func TestLever_RemoteWorkplaceType(t *testing.T) {
	srv := newLeverServer(t)
	defer srv.Close()
	ps, err := newTestLever(t, srv, config.CompanyFilters{}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var remotes, onsite int
	for _, p := range ps {
		switch p.Remote {
		case "yes":
			remotes++
		case "no":
			onsite++
		}
	}
	if remotes == 0 || onsite == 0 {
		t.Errorf("expected a mix of remote and onsite, got remote=%d onsite=%d", remotes, onsite)
	}
}

func TestLever_LocationNameFilter(t *testing.T) {
	srv := newLeverServer(t)
	defer srv.Close()
	ps, err := newTestLever(t, srv, config.CompanyFilters{LocationNames: []string{"Philadelphia"}}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) == 0 {
		t.Fatal("expected Philadelphia roles")
	}
	for _, p := range ps {
		if !strings.Contains(strings.ToLower(p.Location), "philadelphia") {
			t.Errorf("location filter leaked %q", p.Location)
		}
	}
}

func titles(ps []posting.Posting) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Title
	}
	return out
}

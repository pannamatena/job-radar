package sources

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/httpclient"
)

func testClient() *httpclient.Client {
	return httpclient.New(httpclient.Config{UserAgent: "test", MinHostInterval: time.Millisecond, BackoffBase: time.Millisecond})
}

// greenhouseBoth serves the recorded board at /anthropic/jobs and each of its
// jobs at /anthropic/jobs/{id}, the way the real API does. Any other id is 404,
// and the id "500" is a server error.
func greenhouseBoth(t *testing.T) (*httptest.Server, []json.RawMessage) {
	t.Helper()
	body, err := os.ReadFile("../../testdata/greenhouse_anthropic.json")
	if err != nil {
		t.Fatal(err)
	}
	var board struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &board); err != nil {
		t.Fatal(err)
	}
	byID := map[string]json.RawMessage{}
	for _, j := range board.Jobs {
		var h struct {
			ID json.Number `json:"id"`
		}
		_ = json.Unmarshal(j, &h)
		byID[h.ID.String()] = j
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/jobs"):
			_, _ = w.Write(body)
		case strings.Contains(r.URL.Path, "/jobs/"):
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			if id == "500" {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			j, ok := byID[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(j)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv, board.Jobs
}

// The two ways of reading a job (ADR-0038) must give the same posting, with the
// same id, or a job found by search and by the watch-list would be two jobs.
func TestGreenhouse_OneJobMatchesTheBoardList(t *testing.T) {
	srv, _ := greenhouseBoth(t)
	defer srv.Close()
	g := NewGreenhouse(testClient(), config.Company{Name: "Anthropic", ATS: config.ATSGreenhouse, Slug: "anthropic"})
	g.baseURL = srv.URL

	all, err := g.Fetch(context.Background())
	if err != nil || len(all) == 0 {
		t.Fatalf("Fetch: %v (%d postings)", err, len(all))
	}
	for _, want := range all {
		id := want.ID[strings.LastIndex(want.ID, "/")+1:]
		got, found, err := g.FetchOne(context.Background(), id)
		if err != nil || !found {
			t.Fatalf("FetchOne(%s): found=%v err=%v", id, found, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("job %s differs between the two reads:\n one: %+v\nlist: %+v", id, got, want)
		}
	}
}

func TestGreenhouse_OneJobClosedAndErrors(t *testing.T) {
	srv, _ := greenhouseBoth(t)
	defer srv.Close()
	g := NewGreenhouse(testClient(), config.Company{Name: "Anthropic", ATS: config.ATSGreenhouse, Slug: "anthropic"})
	g.baseURL = srv.URL

	if _, found, err := g.FetchOne(context.Background(), "1"); found || err != nil {
		t.Errorf("a 404 is a closed job, not an error: found=%v err=%v", found, err)
	}
	if _, found, err := g.FetchOne(context.Background(), "500"); found || err == nil {
		t.Errorf("a server error must be an error, not a closed job: found=%v err=%v", found, err)
	}
}

func leverBoth(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("../../testdata/lever_gopuff.json")
	if err != nil {
		t.Fatal(err)
	}
	var jobs []json.RawMessage
	if err := json.Unmarshal(body, &jobs); err != nil {
		t.Fatal(err)
	}
	byID := map[string]json.RawMessage{}
	for _, j := range jobs {
		var h struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(j, &h)
		byID[h.ID] = j
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		switch {
		case len(parts) == 1:
			_, _ = w.Write(body)
		case parts[1] == "boom":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			j, ok := byID[parts[1]]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(j)
		}
	}))
}

func TestLever_OneJobMatchesTheBoardList(t *testing.T) {
	srv := leverBoth(t)
	defer srv.Close()
	l := NewLever(testClient(), config.Company{Name: "Gopuff", ATS: config.ATSLever, Slug: "gopuff"})
	l.baseURL = srv.URL

	all, err := l.Fetch(context.Background())
	if err != nil || len(all) == 0 {
		t.Fatalf("Fetch: %v (%d postings)", err, len(all))
	}
	for _, want := range all {
		id := want.ID[strings.LastIndex(want.ID, "/")+1:]
		got, found, err := l.FetchOne(context.Background(), id)
		if err != nil || !found {
			t.Fatalf("FetchOne(%s): found=%v err=%v", id, found, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("posting %s differs between the two reads:\n one: %+v\nlist: %+v", id, got, want)
		}
	}
}

func TestLever_OneJobClosedAndErrors(t *testing.T) {
	srv := leverBoth(t)
	defer srv.Close()
	l := NewLever(testClient(), config.Company{Name: "Gopuff", ATS: config.ATSLever, Slug: "gopuff"})
	l.baseURL = srv.URL

	if _, found, err := l.FetchOne(context.Background(), "00000000-0000-0000-0000-000000000000"); found || err != nil {
		t.Errorf("a 404 is a closed posting, not an error: found=%v err=%v", found, err)
	}
	if _, found, err := l.FetchOne(context.Background(), "boom"); found || err == nil {
		t.Errorf("a server error must be an error, not a closed posting: found=%v err=%v", found, err)
	}
}

func TestReadOne_AshbyHasNoSingleRead(t *testing.T) {
	_, _, err := ReadOne(context.Background(), testClient(), config.Company{Name: "x", ATS: config.ATSAshby, Slug: "x"}, "abc")
	if !errors.Is(err, ErrNoSingleRead) {
		t.Errorf("want ErrNoSingleRead, got %v", err)
	}
}

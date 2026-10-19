package search

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/sources"
)

const jobID = "43dbf072-0fc1-48c9-8c1e-7416db7d4a14"

// fakeSearcher returns canned candidates per query.
type fakeSearcher struct {
	results map[string][]Candidate
	errFor  string
	usage   Usage
	asked   []string
}

func (f *fakeSearcher) Search(_ context.Context, q string, _ Options) (Result, error) {
	f.asked = append(f.asked, q)
	if q == f.errFor {
		return Result{}, errors.New("boom")
	}
	return Result{Candidates: f.results[q], Usage: f.usage}, nil
}

func TestPickQueriesRotates(t *testing.T) {
	all := []string{"a", "b", "c", "d", "e"}
	if got := PickQueries(all, 0, 7); !reflect.DeepEqual(got, all) {
		t.Errorf("0 means all, got %v", got)
	}
	if got := PickQueries(all, 9, 0); !reflect.DeepEqual(got, all) {
		t.Errorf("more than there are means all, got %v", got)
	}
	if got := PickQueries(all, 2, 0); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("got %v", got)
	}
	if got := PickQueries(all, 2, 4); !reflect.DeepEqual(got, []string{"e", "a"}) {
		t.Errorf("should wrap round, got %v", got)
	}
	if got := PickQueries(all, 2, 7); !reflect.DeepEqual(got, []string{"c", "d"}) {
		t.Errorf("start beyond the end wraps, got %v", got)
	}
	if PickQueries(nil, 2, 0) != nil {
		t.Error("no queries, nothing to pick")
	}
}

func TestFinderSortsEverythingTheSearchReturns(t *testing.T) {
	cfg := config.Search{MaxSearchesPerQuery: 2, BlockedDomains: []string{"spam.example"}, LinkDomains: []string{"linkedin.com"}}
	fs := &fakeSearcher{results: map[string][]Candidate{
		"q1": {
			{URL: "https://jobs.ashbyhq.com/acme/" + jobID, Title: "EM"},
			{URL: "https://jobs.ashbyhq.com/acme/" + jobID + "?utm=x", Title: "EM again"}, // same posting
			{URL: "https://boards.greenhouse.io/globex/jobs/12345", Title: "Lead"},
			{URL: "https://www.linkedin.com/jobs/view/1", Title: "On LinkedIn"},
			{URL: "https://news.example.com/jobs/2", Title: "Some\nSite  \u0007Job"},
			{URL: "https://deals.spam.example/x", Title: "blocked"},
			{URL: "https://jobs.ashbyhq.com.evil.example/acme/" + jobID, Title: "lookalike"},
			{URL: "javascript:alert(1)", Title: "junk"},
			{URL: "not a url", Title: "junk"},
		},
		"q2": {{URL: "https://news.example.com/jobs/2", Title: "same lead"}},
	}}
	f := &Finder{Searcher: fs, Cfg: cfg}
	got := f.Run(context.Background(), []string{"q1", "q2"}, 100)

	wantRefs := []string{"ashby:acme/" + jobID, "greenhouse:globex/12345"}
	var gotRefs []string
	for _, r := range got.Refs {
		gotRefs = append(gotRefs, r.PostingID())
	}
	if !reflect.DeepEqual(gotRefs, wantRefs) {
		t.Errorf("refs = %v, want %v", gotRefs, wantRefs)
	}
	byHost := map[string]Lead{}
	for _, l := range got.Leads {
		byHost[l.Host] = l
	}
	if li := byHost["www.linkedin.com"]; !li.Known {
		t.Errorf("a LinkedIn link is a known link lead: %+v", li)
	}
	if n := byHost["news.example.com"]; n.Known || n.Title != "Some Site Job" {
		t.Errorf("an unknown site is not a link, and its title is cleaned: %+v", n)
	}
	if _, ok := byHost["deals.spam.example"]; ok {
		t.Error("a blocked site must be dropped")
	}
	if l, ok := byHost["jobs.ashbyhq.com.evil.example"]; !ok || l.Known {
		t.Errorf("a lookalike host is only ever an unknown lead: %+v", l)
	}
	if len(got.Leads) != 3 { // linkedin, news (once), lookalike
		t.Errorf("want 3 leads (the same lead twice counts once), got %d: %+v", len(got.Leads), got.Leads)
	}
}

func TestFinderStopsAtTheBudgetAndKeepsGoingAfterAnError(t *testing.T) {
	price := config.ModelPrice{Input: 0, Output: 0}
	fs := &fakeSearcher{errFor: "bad", usage: Usage{Searches: 1}, results: map[string][]Candidate{}}
	f := &Finder{Searcher: fs, Cfg: config.Search{MaxSearchesPerQuery: 1}, Price: price}

	got := f.Run(context.Background(), []string{"bad", "ok", "never"}, 0.01) // a failed query costs nothing; "ok" uses the $0.01 budget
	if len(got.Errors) != 1 || !strings.Contains(got.Errors[0], "boom") {
		t.Errorf("errors: %v", got.Errors)
	}
	if !reflect.DeepEqual(fs.asked, []string{"bad", "ok"}) {
		t.Errorf("asked %v; after the cap is reached the rest must not run", fs.asked)
	}
	if !strings.Contains(got.Stopped, "cap") || got.Queries != 2 {
		t.Errorf("stopped=%q queries=%d", got.Stopped, got.Queries)
	}
}

func TestResolveReadsBoardsOnceKeepsOnlyFoundPostingsAndRespectsLists(t *testing.T) {
	ref := func(ats, slug, id string) Ref { return Ref{ATS: ats, Slug: slug, JobID: id} }
	refs := []Ref{
		ref("ashby", "acme", jobID),
		ref("ashby", "acme", "11111111-1111-1111-1111-111111111111"), // closed: not on the board any more
		ref("greenhouse", "globex", "1"),
		ref("greenhouse", "mine", "2"),   // on the watch-list
		ref("greenhouse", "oldjob", "3"), // ignored company
		ref("lever", "broken", jobID),    // board can't be read
	}
	fetched := map[string]int{}
	fetch := func(_ context.Context, c config.Company) ([]posting.Posting, error) {
		fetched[c.ATS+":"+c.Slug]++
		switch c.Slug {
		case "acme":
			return []posting.Posting{{ID: "ashby:acme/" + jobID, Company: "acme"}, {ID: "ashby:acme/other", Company: "acme"}}, nil
		case "globex":
			return []posting.Posting{{ID: "greenhouse:globex/1"}, {ID: "greenhouse:globex/99"}}, nil
		}
		return nil, errors.New("503")
	}
	watch := []config.Company{{Name: "Mine", ATS: "greenhouse", Slug: "mine"}}
	got, stats, problems := Resolve(context.Background(), fetch, nil, refs, watch, []string{"Old Job"})

	var ids []string
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	if want := []string{"ashby:acme/" + jobID, "greenhouse:globex/1"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("postings = %v, want %v", ids, want)
	}
	if fetched["ashby:acme"] != 1 {
		t.Errorf("a board with two found jobs is read once, got %d", fetched["ashby:acme"])
	}
	if fetched["greenhouse:mine"] != 0 || fetched["greenhouse:oldjob"] != 0 {
		t.Errorf("watch-list and ignored boards must not be read: %v", fetched)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "broken") {
		t.Errorf("problems = %v", problems)
	}
	want := ResolveStats{Found: 6, Read: 2, OnWatch: 1, Ignored: 1, Closed: 1, BoardsOK: 2, BoardsFail: 1, FailedJobs: 1}
	if stats != want {
		t.Errorf("stats = %+v, want %+v", stats, want)
	}
	if stats.Read+stats.OnWatch+stats.Ignored+stats.Closed+stats.FailedJobs != stats.Found {
		t.Error("every found posting must be accounted for")
	}
}

// stubBoards builds a FetchBoard and a FetchOne over the same fake boards and
// counts the requests each one receives.
type stubBoards struct {
	open       map[string][]string // "ats:slug" -> open job ids
	lists      map[string]int
	singles    map[string]int
	noSingle   map[string]bool // boards with no single-posting read (like Ashby)
	singleFail map[string]bool // "ats:slug/id" fails with a server error
}

func newStubBoards(open map[string][]string) *stubBoards {
	return &stubBoards{open: open, lists: map[string]int{}, singles: map[string]int{}, noSingle: map[string]bool{}, singleFail: map[string]bool{}}
}

func (b *stubBoards) post(ats, slug, id string) posting.Posting {
	return posting.Posting{ID: posting.MakeID(ats, slug+"/"+id), Company: slug, Title: "Role " + id}
}

func (b *stubBoards) fetch(_ context.Context, c config.Company) ([]posting.Posting, error) {
	b.lists[c.ATS+":"+c.Slug]++
	var out []posting.Posting
	for _, id := range b.open[c.ATS+":"+c.Slug] {
		out = append(out, b.post(c.ATS, c.Slug, id))
	}
	return out, nil
}

func (b *stubBoards) one(_ context.Context, c config.Company, id string) (posting.Posting, bool, error) {
	key := c.ATS + ":" + c.Slug
	if b.noSingle[key] {
		return posting.Posting{}, false, sources.ErrNoSingleRead
	}
	b.singles[key]++
	if b.singleFail[key+"/"+id] {
		return posting.Posting{}, false, errors.New("HTTP 500")
	}
	for _, o := range b.open[key] {
		if o == id {
			return b.post(c.ATS, c.Slug, id), true, nil
		}
	}
	return posting.Posting{}, false, nil
}

func refsFor(ats, slug string, ids ...string) []Ref {
	var out []Ref
	for _, id := range ids {
		out = append(out, Ref{ATS: ats, Slug: slug, JobID: id})
	}
	return out
}

func TestResolveReadsFewJobsOneByOne(t *testing.T) {
	b := newStubBoards(map[string][]string{"greenhouse:acme": {"1", "2", "3", "4"}})
	got, stats, problems := Resolve(context.Background(), b.fetch, b.one, refsFor("greenhouse", "acme", "1", "9"), nil, nil)

	if len(got) != 1 || got[0].ID != "greenhouse:acme/1" {
		t.Errorf("postings = %+v", got)
	}
	if b.singles["greenhouse:acme"] != 2 || b.lists["greenhouse:acme"] != 0 {
		t.Errorf("two found jobs are read one by one: singles=%v lists=%v", b.singles, b.lists)
	}
	if want := (ResolveStats{Found: 2, Read: 1, Closed: 1, BoardsOK: 1}); stats != want || len(problems) != 0 {
		t.Errorf("stats = %+v (want %+v), problems = %v", stats, want, problems)
	}
}

func TestResolveReadsTheWholeBoardFromThreeJobs(t *testing.T) {
	b := newStubBoards(map[string][]string{"lever:acme": {"1", "2", "3"}})
	got, stats, _ := Resolve(context.Background(), b.fetch, b.one, refsFor("lever", "acme", "1", "2", "3"), nil, nil)

	if len(got) != 3 || b.lists["lever:acme"] != 1 || b.singles["lever:acme"] != 0 {
		t.Errorf("three found jobs at one board are one board read: got=%d lists=%v singles=%v", len(got), b.lists, b.singles)
	}
	if stats.Read != 3 || stats.Closed != 0 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestResolveFallsBackToTheBoardWhereThereIsNoSingleRead(t *testing.T) {
	b := newStubBoards(map[string][]string{"ashby:acme": {"1"}})
	b.noSingle["ashby:acme"] = true
	got, stats, _ := Resolve(context.Background(), b.fetch, b.one, refsFor("ashby", "acme", "1", "2"), nil, nil)

	if len(got) != 1 || b.lists["ashby:acme"] != 1 {
		t.Errorf("got=%d lists=%v", len(got), b.lists)
	}
	if want := (ResolveStats{Found: 2, Read: 1, Closed: 1, BoardsOK: 1}); stats != want {
		t.Errorf("stats = %+v, want %+v", stats, want)
	}
}

func TestResolveSingleReadErrorIsAFailureNotAClosedJob(t *testing.T) {
	b := newStubBoards(map[string][]string{"greenhouse:acme": {"1", "2"}})
	b.singleFail["greenhouse:acme/1"] = true
	got, stats, problems := Resolve(context.Background(), b.fetch, b.one, refsFor("greenhouse", "acme", "1", "2"), nil, nil)

	if len(got) != 1 || got[0].ID != "greenhouse:acme/2" {
		t.Errorf("the other job is still read: %+v", got)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "acme/1") {
		t.Errorf("problems = %v", problems)
	}
	if want := (ResolveStats{Found: 2, Read: 1, BoardsFail: 1, FailedJobs: 1}); stats != want {
		t.Errorf("stats = %+v, want %+v", stats, want)
	}
	if stats.Read+stats.Closed+stats.FailedJobs != stats.Found {
		t.Error("every found posting must be accounted for")
	}
}

// Both ways of reading must hand back the same postings (ADR-0038).
func TestResolveBothWaysGiveTheSamePostings(t *testing.T) {
	open := map[string][]string{"greenhouse:acme": {"1", "2", "3"}}
	refs := refsFor("greenhouse", "acme", "1", "2")

	b1 := newStubBoards(open)
	single, _, _ := Resolve(context.Background(), b1.fetch, b1.one, refs, nil, nil)
	b2 := newStubBoards(open)
	whole, _, _ := Resolve(context.Background(), b2.fetch, nil, refs, nil, nil)

	if b1.singles["greenhouse:acme"] != 2 || b2.lists["greenhouse:acme"] != 1 {
		t.Fatalf("the two runs must use the two ways: %v %v", b1.singles, b2.lists)
	}
	if !reflect.DeepEqual(single, whole) {
		t.Errorf("one by one %+v != whole board %+v", single, whole)
	}
}

func TestNormalName(t *testing.T) {
	for _, s := range []string{"Acme Labs", "acme-labs", "AcmeLabs", " ACME_labs "} {
		if NormalName(s) != "acmelabs" {
			t.Errorf("NormalName(%q) = %q", s, NormalName(s))
		}
	}
}

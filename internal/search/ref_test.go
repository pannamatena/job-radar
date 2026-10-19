package search

import "testing"

func TestParseJobURL(t *testing.T) {
	const id = "43dbf072-0fc1-48c9-8c1e-7416db7d4a14"
	tests := []struct {
		name string
		url  string
		want Ref
		ok   bool
	}{
		{"ashby", "https://jobs.ashbyhq.com/acme/" + id, Ref{"ashby", "acme", id}, true},
		{"ashby with an encoded space in the name", "https://jobs.ashbyhq.com/Acme%20Labs/" + id, Ref{"ashby", "Acme Labs", id}, true},
		{"ashby application page", "https://jobs.ashbyhq.com/acme/" + id + "/application", Ref{"ashby", "acme", id}, true},
		{"ashby tracking query is ignored", "https://jobs.ashbyhq.com/acme/" + id + "?utm_source=x", Ref{"ashby", "acme", id}, true},
		{"greenhouse", "https://boards.greenhouse.io/acme/jobs/4087603101", Ref{"greenhouse", "acme", "4087603101"}, true},
		{"greenhouse new host", "https://job-boards.greenhouse.io/acme/jobs/4087603101", Ref{"greenhouse", "acme", "4087603101"}, true},
		{"greenhouse eu host", "https://job-boards.eu.greenhouse.io/acme/jobs/4087603101?gh_src=abc", Ref{"greenhouse", "acme", "4087603101"}, true},
		{"lever", "https://jobs.lever.co/acme/" + id, Ref{"lever", "acme", id}, true},
		{"lever apply page", "https://jobs.lever.co/acme/" + id + "/apply", Ref{"lever", "acme", id}, true},
		{"host in capitals is the same host", "https://JOBS.ASHBYHQ.COM/acme/" + id, Ref{"ashby", "acme", id}, true},

		{"empty", "", Ref{}, false},
		{"plain http", "http://jobs.ashbyhq.com/acme/" + id, Ref{}, false},
		{"lookalike: real host as a prefix", "https://jobs.ashbyhq.com.example.net/acme/" + id, Ref{}, false},
		{"lookalike: extra letter before", "https://xjobs.ashbyhq.com/acme/" + id, Ref{}, false},
		{"lookalike: real host in the path", "https://example.net/jobs.ashbyhq.com/acme/" + id, Ref{}, false},
		{"lookalike: real host as the username", "https://jobs.ashbyhq.com@example.net/acme/" + id, Ref{}, false},
		{"username on the real host", "https://me@jobs.ashbyhq.com/acme/" + id, Ref{}, false},
		{"port number", "https://jobs.ashbyhq.com:8443/acme/" + id, Ref{}, false},
		{"look-alike letter (Cyrillic o)", "https://jobs.ashbyhq.cоm/acme/" + id, Ref{}, false},
		{"sub-domain that is not on the list", "https://evil.jobs.lever.co/acme/" + id, Ref{}, false},
		{"company page, not a posting", "https://jobs.ashbyhq.com/acme", Ref{}, false},
		{"not a uuid", "https://jobs.ashbyhq.com/acme/not-a-uuid", Ref{}, false},
		{"greenhouse id not numeric", "https://boards.greenhouse.io/acme/jobs/abc", Ref{}, false},
		{"greenhouse missing /jobs/", "https://boards.greenhouse.io/acme/4087603101", Ref{}, false},
		{"extra path segment", "https://jobs.ashbyhq.com/acme/" + id + "/x/y", Ref{}, false},
		{"path traversal", "https://jobs.ashbyhq.com/../" + id, Ref{}, false},
		{"encoded slash in the name", "https://jobs.ashbyhq.com/a%2Fb/" + id, Ref{}, false},
		{"markup in the name", "https://jobs.ashbyhq.com/%3Cscript%3E/" + id, Ref{}, false},
		{"space in the link", "https://jobs.ashbyhq.com/acme/" + id + " x", Ref{}, false},
		{"control character", "https://jobs.ashbyhq.com/acme/" + id + "\n", Ref{}, false},
		{"javascript link", "javascript:alert(1)", Ref{}, false},
		{"a different site", "https://www.linkedin.com/jobs/view/123", Ref{}, false},
		{"far too long", "https://jobs.ashbyhq.com/acme/" + id + "?" + string(make([]byte, 600)), Ref{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseJobURL(tt.url)
			if ok != tt.ok || got != tt.want {
				t.Errorf("ParseJobURL(%q) = %+v, %v; want %+v, %v", tt.url, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPostingIDMatchesTheWatchListIDs(t *testing.T) {
	const id = "43dbf072-0fc1-48c9-8c1e-7416db7d4a14"
	r, _ := ParseJobURL("https://jobs.ashbyhq.com/Acme%20Labs/" + id)
	if r.PostingID() != "ashby:Acme Labs/"+id {
		t.Errorf("got %q", r.PostingID())
	}
	g, _ := ParseJobURL("https://boards.greenhouse.io/acme/jobs/123")
	if g.PostingID() != "greenhouse:acme/123" {
		t.Errorf("got %q", g.PostingID())
	}
}

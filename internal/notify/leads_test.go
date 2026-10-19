package notify

import (
	"strings"
	"testing"
)

func TestDefang(t *testing.T) {
	tests := map[string]string{
		"news.example.com":                "news[.]example[.]com",
		"https://evil.example/x":          "hxxps://evil[.]example/x",
		"visit evil.example.com for jobs": "visit evil[.]example[.]com for jobs",
		"Node.js engineer":                "Node[.]js engineer",
		"A.B.C":                           "A[.]B[.]C",
		"Engineering Manager":             "Engineering Manager",
		"1.5 years":                       "1.5 years", // a dot before a digit isn't a site
	}
	for in, want := range tests {
		if got := Defang(in); got != want {
			t.Errorf("Defang(%q) = %q, want %q", in, got, want)
		}
	}
}

func leadsMessage() Message {
	return Message{Kind: KindDigest, Subject: "s", Leads: []Lead{
		{Title: "Engineering Manager at example.org", Host: "news.example.com", URL: "https://news.example.com/job/1"},
		{Title: "Lead engineer", Host: "www.linkedin.com", URL: "https://www.linkedin.com/jobs/view/1", Known: true},
		{Title: "Tricky", Host: "bad.example", URL: "javascript:alert(1)", Known: true},
	}, NewSites: []string{"news.example.com"}}
}

func TestLeadsCannotBecomeLinks(t *testing.T) {
	m := leadsMessage()
	for name, out := range map[string]string{"text": RenderText(m), "html": RenderHTML(m)} {
		if !strings.Contains(out, "news[.]example[.]com (unknown site)") {
			t.Errorf("%s: the unknown site must be written defanged:\n%s", name, out)
		}
		if strings.Contains(out, "news.example.com/job") || strings.Contains(out, "news.example.com (unknown") {
			t.Errorf("%s: the unknown site's real address must never appear:\n%s", name, out)
		}
		if !strings.Contains(out, "Engineering Manager at example[.]org") {
			t.Errorf("%s: a site name inside a title is defanged too:\n%s", name, out)
		}
		if !strings.Contains(out, "Check that a site is safe before you open it") {
			t.Errorf("%s: the warning is missing", name)
		}
		if !strings.Contains(out, "New sites seen: news[.]example[.]com") {
			t.Errorf("%s: the new-site notice is missing", name)
		}
		if strings.Contains(out, "javascript:") {
			t.Errorf("%s: an unsafe link must never be shown", name)
		}
	}
	if !strings.Contains(RenderHTML(m), `<a href="https://www.linkedin.com/jobs/view/1"`) {
		t.Error("a known site's https link is a normal link in the HTML email")
	}
	if strings.Count(RenderHTML(m), "<a ") != 1 {
		t.Errorf("only the known site may be a link:\n%s", RenderHTML(m))
	}
}

func TestLeadsAreNotNumberedOrVotedOn(t *testing.T) {
	m := leadsMessage()
	if len(Numbered(m)) != 0 {
		t.Error("leads have no number")
	}
	if strings.Contains(RenderText(m), "Reply to this email") {
		t.Error("leads alone don't invite votes")
	}
}

func TestLeadsAreCappedAndEscaped(t *testing.T) {
	m := Message{}
	for i := 0; i < MaxLeadsShown+3; i++ {
		m.Leads = append(m.Leads, Lead{Title: "<script>alert(1)</script>", Host: "x.example"})
	}
	html := RenderHTML(m)
	if strings.Contains(html, "<script>") {
		t.Error("lead titles must be escaped")
	}
	if !strings.Contains(RenderText(m), "… and 3 more") {
		t.Error("extra leads are summarised")
	}
}

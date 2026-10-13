package posting

import (
	"strings"
	"testing"
)

var adBody = strings.Repeat("You will lead a team of frontend engineers building our React and TypeScript product. ", 6) +
	"We value kindness, ownership and shipping small changes often."

func ats(id string) Posting {
	return Posting{ID: "greenhouse:" + id, Source: "greenhouse", Company: "Acme Labs",
		Title: "Engineering Manager", Location: "Manchester", Remote: RemoteHybrid, DescriptionText: adBody}
}

func email(id string) Posting {
	return Posting{ID: "imap:" + id, Source: "imap", Company: "Acme Labs",
		Title: "Engineering Manager", Location: "Manchester", Remote: RemoteUnknown,
		DescriptionText: "Lead a team of frontend engineers."}
}

func TestSameJob(t *testing.T) {
	a := ats("1")

	reposted := ats("x")
	reposted.Source, reposted.ID = "lever", "lever:x"
	reposted.DescriptionText = adBody + " Apply via our careers page."

	differentTitle := reposted
	differentTitle.Title = "Engineering Manager (Payments)"

	sameTitleOnsite := reposted
	sameTitleOnsite.Remote = RemoteNo // the owner's real example: same title, one remote, one on-site

	sameTitleDifferentBody := reposted
	sameTitleDifferentBody.DescriptionText = strings.Repeat("Own our Kotlin payments platform and the on-call rota for it. ", 8)

	snippetNoLocation := email("2")
	snippetNoLocation.Location = ""

	cases := []struct {
		name string
		b    Posting
		want bool
	}{
		{"identical repost on another board", reposted, true},
		{"title differs (qualifier kept)", differentTitle, false},
		{"same title, different work model", sameTitleOnsite, false},
		{"same title, different body", sameTitleDifferentBody, false},
		{"email snippet with matching company/title/location", email("1"), true},
		{"email snippet without a location: unsure, don't merge", snippetNoLocation, false},
	}
	for _, c := range cases {
		if got := SameJob(a, c.b); got != c.want {
			t.Errorf("%s: SameJob = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGroup_PrefersATSCopyAndScoresOnce(t *testing.T) {
	e := email("1")
	a := ats("1")
	other := ats("2")
	other.Title = "Staff Engineer"

	canonical, dup := Group([]Posting{e, a, other}) // email first on purpose
	if len(canonical) != 2 {
		t.Fatalf("want 2 jobs to score, got %d", len(canonical))
	}
	if dup[e.ID] != a.ID {
		t.Errorf("the email copy should point at the ATS copy, got %q", dup[e.ID])
	}
	if _, isDup := dup[a.ID]; isDup {
		t.Errorf("the ATS copy must be canonical")
	}
}

func TestGroup_SameSourceIsNeverMerged(t *testing.T) {
	// Two genuinely separate ads on the same board can share title and text
	// (e.g. one per office); the board's own ids already tell them apart.
	a, b := ats("1"), ats("2")
	canonical, _ := Group([]Posting{a, b})
	if len(canonical) != 2 {
		t.Errorf("same-source postings must not be merged, got %d", len(canonical))
	}
}

package scorer

import (
	"context"
	"slices"
	"testing"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
)

func testRules() config.Rules {
	return config.Rules{
		Tracks: map[string]config.Track{
			"manager": {Titles: []string{"engineering manager", "director of engineering"}},
			"lead_ic": {Titles: []string{"staff engineer", "senior frontend", "senior ui engineer"}},
			"tpm":     {Titles: []string{"technical program manager"}},
		},
		MustHaveAny: []string{"react", "typescript", "node"},
		NiceToHave:  []string{"startup", "ai"},
		RedFlags: map[string][]string{
			"wants_go": {"golang", "hands-on go"},
		},
		Thresholds: config.Thresholds{Alert: 4, Digest: 3},
	}
}

func homeLocation() config.Location {
	return config.Location{
		HomeArea:     "Manchester",
		CountsAsHome: []string{"Manchester"},
		AllowedWorkModels: config.AllowedWorkModels{
			Hybrid: "home_only",
			Onsite: "home_only",
		},
	}
}

func score(t *testing.T, s *Rules, p posting.Posting) Result {
	t.Helper()
	r, err := s.Score(context.Background(), p)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if r.Scorer != ScorerRules {
		t.Errorf("Scorer = %q", r.Scorer)
	}
	if len(r.Breakdown) == 0 {
		t.Errorf("Breakdown should not be empty")
	}
	return r
}

func TestScore_StrongManagerFit(t *testing.T) {
	s := NewRules(testRules(), homeLocation())
	p := posting.Posting{
		Title:           "Engineering Manager",
		DescriptionText: "We use React and TypeScript at our startup. Hands-on Go a plus.",
		Remote:          posting.RemoteYes, // remote → location fine
	}
	r := score(t, s, p)
	// 1 +2 track +2 must(react,typescript cap) +1 nice(startup) −1 wants_go = 5
	if r.Score != 5 {
		t.Errorf("score = %d, want 5\nbreakdown: %v", r.Score, r.Breakdown)
	}
	if r.Track != "manager" {
		t.Errorf("track = %q, want manager", r.Track)
	}
	if !slices.Contains(r.Flags, "wants_go") {
		t.Errorf("flags = %v, want wants_go", r.Flags)
	}
}

func TestScore_PlainBackendIsLow(t *testing.T) {
	s := NewRules(testRules(), homeLocation())
	p := posting.Posting{
		Title:           "Senior Backend Engineer",
		DescriptionText: "Java, Spring, Postgres.",
		Remote:          posting.RemoteYes,
	}
	r := score(t, s, p)
	if r.Score != 1 {
		t.Errorf("score = %d, want 1\nbreakdown: %v", r.Score, r.Breakdown)
	}
	if r.Track != "none" {
		t.Errorf("track = %q, want none", r.Track)
	}
}

func TestScore_TPMTitle(t *testing.T) {
	s := NewRules(testRules(), homeLocation())
	r := score(t, s, posting.Posting{Title: "Technical Program Manager II", Remote: posting.RemoteYes})
	if r.Score != 3 || r.Track != "tpm" {
		t.Errorf("got score=%d track=%q, want 3/tpm\nbreakdown: %v", r.Score, r.Track, r.Breakdown)
	}
}

func TestScore_LeadICTweaks(t *testing.T) {
	s := NewRules(testRules(), homeLocation())
	for _, title := range []string{"Senior UI Engineer", "Staff Engineer"} {
		r := score(t, s, posting.Posting{Title: title, Remote: posting.RemoteYes})
		if r.Track != "lead_ic" {
			t.Errorf("%q: track = %q, want lead_ic", title, r.Track)
		}
	}
}

func TestScore_DirectorOfEngineeringIsManager(t *testing.T) {
	s := NewRules(testRules(), homeLocation())
	r := score(t, s, posting.Posting{Title: "Director of Engineering", Remote: posting.RemoteYes})
	if r.Track != "manager" {
		t.Errorf("track = %q, want manager", r.Track)
	}
}

func TestScore_OnsiteOutsideHomeIsCappedAndFlagged(t *testing.T) {
	s := NewRules(testRules(), homeLocation())
	p := posting.Posting{
		Title:    "Engineering Manager", // would score 3 on fit alone
		Remote:   posting.RemoteNo,      // onsite
		Location: "Berlin, Germany",
	}
	r := score(t, s, p)
	if r.Score > locationCap {
		t.Errorf("onsite role outside home should be capped at %d, got %d\nbreakdown: %v", locationCap, r.Score, r.Breakdown)
	}
	if !slices.Contains(r.Flags, FlagOutsideHomeArea) {
		t.Errorf("expected %s flag, got %v", FlagOutsideHomeArea, r.Flags)
	}
	if r.WorkModel != "onsite" {
		t.Errorf("work model = %q, want onsite", r.WorkModel)
	}
}

func TestScore_OnsiteAtHomeIsFine(t *testing.T) {
	s := NewRules(testRules(), homeLocation())
	p := posting.Posting{Title: "Engineering Manager", Remote: posting.RemoteNo, Location: "Manchester, UK"}
	r := score(t, s, p)
	if slices.Contains(r.Flags, FlagOutsideHomeArea) {
		t.Errorf("home onsite role should not be flagged outside home: %v", r.Flags)
	}
	if r.Score != 3 {
		t.Errorf("score = %d, want 3", r.Score)
	}
}

func TestScore_NoLocationRulesNeverPenalises(t *testing.T) {
	s := NewRules(testRules(), config.Location{}) // no location config
	r := score(t, s, posting.Posting{Title: "Engineering Manager", Remote: posting.RemoteNo, Location: "Mars"})
	if slices.Contains(r.Flags, FlagOutsideHomeArea) {
		t.Errorf("with no location rules, nothing should be flagged outside home")
	}
}

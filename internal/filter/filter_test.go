package filter

import (
	"testing"
	"time"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
)

func titleOnly(title string) posting.Posting { return posting.Posting{Title: title} }

// TestRequiredTitles covers the exact keep/drop titles the plan requires
// (BUILD_PLAN §6, "My pre-filter rules").
func TestRequiredTitles(t *testing.T) {
	f := New(config.PreFilter{}) // defaults

	mustKeep := []string{
		"Senior Frontend Engineer",
		"Senior Software Engineer, Frontend",
		"Senior Full Stack Developer (React)",
		"Staff Frontend Engineer",
		"Tech Lead",
		"Engineering Manager",
		"Technical Program Manager II",
	}
	for _, title := range mustKeep {
		if d := f.Check(titleOnly(title)); !d.Keep {
			t.Errorf("KEEP expected but dropped: %q (%s)", title, d.Reason)
		}
	}

	mustDrop := []string{
		"Junior Frontend Developer",
		"Director of Engineering",
		"Engineering Manager – Rail",
	}
	for _, title := range mustDrop {
		if d := f.Check(titleOnly(title)); d.Keep {
			t.Errorf("DROP expected but kept: %q (%s)", title, d.Reason)
		}
	}
}

// TestNonSoftwareFromDescription stands in for the WSP rail ad (whose real text
// is in the owner's private eval set): a software-sounding title with a rail /
// civil-engineering body must be dropped.
func TestNonSoftwareFromDescription(t *testing.T) {
	f := New(config.PreFilter{})
	p := posting.Posting{
		Title:           "Engineering Manager",
		DescriptionText: "Lead a team delivering railway signalling projects; chartered engineer status and CDM experience required.",
	}
	if d := f.Check(p); d.Keep {
		t.Errorf("expected drop for a rail-engineering body, kept (%s)", d.Reason)
	}
}

func TestWholeWordMatching(t *testing.T) {
	f := New(config.PreFilter{})
	// "intern" must not match "International" / "Internal".
	for _, title := range []string{"Internal Tools Engineer", "International Payments Engineer"} {
		if d := f.Check(titleOnly(title)); !d.Keep {
			t.Errorf("whole-word matching failed, wrongly dropped %q (%s)", title, d.Reason)
		}
	}
	// "rail" must not match "Rails" (e.g. Ruby on Rails).
	if d := f.Check(titleOnly("Senior Ruby on Rails Engineer")); !d.Keep {
		t.Errorf("wrongly dropped a Rails role (%s)", d.Reason)
	}
}

func TestKeepOverridesDrop(t *testing.T) {
	// A configured keep pattern rescues a title that also matches a drop pattern.
	f := New(config.PreFilter{
		TitleKeep: []string{"engineering manager"},
		TitleDrop: []string{"manager"},
	})
	if d := f.Check(titleOnly("Engineering Manager")); !d.Keep {
		t.Errorf("keep pattern should override title drop, got drop (%s)", d.Reason)
	}
	// But a plain "Account Manager" still drops on "manager".
	if d := f.Check(titleOnly("Account Manager")); d.Keep {
		t.Errorf("expected drop for Account Manager (%s)", d.Reason)
	}
}

func TestNotSoftwareOverridesKeep(t *testing.T) {
	f := New(config.PreFilter{TitleKeep: []string{"engineering manager"}})
	if d := f.Check(titleOnly("Engineering Manager – Rail")); d.Keep {
		t.Errorf("non-software should override a keep, got keep (%s)", d.Reason)
	}
}

// TestAgeNeverDrops locks in ADR-0025: an old posting is treated exactly like a
// fresh one by the pre-filter.
func TestAgeNeverDrops(t *testing.T) {
	f := New(config.PreFilter{})
	old := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	p := posting.Posting{Title: "Senior Frontend Engineer", PostedAt: &old}
	if d := f.Check(p); !d.Keep {
		t.Errorf("an old but matching role must still be kept (%s)", d.Reason)
	}
}

func TestDefaultKeepWhenInDoubt(t *testing.T) {
	f := New(config.PreFilter{})
	// A title that matches no rule at all is kept.
	if d := f.Check(titleOnly("Developer Advocate")); !d.Keep {
		t.Errorf("unmatched title should be kept by default (%s)", d.Reason)
	}
}

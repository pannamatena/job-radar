// Package filter is the cheap, deterministic pre-filter that runs before any
// scoring. Its only job is to drop obvious misses — junior/director titles,
// non-software engineering — so the scorer (and, later, the paid LLM) only sees
// plausible roles.
//
// Two principles, both from the decision records:
//   - When in doubt, keep (ADR-0012). The default decision is Keep; a posting is
//     only dropped when a rule is sure. A wrongly dropped role is never seen; a
//     wrongly kept one costs at most one cheap scorer pass.
//   - Age is irrelevant (ADR-0025). The pre-filter never looks at a posting's
//     date.
//
// Rules come from the user's config, with sensible software-role defaults.
package filter

import (
	"regexp"
	"strings"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
)

// DefaultTitleDrop drops titles that are clearly the wrong level or kind for a
// software IC/lead/manager search.
var DefaultTitleDrop = []string{
	"junior", "graduate", "intern", "internship", "trainee", "apprentice",
	"mid-level", "mid level",
	"director", "vp", "vice president",
	"scrum master",
	"staff technical program manager", "principal technical program manager",
}

// DefaultNotSoftware drops non-software engineering disciplines, matched in the
// title or description.
var DefaultNotSoftware = []string{
	"rail", "railway", "signalling", "signaling", "rolling stock", "permanent way",
	"chartered engineer", "civil engineer", "structural engineer",
	"mechanical engineer", "electrical engineer", "cdm",
}

// Filter holds the compiled rules.
type Filter struct {
	keep        []*regexp.Regexp
	drop        []*regexp.Regexp
	notSoftware []*regexp.Regexp
}

// Decision is the outcome for one posting.
type Decision struct {
	Keep   bool
	Reason string // short human explanation, e.g. `drop: title matches "director"`
}

// New compiles a Filter from config, applying defaults to any empty list.
func New(pf config.PreFilter) *Filter {
	drop := pf.TitleDrop
	if len(drop) == 0 {
		drop = DefaultTitleDrop
	}
	notSW := pf.NotSoftware
	if len(notSW) == 0 {
		notSW = DefaultNotSoftware
	}
	return &Filter{
		keep:        compileWordPatterns(pf.TitleKeep),
		drop:        compileWordPatterns(drop),
		notSoftware: compileWordPatterns(notSW),
	}
}

// compileWordPatterns builds case-insensitive, whole-word regexes. Whole-word
// matching matters: a naive substring match for "intern" would wrongly hit
// "internal", and "vp" would hit nothing sane without boundaries.
func compileWordPatterns(patterns []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(p)+`\b`))
	}
	return out
}

// Check decides whether to keep a posting. Precedence:
//  1. a non-software keyword (in title or description) → drop (overrides keep);
//  2. a keep pattern in the title → keep (overrides a title-drop);
//  3. a drop pattern in the title → drop;
//  4. otherwise → keep (the default, ADR-0012).
func (f *Filter) Check(p posting.Posting) Decision {
	title := p.Title

	if m := firstMatch(f.notSoftware, title); m != "" {
		return Decision{Keep: false, Reason: `drop: not software (title mentions "` + m + `")`}
	}
	if m := firstMatch(f.notSoftware, p.DescriptionText); m != "" {
		return Decision{Keep: false, Reason: `drop: not software (description mentions "` + m + `")`}
	}
	if m := firstMatch(f.keep, title); m != "" {
		return Decision{Keep: true, Reason: `keep: title matches "` + m + `"`}
	}
	if m := firstMatch(f.drop, title); m != "" {
		return Decision{Keep: false, Reason: `drop: title matches "` + m + `"`}
	}
	return Decision{Keep: true, Reason: "keep: no drop rule matched"}
}

// firstMatch returns the matched substring of the first regex that hits, or "".
func firstMatch(res []*regexp.Regexp, s string) string {
	for _, re := range res {
		if m := re.FindString(s); m != "" {
			return m
		}
	}
	return ""
}

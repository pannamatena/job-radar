package scorer

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
)

// ScorerRules is the name recorded on results from the rules scorer.
const ScorerRules = "rules"

// Score caps.
const (
	minScore           = 1
	maxScore           = 5
	baseScore          = 1
	trackBoost         = 2
	mustHaveBoostCap   = 2
	niceToHaveBoostCap = 1
	// locationCap is the highest score a role outside the user's location rules
	// can get — the rubric treats a location miss as a 1–2 (ADR-aligned).
	locationCap = 2
)

// Rules is the free, deterministic scorer. It makes no network calls.
type Rules struct {
	cfg      config.Rules
	loc      config.Location
	tracks   []compiledTrack // sorted by name for determinism
	mustHave []keyword
	niceHave []keyword
	redFlags []compiledFlag // sorted by name for determinism
}

type compiledTrack struct {
	name     string
	patterns []keyword
}

type compiledFlag struct {
	name     string
	patterns []keyword
}

type keyword struct {
	text string
	re   *regexp.Regexp
}

// NewRules builds a rules scorer from the user's rules and location config.
func NewRules(rules config.Rules, loc config.Location) *Rules {
	s := &Rules{cfg: rules, loc: loc}

	names := make([]string, 0, len(rules.Tracks))
	for name := range rules.Tracks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s.tracks = append(s.tracks, compiledTrack{name: name, patterns: compile(rules.Tracks[name].Titles)})
	}

	s.mustHave = compile(rules.MustHaveAny)
	s.niceHave = compile(rules.NiceToHave)

	flagNames := make([]string, 0, len(rules.RedFlags))
	for name := range rules.RedFlags {
		flagNames = append(flagNames, name)
	}
	sort.Strings(flagNames)
	for _, name := range flagNames {
		s.redFlags = append(s.redFlags, compiledFlag{name: name, patterns: compile(rules.RedFlags[name])})
	}
	return s
}

func compile(patterns []string) []keyword {
	out := make([]keyword, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, keyword{text: p, re: regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(p) + `\b`)})
	}
	return out
}

// Score applies the formula: 1 + (2 if a track matches) + up to 2 must_have +
// up to 1 nice_to_have − 1 per red flag, clamped to 1–5; then a location miss
// caps the score. A posting's age is never considered (ADR-0025).
func (s *Rules) Score(_ context.Context, p posting.Posting) (Result, error) {
	titleLower := strings.ToLower(p.Title)
	textLower := strings.ToLower(p.Title + "\n" + p.DescriptionText)

	var breakdown []string
	score := baseScore
	breakdown = append(breakdown, "+1 base")

	// Track match (strongest signal): pick the longest matching title phrase.
	track, trackPhrase := s.bestTrack(titleLower)
	if track != "" {
		score += trackBoost
		breakdown = append(breakdown, fmt.Sprintf("+%d title matches %s track (%q)", trackBoost, track, trackPhrase))
	} else {
		track = "none"
	}

	// must_have_any boosts (capped).
	mustHits := matches(s.mustHave, textLower)
	if n := len(mustHits); n > 0 {
		boost := min(n, mustHaveBoostCap)
		score += boost
		breakdown = append(breakdown, fmt.Sprintf("+%d must-have (%s)", boost, strings.Join(mustHits, ", ")))
	}

	// nice_to_have boost (capped at +1 total).
	niceHits := matches(s.niceHave, textLower)
	if len(niceHits) > 0 {
		score += niceToHaveBoostCap
		breakdown = append(breakdown, fmt.Sprintf("+%d nice-to-have (%s)", niceToHaveBoostCap, strings.Join(niceHits, ", ")))
	}

	// red flags: each distinct flag matched costs 1 and is recorded.
	var flags []string
	for _, f := range s.redFlags {
		if hits := matches(f.patterns, textLower); len(hits) > 0 {
			score--
			flags = append(flags, f.name)
			breakdown = append(breakdown, fmt.Sprintf("−1 red flag %s (%q)", f.name, hits[0]))
		}
	}

	score = clamp(score, minScore, maxScore)

	// Work model + location rules.
	workModel := workModelOf(p.Remote)
	if ok, flag := s.locationFit(p.Location, workModel); !ok {
		flags = append(flags, flag)
		if score > locationCap {
			breakdown = append(breakdown, fmt.Sprintf("capped at %d (%s)", locationCap, flag))
			score = locationCap
		} else {
			breakdown = append(breakdown, "flag: "+flag)
		}
	}

	return Result{
		Score:     score,
		Track:     track,
		WorkModel: workModel,
		Office:    p.Location,
		Why:       s.why(track, trackPhrase, mustHits, niceHits),
		Gap:       s.gap(flags),
		Flags:     flags,
		Breakdown: breakdown,
		Scorer:    ScorerRules,
	}, nil
}

// bestTrack returns the track whose longest title phrase matches the title.
func (s *Rules) bestTrack(titleLower string) (name, phrase string) {
	best := -1
	for _, t := range s.tracks {
		for _, kw := range t.patterns {
			if kw.re.MatchString(titleLower) && len(kw.text) > best {
				best = len(kw.text)
				name = t.name
				phrase = kw.text
			}
		}
	}
	return name, phrase
}

// locationFit decides whether a posting's location fits the user's rules and,
// if not, which flag explains why. All of it is driven by config, so it works
// for any location (not just a hard-coded US/UK): hybrid/onsite are judged by
// the user's home cities (counts_as_home), remote by the regions the user can
// work in (allowed_work_models.remote_open_to).
func (s *Rules) locationFit(location, workModel string) (ok bool, flag string) {
	// No location rules configured → never penalise.
	if s.loc.HomeArea == "" && len(s.loc.CountsAsHome) == 0 {
		return true, ""
	}
	switch workModel {
	case "not_stated":
		// A missing work model is never a reason to penalise (ADR-0012).
		return true, ""
	case "remote":
		if s.remoteOK(location) {
			return true, ""
		}
		return false, FlagOutsideEligibleRegion
	case "hybrid":
		if s.loc.AllowedWorkModels.Hybrid == "any" || s.isHome(location) {
			return true, ""
		}
		return false, FlagOutsideHomeArea
	case "onsite":
		if s.loc.AllowedWorkModels.Onsite == "any" || s.isHome(location) {
			return true, ""
		}
		return false, FlagOutsideHomeArea
	}
	return true, ""
}

// remoteOK reports whether a remote role is open to a region the user can work
// in. It keeps the role if the location names an allowed region, is a home
// city, or is a bare "Remote" with no region (unknown → keep, ADR-0012). It
// only rejects a remote role that names a specific region the user is not open
// to (e.g. "United States", "Remote (UK)").
func (s *Rules) remoteOK(location string) bool {
	open := s.loc.AllowedWorkModels.RemoteOpenTo
	if len(open) == 0 {
		return true // no remote restriction configured
	}
	l := strings.ToLower(location)
	for _, r := range open {
		if rl := strings.ToLower(strings.TrimSpace(r)); rl != "" && strings.Contains(l, rl) {
			return true
		}
	}
	if s.isHome(location) {
		return true
	}
	return isGenericRemote(l)
}

// isGenericRemote reports whether a location is just "remote" with no specific
// place named (so we can't tell it's outside the user's regions → keep it).
func isGenericRemote(lower string) bool {
	filler := []string{
		"remote", "first", "friendly", "anywhere", "worldwide", "global",
		"distributed", "flexible", "work from home", "wfh", "fully", "hybrid",
	}
	stripped := lower
	for _, f := range filler {
		stripped = strings.ReplaceAll(stripped, f, " ")
	}
	// Keep only letters; if nothing meaningful remains, it's a generic "remote".
	var b strings.Builder
	for _, r := range stripped {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String()) == ""
}

// isHome reports whether a location counts as the user's home area.
func (s *Rules) isHome(location string) bool {
	l := strings.ToLower(location)
	homes := s.loc.CountsAsHome
	if len(homes) == 0 && s.loc.HomeArea != "" {
		homes = []string{s.loc.HomeArea}
	}
	for _, h := range homes {
		if hl := strings.ToLower(strings.TrimSpace(h)); hl != "" && strings.Contains(l, hl) {
			return true
		}
	}
	return false
}

func (s *Rules) why(track, phrase string, must, nice []string) string {
	var parts []string
	if track != "none" {
		parts = append(parts, fmt.Sprintf("%s-track title %q", track, phrase))
	}
	if len(must) > 0 {
		parts = append(parts, "mentions "+strings.Join(must, ", "))
	} else if len(nice) > 0 {
		parts = append(parts, "mentions "+strings.Join(nice, ", "))
	}
	if len(parts) == 0 {
		return "no track title or key skills matched (rules)"
	}
	return trim(strings.Join(parts, "; ")+" (rules)", 200)
}

func (s *Rules) gap(flags []string) string {
	if len(flags) == 0 {
		return "no obvious gaps from keywords (rules)"
	}
	return trim("flags: "+strings.Join(flags, ", ")+" (rules)", 200)
}

// matches returns the matched keyword texts (deduped, in pattern order).
func matches(kws []keyword, text string) []string {
	var out []string
	for _, kw := range kws {
		if kw.re.MatchString(text) {
			out = append(out, kw.text)
		}
	}
	return out
}

func workModelOf(r posting.RemoteKind) string {
	switch r {
	case posting.RemoteYes:
		return "remote"
	case posting.RemoteHybrid:
		return "hybrid"
	case posting.RemoteNo:
		return "onsite"
	default:
		return "not_stated"
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

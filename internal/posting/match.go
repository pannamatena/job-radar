package posting

import (
	"sort"
	"strings"
)

// Cross-source duplicate matching (ADR-0026). The same job can reach us from
// the company's own board and, later, from a job-alert email, under different
// ids. We match them so each job is scored (and paid for) once and alerted
// once. The rule, in order:
//
//  1. Different company or different title → different jobs. Titles are only
//     lightly normalised (case, spacing), never stripped of qualifiers.
//  2. Same title: if both copies state a work model and they differ, or both
//     state a location and they differ → different jobs.
//  3. Same title: if both have a real description, near-identical wording
//     means the same job; clearly different wording means different jobs.
//  4. If one copy is only a snippet (an alert email), lean on company + title
//     + location agreeing — that needs a location on both.
//  5. Still unsure → don't merge. A duplicate alert is mild noise; a wrongly
//     merged role is a role the user never sees.

// minBodyRunes is the length below which a description counts as a snippet.
const minBodyRunes = 300

// sameBodyThreshold is the word-shingle overlap (Jaccard) above which two
// descriptions count as the same ad. Reposts are usually word-for-word, with
// small differences in boilerplate; 0.8 tolerates that while different roles
// that share a template ("About us…") stay well below it.
const sameBodyThreshold = 0.8

// SameJob reports whether two postings from different sources are the same job.
func SameJob(a, b Posting) bool {
	if a.ID == b.ID {
		return true
	}
	if normText(a.Company) != normText(b.Company) || normText(a.Title) != normText(b.Title) {
		return false
	}
	if a.Remote != RemoteUnknown && b.Remote != RemoteUnknown && a.Remote != b.Remote {
		return false
	}
	locA, locB := normText(a.Location), normText(b.Location)
	if locA != "" && locB != "" && locA != locB {
		return false
	}

	longA := len([]rune(a.DescriptionText)) >= minBodyRunes
	longB := len([]rune(b.DescriptionText)) >= minBodyRunes
	if longA && longB {
		return shingleSimilarity(a.DescriptionText, b.DescriptionText) >= sameBodyThreshold
	}
	// At least one snippet: only merge when the location is known on both and
	// agrees (checked above).
	return locA != "" && locB != ""
}

// Group finds duplicates within a batch. It returns the postings to score
// (one canonical copy per job) and, for every duplicate, the id of its
// canonical copy. The canonical copy is the ATS board's (richer, with a stable
// apply link); among equals, the one with the longer description, then the
// smaller id, so the choice is deterministic.
func Group(ps []Posting) (canonical []Posting, duplicateOf map[string]string) {
	duplicateOf = map[string]string{}
	ordered := make([]Posting, len(ps))
	copy(ordered, ps)
	sort.SliceStable(ordered, func(i, j int) bool { return preferred(ordered[i], ordered[j]) })

	for _, p := range ordered {
		matched := false
		for _, c := range canonical {
			if c.Source != p.Source && SameJob(c, p) {
				duplicateOf[p.ID] = c.ID
				matched = true
				break
			}
		}
		if !matched {
			canonical = append(canonical, p)
		}
	}
	return canonical, duplicateOf
}

// preferred reports whether a should be the canonical copy over b.
func preferred(a, b Posting) bool {
	if ra, rb := sourceRank(a.Source), sourceRank(b.Source); ra != rb {
		return ra < rb
	}
	if la, lb := len(a.DescriptionText), len(b.DescriptionText); la != lb {
		return la > lb
	}
	return a.ID < b.ID
}

// sourceRank: ATS boards first, anything else (alert emails) after.
func sourceRank(source string) int {
	switch source {
	case "greenhouse", "lever", "ashby":
		return 0
	}
	return 1
}

func normText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// shingleSimilarity is the Jaccard overlap of the two texts' 3-word shingles
// (every run of three consecutive words). Unlike comparing word sets, it
// notices when the same words appear in a different order or context.
func shingleSimilarity(a, b string) float64 {
	sa, sb := shingles(a), shingles(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for s := range sa {
		if sb[s] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	return float64(inter) / float64(union)
}

func shingles(text string) map[string]bool {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
	})
	out := map[string]bool{}
	for i := 0; i+3 <= len(words); i++ {
		out[words[i]+" "+words[i+1]+" "+words[i+2]] = true
	}
	return out
}

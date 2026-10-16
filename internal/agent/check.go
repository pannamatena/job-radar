package agent

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pannamatena/job-radar/internal/config"
)

// backticked picks out names written in `backticks` that look like flag names
// (words joined by underscores, e.g. `wants_go`).
var backticked = regexp.MustCompile("`([a-z]+(?:_[a-z]+)+)`")

// flagPrefixes are how flag names usually start. Only backticked names that
// start like this are treated as "this looks like a flag", so the rubric can
// still mention things like `record_score` or `work_model` without a warning.
var flagPrefixes = []string{"wants_", "needs_", "outside_", "not_", "too_", "no_", "unqualified_", "limited_"}

// CheckRubric compares the names used in rubric.md with the config and returns
// plain-language warnings. It exists because the rubric and config.yaml must
// agree on track and flag names, and a mismatch is easy to make and hard to see:
// the AI can only record names from the config, so a flag the rubric describes
// but the config doesn't define can never be set.
func CheckRubric(rubric string, cfg *config.Config) []string {
	allowed := cfg.AllowedFlags()
	var warns []string

	// 1. A flag-looking name in the rubric that the config doesn't define.
	seen := map[string]bool{}
	for _, m := range backticked.FindAllStringSubmatch(rubric, -1) {
		name := m[1]
		if seen[name] || slices.Contains(allowed, name) || !hasFlagPrefix(name) {
			continue
		}
		seen[name] = true
		warns = append(warns, fmt.Sprintf(
			"rubric.md describes `%s`, but config.yaml doesn't define it, so the AI can't set it. Add it under rules.red_flags (an empty list %s: [] is fine) or remove it from the rubric.", name, name))
	}

	// 2. A track or your own flag that the config defines but the rubric never mentions.
	var defined []string
	for name := range cfg.Rules.RedFlags {
		defined = append(defined, name)
	}
	defined = append(defined, cfg.TrackNames()...)
	sort.Strings(defined)
	for _, name := range defined {
		if !strings.Contains(rubric, "`"+name+"`") {
			warns = append(warns, fmt.Sprintf(
				"config.yaml defines `%s`, but rubric.md never mentions it (write it in `backticks`), so the AI won't know when to use it.", name))
		}
	}
	return warns
}

func hasFlagPrefix(name string) bool {
	for _, p := range flagPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

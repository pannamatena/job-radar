package search

import (
	"fmt"
	"sort"
	"strings"
)

// Factory builds a Searcher for a model. Each provider supplies one; it reads
// its own credentials (for example an API key from the environment) and
// returns a clear error when they are missing.
type Factory func(model string) (Searcher, error)

// providers lists every search provider job-radar can use. To add one:
//  1. write a type that implements Searcher in its own file (see anthropic.go),
//  2. add one line here with its name,
//  3. add a test that uses a fake server, as anthropic_test.go does,
//  4. mention it in docs/finding-jobs-by-search.md.
//
// Nothing else changes: the strict link check, the board reads, the scoring and
// the digest do not depend on which provider found the links.
var providers = map[string]Factory{
	"anthropic": func(model string) (Searcher, error) {
		s, err := NewAnthropic(model)
		if err != nil {
			return nil, err // not `return s, err`: that would be a non-nil interface holding a nil pointer
		}
		return s, nil
	},
}

// Providers returns the names job-radar can search with, sorted.
func Providers() []string {
	names := make([]string, 0, len(providers))
	for n := range providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// New builds the Searcher for a provider name from config (search.provider).
func New(provider, model string) (Searcher, error) {
	f, ok := providers[strings.ToLower(strings.TrimSpace(provider))]
	if !ok {
		return nil, fmt.Errorf("search.provider %q is not supported (have: %s)", provider, strings.Join(Providers(), ", "))
	}
	return f(model)
}

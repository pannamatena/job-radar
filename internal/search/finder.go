package search

import (
	"context"
	"net/url"
	"strings"
	"unicode"

	"github.com/pannamatena/job-radar/internal/config"
)

// Lead is a search result that is not a posting on a known job board. It is
// never fetched or scored. A lead on a well-known host (Known) is shown as a
// normal link; every other lead is shown as text that can't become a link.
type Lead struct {
	URL   string
	Host  string
	Title string
	Known bool
}

// Found is everything a set of queries produced.
type Found struct {
	Refs    []Ref // postings on known job boards, once each
	Leads   []Lead
	Queries int
	Usage   Usage
	CostUSD float64
	Stopped string   // why it stopped early, if it did
	Errors  []string // queries that failed (the others still ran)
}

// Finder runs queries and sorts what comes back.
type Finder struct {
	Searcher Searcher
	Cfg      config.Search
	Price    config.ModelPrice // the search model's token prices
}

// PickQueries chooses this run's queries: all of them, or perRun of them
// starting start places into the list and wrapping round, so that over several
// runs every query is used.
func PickQueries(all []string, perRun, start int) []string {
	if len(all) == 0 {
		return nil
	}
	if perRun <= 0 || perRun >= len(all) {
		return all
	}
	out := make([]string, 0, perRun)
	for i := 0; i < perRun; i++ {
		out = append(out, all[(start+i)%len(all)])
	}
	return out
}

// Run runs the queries until they are done or budgetLeftUSD is used up, and
// sorts every link: a posting on a known job board becomes a Ref, anything
// else a Lead, and anything from a blocked site is dropped.
func (f *Finder) Run(ctx context.Context, queries []string, budgetLeftUSD float64) Found {
	var out Found
	seenRef := map[string]bool{}
	seenLead := map[string]bool{}
	for _, q := range queries {
		if ctx.Err() != nil {
			out.Stopped = "cancelled"
			break
		}
		if out.CostUSD >= budgetLeftUSD {
			out.Stopped = "the monthly search spend cap was reached"
			break
		}
		res, err := f.Searcher.Search(ctx, q, Options{MaxSearches: f.Cfg.MaxSearchesPerQuery, BlockedDomains: f.Cfg.BlockedDomains})
		out.Queries++
		out.Usage = out.Usage.Add(res.Usage)
		out.CostUSD = out.Usage.CostUSD(f.Price, f.Cfg.PricePerSearch())
		if err != nil {
			out.Errors = append(out.Errors, err.Error())
			continue
		}
		for _, c := range res.Candidates {
			if ref, ok := ParseJobURL(c.URL); ok {
				if !seenRef[ref.PostingID()] {
					seenRef[ref.PostingID()] = true
					out.Refs = append(out.Refs, ref)
				}
				continue
			}
			if lead, ok := f.lead(c); ok && !seenLead[lead.URL] {
				seenLead[lead.URL] = true
				out.Leads = append(out.Leads, lead)
			}
		}
	}
	return out
}

// lead turns a candidate into a Lead, or drops it (unusable link, blocked site).
func (f *Finder) lead(c Candidate) (Lead, bool) {
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return Lead{}, false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || hostMatches(host, f.Cfg.BlockedDomains) {
		return Lead{}, false
	}
	// Only an https link on a well-known host is ever shown as a link, and only
	// if it is plain ASCII with no control characters or spaces.
	known := u.Scheme == "https" && isPlainLink(c.URL) && hostMatches(host, f.Cfg.LinkDomains)
	return Lead{URL: c.URL, Host: host, Title: cleanTitle(c.Title), Known: known}, true
}

// hostMatches reports whether host is one of the domains or a sub-domain of one.
func hostMatches(host string, domains []string) bool {
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" && (host == d || strings.HasSuffix(host, "."+d)) {
			return true
		}
	}
	return false
}

func isPlainLink(s string) bool {
	if len(s) > maxURLLength {
		return false
	}
	for _, r := range s {
		if r <= 0x20 || r >= 0x7f {
			return false
		}
	}
	return true
}

// cleanTitle makes a result's title safe to print: no control characters, one
// line, a sensible length. (It is still escaped wherever it is shown.)
func cleanTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 160 {
		s = string(r[:160]) + "…"
	}
	return s
}

// NormalName lowercases a company name and drops everything but letters and
// digits, so "Acme Labs", "acme-labs" and "AcmeLabs" match.
func NormalName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Package search finds job postings by web search (ADR-0037).
//
// A search returns links, and links in search results are text anyone on the
// internet can write. This package therefore never opens one. ParseJobURL
// checks a link strictly and, if (and only if) it points at a posting on a
// known job-board host, returns the company name and job id from it. The
// caller then asks that board's official API for the posting.
package search

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
)

// Ref names one posting on a job board: enough to ask the board's API for it.
type Ref struct {
	ATS   string // config.ATSAshby, ATSGreenhouse or ATSLever
	Slug  string // the company's name on that board, as the board spells it
	JobID string
}

// PostingID is the id the same job gets when it is read from a watch-list
// board, so a job found by search and by the watch-list is one job.
func (r Ref) PostingID() string {
	return posting.MakeID(r.ATS, r.Slug+"/"+r.JobID)
}

// maxURLLength is generous for a real job link and small enough to reject junk.
const maxURLLength = 500

var (
	uuidRe   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	digitsRe = regexp.MustCompile(`^[0-9]{1,20}$`)
	// slugRe is deliberately narrow: letters, digits, space, dot, dash and
	// underscore. It is used after the path has been percent-decoded.
	slugRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,99}$`)
)

// boardHosts maps each accepted host to its job board. The match is exact:
// "jobs.ashbyhq.com.example.net" and "xjobs.ashbyhq.com" are not on this list.
var boardHosts = map[string]string{
	"jobs.ashbyhq.com":            config.ATSAshby,
	"boards.greenhouse.io":        config.ATSGreenhouse,
	"job-boards.greenhouse.io":    config.ATSGreenhouse,
	"job-boards.eu.greenhouse.io": config.ATSGreenhouse,
	"jobs.lever.co":               config.ATSLever,
}

// ParseJobURL returns the posting a link points at, or false if the link is
// not, strictly, a posting on a known job-board host. It never fetches anything.
func ParseJobURL(raw string) (Ref, bool) {
	if raw == "" || len(raw) > maxURLLength {
		return Ref{}, false
	}
	for _, r := range raw {
		// Control characters, spaces and anything outside ASCII are never in a
		// genuine link here (non-ASCII hosts are how lookalike domains work).
		if r <= 0x20 || r >= 0x7f {
			return Ref{}, false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" {
		return Ref{}, false
	}
	ats, ok := boardHosts[strings.ToLower(u.Hostname())]
	if !ok {
		return Ref{}, false
	}
	// u.EscapedPath keeps %20 and friends as written; split on "/" before decoding
	// so an encoded slash can't smuggle in an extra path segment.
	path := strings.Trim(u.EscapedPath(), "/")
	parts := strings.Split(path, "/")
	switch ats {
	case config.ATSGreenhouse:
		// /{company}/jobs/{numeric id}
		if len(parts) != 3 || parts[1] != "jobs" || !digitsRe.MatchString(parts[2]) {
			return Ref{}, false
		}
		return build(ats, parts[0], parts[2])
	case config.ATSAshby, config.ATSLever:
		// /{company}/{uuid}, optionally followed by /application (Ashby) or /apply (Lever)
		if len(parts) == 3 && (parts[2] == "application" || parts[2] == "apply") {
			parts = parts[:2]
		}
		if len(parts) != 2 || !uuidRe.MatchString(parts[1]) {
			return Ref{}, false
		}
		return build(ats, parts[0], strings.ToLower(parts[1]))
	}
	return Ref{}, false
}

func build(ats, rawSlug, id string) (Ref, bool) {
	slug, err := url.PathUnescape(rawSlug)
	if err != nil || !slugRe.MatchString(slug) || strings.Contains(slug, "..") {
		return Ref{}, false
	}
	return Ref{ATS: ats, Slug: slug, JobID: id}, true
}

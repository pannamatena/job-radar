package sources

import (
	"net/url"
	"regexp"
)

// Detection is the result of inspecting a careers page (and its URL) to work
// out which applicant-tracking system a company uses.
type Detection struct {
	ATS         string // "greenhouse" | "lever" | "ashby" when supported
	Slug        string // the board token / org handle
	Unsupported string // name of a detected ATS we can't read directly, if any
}

// Found reports whether a supported ATS + slug was detected.
func (d Detection) Found() bool { return d.ATS != "" && d.Slug != "" }

// supportedPatterns maps a supported ATS to a regex that captures its slug from
// either a board URL or an API URL. Checked against both the page body and the
// page's own URL (in case the user pasted the ATS link directly).
var supportedPatterns = []struct {
	ats string
	re  *regexp.Regexp
}{
	{sourceGreenhouse, regexp.MustCompile(`(?i)(?:boards(?:-api)?|job-boards)\.greenhouse\.io/(?:embed/job_board/js\?for=|embed/job_board\?for=|v1/boards/)?([A-Za-z0-9_.-]+)`)},
	{sourceLever, regexp.MustCompile(`(?i)(?:jobs\.lever\.co|api\.lever\.co/v0/postings)/([A-Za-z0-9_.-]+)`)},
	{sourceAshby, regexp.MustCompile(`(?i)(?:jobs\.ashbyhq\.com|api\.ashbyhq\.com/posting-api/job-board)/([A-Za-z0-9_.%-]+)`)},
}

// unsupportedPatterns recognises ATSes with no public feed we can use, so
// discover can say so clearly rather than failing silently.
var unsupportedPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"SmartRecruiters", regexp.MustCompile(`(?i)smartrecruiters\.com`)},
	{"Workday", regexp.MustCompile(`(?i)myworkdayjobs\.com|workday\.com`)},
	{"Workable", regexp.MustCompile(`(?i)workable\.com`)},
	{"Teamtailor", regexp.MustCompile(`(?i)teamtailor\.com`)},
	{"Personio", regexp.MustCompile(`(?i)personio\.(?:com|de)`)},
	{"BambooHR", regexp.MustCompile(`(?i)bamboohr\.com`)},
	{"Recruitee", regexp.MustCompile(`(?i)recruitee\.com`)},
	{"iCIMS", regexp.MustCompile(`(?i)icims\.com`)},
}

// Detect inspects a careers page URL and its HTML body for a known ATS.
func Detect(pageURL string, body []byte) Detection {
	haystacks := []string{pageURL, string(body)}

	for _, p := range supportedPatterns {
		for _, h := range haystacks {
			if m := p.re.FindStringSubmatch(h); m != nil {
				slug := m[1]
				if dec, err := url.QueryUnescape(slug); err == nil {
					slug = dec
				}
				return Detection{ATS: p.ats, Slug: slug}
			}
		}
	}
	for _, p := range unsupportedPatterns {
		for _, h := range haystacks {
			if p.re.MatchString(h) {
				return Detection{Unsupported: p.name}
			}
		}
	}
	return Detection{}
}

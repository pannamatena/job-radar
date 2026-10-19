package notify

import (
	"fmt"
	"regexp"
	"strings"
)

// Lead is a job link from a web search that job-radar did not read: it is not
// a posting on a known job board. It has no score. (ADR-0037)
type Lead struct {
	Title string
	Host  string // the site, e.g. "news.example.com"
	URL   string // the full link; only ever shown when Known
	Known bool   // a well-known site, shown as a normal link
}

// MaxLeadsShown caps how many leads one email lists.
const MaxLeadsShown = 10

// leadsWarning is shown above the leads. It is for readers who don't think
// about where a link goes.
const leadsWarning = `Warning: job-radar does not know the sites marked "unknown site" and has not opened them. ` +
	`Check that a site is safe before you open it. Their addresses are written like example[.]com so that ` +
	`they can't be clicked by accident; type the address in yourself if you want to look.`

var dotBetweenLetters = regexp.MustCompile(`([A-Za-z0-9])\.([A-Za-z])`)

// Defang writes a host name, or text that may contain one, so that email
// programs won't turn it into a link: dots between letters become [.] and a
// leading http becomes hxxp.
func Defang(s string) string {
	s = strings.NewReplacer("http://", "hxxp://", "https://", "hxxps://", "HTTP://", "hxxp://", "HTTPS://", "hxxps://").Replace(s)
	// Applied twice because the pattern consumes the letter after each dot.
	return dotBetweenLetters.ReplaceAllString(dotBetweenLetters.ReplaceAllString(s, "$1[.]$2"), "$1[.]$2")
}

// leadLines are the plain facts of each shown lead, shared by both renderers.
type leadLine struct {
	text string // the title, defanged unless the lead is known
	site string // "example[.]com (unknown site)" or the full link for a known site
	link string // only for a known site and only if it is a safe http(s) link
}

func leadLines(m Message) (lines []leadLine, more int) {
	for i, l := range m.Leads {
		if i >= MaxLeadsShown {
			more = len(m.Leads) - MaxLeadsShown
			break
		}
		if l.Known {
			if u := safeURL(l.URL); u != "" {
				lines = append(lines, leadLine{text: l.Title, site: u + " (not scored)", link: u})
				continue
			}
		}
		lines = append(lines, leadLine{text: Defang(l.Title), site: Defang(l.Host) + " (unknown site)"})
	}
	return lines, more
}

func newSitesText(m Message) string {
	if len(m.NewSites) == 0 {
		return ""
	}
	var d []string
	for _, h := range m.NewSites {
		d = append(d, Defang(h))
	}
	return "New sites seen: " + strings.Join(d, ", ") + ". If you want a site covered, sign up for its email job alerts; job-radar will read those in a later version."
}

func leadsText(m Message) string {
	lines, more := leadLines(m)
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n== Leads from other sites (not scored) ==\n\n" + leadsWarning + "\n\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "  • %s — %s\n", l.text, l.site)
	}
	if more > 0 {
		fmt.Fprintf(&b, "  … and %d more\n", more)
	}
	if s := newSitesText(m); s != "" {
		b.WriteString("\n" + s + "\n")
	}
	return b.String()
}

func leadsHTML(m Message) string {
	lines, more := leadLines(m)
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<h2 style="font-size:16px;margin:24px 0 8px;border-bottom:1px solid #ddd;padding-bottom:4px">Leads from other sites (not scored)</h2>`)
	fmt.Fprintf(&b, `<p style="background:#fff6e0;border-radius:4px;padding:8px 10px;font-size:13px"><strong>⚠️ %s</strong></p>`, esc(leadsWarning))
	b.WriteString(`<ul style="padding-left:18px">`)
	for _, l := range lines {
		if l.link != "" {
			fmt.Fprintf(&b, `<li>%s — <a href="%s" style="color:#0b5fff">%s</a></li>`, esc(l.text), esc(l.link), esc(l.site))
		} else {
			fmt.Fprintf(&b, `<li>%s — %s</li>`, esc(l.text), esc(l.site))
		}
	}
	if more > 0 {
		fmt.Fprintf(&b, `<li>… and %d more</li>`, more)
	}
	b.WriteString(`</ul>`)
	if s := newSitesText(m); s != "" {
		fmt.Fprintf(&b, `<p style="color:#555;font-size:13px">%s</p>`, esc(s))
	}
	return b.String()
}

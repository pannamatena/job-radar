package notify

import (
	"fmt"
	"html"
	"net/url"
	"strings"
)

// replyHint tells the reader how to vote. Votes are read back on the next run.
const replyHint = `Reply to this email with e.g. "1 👍, 3 👎" (or "1 up 3 down"). 👍 = good call, 👎 = bad call (scored too high). Under "Just below the line", 👍 means it should have scored higher.`

// RenderText is the plain-text version of a message.
func RenderText(m Message) string {
	var b strings.Builder
	if m.Intro != "" {
		b.WriteString(m.Intro + "\n")
	}
	n := 0
	for _, sec := range m.Sections {
		if len(sec.Alerts) == 0 {
			continue
		}
		if sec.Title != "" {
			fmt.Fprintf(&b, "\n== %s ==\n", sec.Title)
		}
		for _, a := range sec.Alerts {
			n++
			fmt.Fprintf(&b, "\n%d. [%d%s] %s — %s\n", n, a.Score, aiTag(a), a.Company, a.Title)
			fmt.Fprintf(&b, "   %s\n", details(a))
			if a.Why != "" {
				fmt.Fprintf(&b, "   Why: %s\n", a.Why)
			}
			if a.Gap != "" {
				fmt.Fprintf(&b, "   Gap: %s\n", a.Gap)
			}
			if u := safeURL(a.URL); u != "" {
				fmt.Fprintf(&b, "   %s\n", u)
			}
		}
	}
	if n > 0 {
		b.WriteString("\n" + replyHint + "\n")
	}
	return b.String()
}

// RenderHTML is the HTML version. Everything that came from a job ad or from the
// scorer is escaped, and only http(s) links are made clickable.
func RenderHTML(m Message) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:-apple-system,Segoe UI,Helvetica,Arial,sans-serif;max-width:640px;color:#1a1a1a;line-height:1.45">`)
	if m.Intro != "" {
		fmt.Fprintf(&b, `<p>%s</p>`, esc(m.Intro))
	}
	n := 0
	for _, sec := range m.Sections {
		if len(sec.Alerts) == 0 {
			continue
		}
		if sec.Title != "" {
			fmt.Fprintf(&b, `<h2 style="font-size:16px;margin:24px 0 8px;border-bottom:1px solid #ddd;padding-bottom:4px">%s</h2>`, esc(sec.Title))
		}
		for _, a := range sec.Alerts {
			n++
			title := esc(a.Company) + " — " + esc(a.Title)
			if u := safeURL(a.URL); u != "" {
				title = fmt.Sprintf(`<a href="%s" style="color:#0b5fff;text-decoration:none">%s</a>`, esc(u), title)
			}
			fmt.Fprintf(&b, `<div style="margin:14px 0"><div><strong>%d.</strong> <span style="background:#eef3ff;border-radius:4px;padding:1px 6px">%d%s</span> %s</div>`,
				n, a.Score, esc(aiTag(a)), title)
			fmt.Fprintf(&b, `<div style="color:#555;font-size:13px">%s</div>`, esc(details(a)))
			if a.Why != "" {
				fmt.Fprintf(&b, `<div><strong>Why:</strong> %s</div>`, esc(a.Why))
			}
			if a.Gap != "" {
				fmt.Fprintf(&b, `<div><strong>Gap:</strong> %s</div>`, esc(a.Gap))
			}
			b.WriteString(`</div>`)
		}
	}
	if n > 0 {
		fmt.Fprintf(&b, `<p style="color:#555;font-size:13px;margin-top:24px">%s</p>`, esc(replyHint))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func esc(s string) string { return html.EscapeString(s) }

func aiTag(a Alert) string {
	if a.Scorer == "llm" {
		return " AI"
	}
	return ""
}

func details(a Alert) string {
	parts := []string{orNone(a.Track), orNone(a.WorkModel)}
	if a.Location != "" {
		parts = append(parts, a.Location)
	}
	if a.New {
		parts = append(parts, "new")
	}
	return strings.Join(parts, " · ")
}

// safeURL returns the URL only if it is an http or https link, so a hostile ad
// can't plant a javascript: or data: link in the email.
func safeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

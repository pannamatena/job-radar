package posting

import (
	"bufio"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTMLToText turns an HTML job description into readable plain text. Job
// boards return descriptions as HTML; the scorer and the rules engine want
// plain text, and so do humans reading `score-one`. We keep it deliberately
// simple: strip tags, drop <script>/<style> content, turn block-level elements
// and <br> into line breaks, and collapse runs of blank lines. This avoids a
// heavier Markdown/readability dependency (see BUILD_PLAN.md §2 "HTML to text").
func HTMLToText(htmlStr string) string {
	node, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		// html.Parse almost never errors (it is very lenient); if it does, fall
		// back to returning the input with tags naively removed.
		return collapseBlankLines(strings.TrimSpace(htmlStr))
	}
	var b strings.Builder
	walk(node, &b)
	return collapseBlankLines(strings.TrimSpace(b.String()))
}

// walk writes the text content of a node tree, inserting newlines around
// block-level elements and for <br>.
func walk(n *html.Node, b *strings.Builder) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(n.Data)
		return
	case html.ElementNode:
		switch n.DataAtom {
		case atom.Script, atom.Style, atom.Head, atom.Noscript:
			return // ignore non-content elements entirely
		case atom.Br:
			b.WriteByte('\n')
			return
		}
	}

	block := n.Type == html.ElementNode && isBlock(n.DataAtom)
	if block {
		b.WriteByte('\n')
	}
	// List items read better with a bullet.
	if n.Type == html.ElementNode && n.DataAtom == atom.Li {
		b.WriteString("- ")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, b)
	}
	if block {
		b.WriteByte('\n')
	}
}

func isBlock(a atom.Atom) bool {
	switch a {
	case atom.P, atom.Div, atom.Li, atom.Ul, atom.Ol, atom.Tr, atom.Table,
		atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
		atom.Section, atom.Article, atom.Header, atom.Footer, atom.Blockquote,
		atom.Pre:
		return true
	}
	return false
}

// collapseBlankLines trims trailing spaces on each line and collapses runs of
// blank lines down to a single blank line, so the output is tidy.
func collapseBlankLines(s string) string {
	var out []string
	blankRun := 0
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t")
		if strings.TrimSpace(line) == "" {
			blankRun++
			if blankRun > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blankRun = 0
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

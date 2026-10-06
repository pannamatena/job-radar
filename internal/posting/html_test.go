package posting

import (
	"strings"
	"testing"
)

func TestHTMLToText(t *testing.T) {
	in := `<div><h2>About the role</h2><p>We want a <strong>Senior Frontend Engineer</strong>.</p>` +
		`<script>var x = 1;</script><ul><li>React</li><li>TypeScript</li></ul>` +
		`First line<br>Second line</div>`
	got := HTMLToText(in)

	for _, want := range []string{
		"About the role",
		"Senior Frontend Engineer",
		"- React",
		"- TypeScript",
		"First line\nSecond line",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "var x = 1") {
		t.Errorf("script contents leaked into text:\n%s", got)
	}
}

func TestHTMLToText_CollapsesBlankLines(t *testing.T) {
	got := HTMLToText("<p>a</p><p></p><p></p><p>b</p>")
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("expected blank lines collapsed, got:\n%q", got)
	}
}

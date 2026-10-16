// Package feedback reads the user's votes out of an email reply such as
// "1 👍, 3 👎" or "1 up 3 down". Reply text is untrusted: anything that isn't a
// number followed by a recognised vote word is ignored, never acted on.
package feedback

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Vote is one "number + thumb" found in a reply.
type Vote struct {
	Number int
	Up     bool
}

var (
	// "2 👍", "2: down", "2 +1", "2-" are not accepted ("-" alone is too vague);
	// the vote word must follow the number.
	numThenVote = regexp.MustCompile(`(?i)(?:^|[^0-9.])(\d{1,3})[ \t]*[:=\-]?[ \t]*(👍|👎|\+1|-1|up|down|yes|no)`)
	// "👍 2" (thumb first) is also common.
	voteThenNum = regexp.MustCompile(`(?i)(👍|👎)[ \t]*(\d{1,3})`)
	// Where the quoted original starts.
	quoteStart = regexp.MustCompile(`(?i)^\s*(>|on .{5,100} wrote:?\s*$|-{2,}\s*original message|from:\s)`)
	// Skin-tone modifiers and variation selectors after an emoji.
	emojiMods = strings.NewReplacer("\U0001F3FB", "", "\U0001F3FC", "", "\U0001F3FD", "", "\U0001F3FE", "", "\U0001F3FF", "", "️", "")
)

// OwnText returns only what the person wrote, dropping the quoted email they
// replied to and anything after it.
func OwnText(body string) string {
	var keep []string
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if quoteStart.MatchString(line) {
			break
		}
		keep = append(keep, line)
	}
	return emojiMods.Replace(strings.Join(keep, "\n"))
}

// Parse finds the votes in a reply body. If the same number is voted on twice,
// the later vote wins. Numbers above max (or below 1) are ignored.
func Parse(body string, max int) []Vote {
	text := OwnText(body)
	type found struct {
		pos int
		v   Vote
	}
	var all []found
	// A word vote must end at a word boundary ("up" in "update" is not a vote), and a
	// number after a thumb must not continue into more digits.
	endsCleanly := func(end int, word string) bool {
		if end >= len(text) {
			return true
		}
		r, _ := utf8.DecodeRuneInString(text[end:])
		if word != "" && (word[0] >= 'a' && word[0] <= 'z' || word[0] >= 'A' && word[0] <= 'Z') {
			return !unicode.IsLetter(r)
		}
		return true
	}
	add := func(pos int, numStr, word string) {
		n, err := strconv.Atoi(numStr)
		if err != nil || n < 1 || n > max {
			return
		}
		switch strings.ToLower(word) {
		case "👍", "+1", "up", "yes":
			all = append(all, found{pos, Vote{n, true}})
		case "👎", "-1", "down", "no":
			all = append(all, found{pos, Vote{n, false}})
		}
	}
	for _, m := range numThenVote.FindAllStringSubmatchIndex(text, -1) {
		if endsCleanly(m[1], text[m[4]:m[5]]) {
			add(m[2], text[m[2]:m[3]], text[m[4]:m[5]])
		}
	}
	for _, m := range voteThenNum.FindAllStringSubmatchIndex(text, -1) {
		if m[5] >= len(text) || !unicode.IsDigit(rune(text[m[5]])) {
			add(m[2], text[m[4]:m[5]], text[m[2]:m[3]])
		}
	}
	// Order by position in the text, so "later wins" means later in the reply.
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].pos < all[j-1].pos; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	last := map[int]int{} // number → index in out
	var out []Vote
	for _, f := range all {
		if i, ok := last[f.v.Number]; ok {
			out[i] = f.v
			continue
		}
		last[f.v.Number] = len(out)
		out = append(out, f.v)
	}
	return out
}

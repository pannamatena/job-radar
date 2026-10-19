package search

import (
	"context"
	"fmt"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
)

// ResolveStats says what happened to every posting the search found, so
// nothing disappears without a reason.
type ResolveStats struct {
	Found      int // postings the search found (after removing repeats)
	Read       int // postings read from their board
	OnWatch    int // skipped: the board is on your watch-list, so the normal run reads it
	Ignored    int // skipped: the company is on your ignore list
	Closed     int // the board no longer lists the posting
	BoardsOK   int
	BoardsFail int // boards that couldn't be read (their postings are not counted above)
	FailedJobs int // postings on those boards
}

// FetchBoard reads one company's whole board through its official API. In
// production it is built from sources.For and the shared, polite HTTP client.
type FetchBoard func(ctx context.Context, company config.Company) ([]posting.Posting, error)

// Resolve reads the postings the refs name, through their boards' APIs (never
// by opening a link). It reads each board once and keeps only the postings that
// were found. A posting the board no longer lists is closed and is dropped.
// Boards already on the watch-list are skipped, because the normal run reads
// them. Companies on the ignore list are skipped.
func Resolve(ctx context.Context, fetch FetchBoard, refs []Ref, watchList []config.Company, ignore []string) (postings []posting.Posting, stats ResolveStats, problems []string) {
	stats.Found = len(refs)
	type board struct{ ats, slug string }
	onList := map[board]bool{}
	for _, c := range watchList {
		onList[board{c.ATS, NormalName(c.Slug)}] = true
	}
	ignored := map[string]bool{}
	for _, n := range ignore {
		ignored[NormalName(n)] = true
	}

	order := []board{}
	byBoard := map[board]map[string]bool{}
	slugOf := map[board]string{}
	for _, r := range refs {
		b := board{r.ATS, NormalName(r.Slug)}
		if onList[b] {
			stats.OnWatch++
			continue
		}
		if ignored[NormalName(r.Slug)] {
			stats.Ignored++
			continue
		}
		if byBoard[b] == nil {
			byBoard[b] = map[string]bool{}
			order = append(order, b)
			slugOf[b] = r.Slug
		}
		byBoard[b][r.PostingID()] = true
	}

	for _, b := range order {
		if ctx.Err() != nil {
			problems = append(problems, "stopped before reading every board found by search")
			break
		}
		slug := slugOf[b]
		ps, err := fetch(ctx, config.Company{Name: slug, ATS: b.ats, Slug: slug})
		if err != nil {
			problems = append(problems, fmt.Sprintf("couldn't read %s board %q found by search: %v", b.ats, slug, err))
			stats.BoardsFail++
			stats.FailedJobs += len(byBoard[b])
			continue
		}
		stats.BoardsOK++
		got := 0
		for _, p := range ps {
			if byBoard[b][p.ID] {
				postings = append(postings, p)
				got++
			}
		}
		stats.Read += got
		stats.Closed += len(byBoard[b]) - got
	}
	return postings, stats, problems
}

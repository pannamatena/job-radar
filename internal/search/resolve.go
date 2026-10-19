package search

import (
	"context"
	"errors"
	"fmt"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/posting"
	"github.com/pannamatena/job-radar/internal/sources"
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

// FetchOne reads one posting by its board id through the board's official API.
// found is false, with no error, when the board says the posting is gone. It
// returns sources.ErrNoSingleRead for a board that can only be read as a whole.
type FetchOne func(ctx context.Context, company config.Company, jobID string) (p posting.Posting, found bool, err error)

// WholeBoardAt is how many found postings at one board make reading the board
// once cheaper than reading each posting (ADR-0038).
const WholeBoardAt = 3

// Resolve reads the postings the refs name, through their boards' APIs (never
// by opening a link). Where the board can answer for a single posting, and
// fewer than WholeBoardAt were found there, each is read on its own and a 404
// means it is closed. Otherwise the board is read once and a posting missing
// from it is closed. Boards already on the watch-list are skipped, because the
// normal run reads them. Companies on the ignore list are skipped.
func Resolve(ctx context.Context, fetch FetchBoard, one FetchOne, refs []Ref, watchList []config.Company, ignore []string) (postings []posting.Posting, stats ResolveStats, problems []string) {
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
	jobsOf := map[board][]string{} // job ids, in the order found
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
		if !byBoard[b][r.PostingID()] {
			byBoard[b][r.PostingID()] = true
			jobsOf[b] = append(jobsOf[b], r.JobID)
		}
	}

	for _, b := range order {
		if ctx.Err() != nil {
			problems = append(problems, "stopped before reading every board found by search")
			break
		}
		slug := slugOf[b]
		co := config.Company{Name: slug, ATS: b.ats, Slug: slug}
		jobs := jobsOf[b]

		if one != nil && len(jobs) < WholeBoardAt {
			got, closed, failed, whole := readEach(ctx, one, co, jobs, &problems)
			if !whole {
				postings = append(postings, got...)
				stats.Read += len(got)
				stats.Closed += closed
				stats.FailedJobs += failed
				if failed > 0 {
					stats.BoardsFail++
				} else {
					stats.BoardsOK++
				}
				continue
			}
			// This board can't answer for one posting, so read it whole below.
		}

		ps, err := fetch(ctx, co)
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

// readEach reads postings one at a time. whole is true, and nothing else is
// meaningful, when the board has no single-posting read.
func readEach(ctx context.Context, one FetchOne, co config.Company, jobs []string, problems *[]string) (got []posting.Posting, closed, failed int, whole bool) {
	for _, id := range jobs {
		p, found, err := one(ctx, co, id)
		switch {
		case errors.Is(err, sources.ErrNoSingleRead):
			return nil, 0, 0, true
		case err != nil:
			*problems = append(*problems, fmt.Sprintf("couldn't read %s posting %s/%s found by search: %v", co.ATS, co.Slug, id, err))
			failed++
		case !found:
			closed++
		default:
			got = append(got, p)
		}
	}
	return got, closed, failed, false
}

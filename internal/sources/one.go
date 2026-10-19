package sources

import (
	"context"
	"errors"

	"github.com/pannamatena/job-radar/internal/config"
	"github.com/pannamatena/job-radar/internal/httpclient"
	"github.com/pannamatena/job-radar/internal/posting"
)

// ErrNoSingleRead means the board has no way to read one posting, so the caller
// should read the whole board instead (ADR-0038). Ashby is the one today.
var ErrNoSingleRead = errors.New("this board can only be read as a whole")

// OneReader is a Source that can also read a single posting by its board id.
// found is false, with no error, when the board says the posting doesn't exist
// (it has been closed).
type OneReader interface {
	FetchOne(ctx context.Context, jobID string) (p posting.Posting, found bool, err error)
}

// ReadOne reads one posting from the company's board, or returns
// ErrNoSingleRead when that board has no single-posting endpoint. Company
// filters are not applied: the caller asked for this posting by name.
func ReadOne(ctx context.Context, client *httpclient.Client, company config.Company, jobID string) (posting.Posting, bool, error) {
	src, err := For(client, company)
	if err != nil {
		return posting.Posting{}, false, err
	}
	one, ok := src.(OneReader)
	if !ok {
		return posting.Posting{}, false, ErrNoSingleRead
	}
	return one.FetchOne(ctx, jobID)
}

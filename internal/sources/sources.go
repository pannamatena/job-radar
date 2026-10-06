// Package sources fetches job postings from the places a user watches: public
// job-board APIs (Greenhouse, and later Lever and Ashby) and, in a later phase,
// job-alert emails over IMAP.
//
// Every source implements the small Source interface and performs all of its
// network I/O through the shared httpclient, so the project's "be a polite
// client" rules apply uniformly (see internal/httpclient and ADR-0022).
package sources

import (
	"context"

	"github.com/pannamatena/job-radar/internal/posting"
)

// Source fetches the currently open postings it is responsible for and returns
// them normalised. Implementations should respect ctx for cancellation and
// must not create their own HTTP client.
type Source interface {
	// Name identifies the source for logging, e.g. "greenhouse:anthropic".
	Name() string
	// Fetch returns the open postings. Company-level filters from config are
	// applied here so the rest of the pipeline sees only relevant roles.
	Fetch(ctx context.Context) ([]posting.Posting, error)
}

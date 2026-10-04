// Package posting defines the normalised shape every job source produces,
// regardless of which job board (or, later, which alert email) it came from.
// Keeping one Posting type means the rest of the pipeline — dedupe, pre-filter,
// scoring, storage, notifications — never has to care where a role came from.
package posting

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// RemoteKind describes whether a role can be done remotely. It is a small
// closed set so the pre-filter and scorer can reason about it consistently.
type RemoteKind string

const (
	RemoteYes     RemoteKind = "yes"
	RemoteNo      RemoteKind = "no"
	RemoteHybrid  RemoteKind = "hybrid"
	RemoteUnknown RemoteKind = "unknown"
)

// Posting is a single job ad, normalised from any source.
type Posting struct {
	// ID is a stable identifier of the form "<source>:<externalID>", e.g.
	// "greenhouse:anthropic/4012345". It must stay the same across runs so
	// dedupe works, which is why it is built from the source and the board's
	// own id rather than anything volatile like the title.
	ID string

	Source          string     // e.g. "greenhouse", "lever", "ashby", "imap"
	Company         string     // display name from the user's config
	Title           string     // role title
	Location        string     // free-text location as the board states it
	Remote          RemoteKind // derived from structured fields or text
	URL             string     // public link to apply / read the full ad
	DescriptionText string     // plain text (HTML stripped)
	PostedAt        *time.Time // when the board says it was posted; nil if unknown
	FirstSeenAt     time.Time  // when this tool first saw it
	RawJSON         []byte     // the source's original record, for debugging
}

// MakeID builds a stable Posting ID from a source name and the source's own
// external identifier. Both are required; external IDs are coerced to a short
// stable form so an empty or very long value can't produce a bad key.
func MakeID(source, externalID string) string {
	if externalID == "" {
		// Should not happen for real boards, but never produce a bare "source:".
		externalID = "unknown"
	}
	return source + ":" + externalID
}

// FingerprintID is a fallback stable ID for sources that have no reliable
// external id (e.g. some alert emails): a hash of the identifying fields.
func FingerprintID(source string, parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return source + ":" + hex.EncodeToString(h.Sum(nil))[:16]
}

// Package jobradar is the module root. It exists mainly to embed the static
// assets that ship inside the binary — the example persona and the
// security-and-privacy notice — so `job-radar init` works from a single
// downloaded executable with no other files alongside it.
//
// The embed directives must live here at the repo root because go:embed paths
// cannot climb above the directory of the source file.
package jobradar

import "embed"

// ExamplesFS holds the fictional-persona example config, profile, rubric and
// eval cases. `job-radar init` copies these into the user's config directory.
//
//go:embed examples
var ExamplesFS embed.FS

// SecurityNotice is the full text of docs/security-and-privacy.md, shown
// (in summary) and linked by `job-radar init`.
//
//go:embed docs/security-and-privacy.md
var SecurityNotice string

// NoticeVersion identifies the version of the security-and-privacy notice the
// user confirms during init. Keep this in step with the "Last updated" date at
// the bottom of docs/security-and-privacy.md: bump both together when the
// notice changes, so returning users are asked to re-read it.
const NoticeVersion = "2026-10-04"

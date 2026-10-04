# job-radar: Architecture Decision Records

Each record explains one decision: the problem, the options I considered, what I chose, and what it costs me. They're written to be public (no personal data), so they go in the repo under `docs/decisions/`, one file per record (`0001-no-scraping.md`, etc.).

**Format for every record:** Status · Context · Options considered · Decision · Trade-offs (what this costs us) · Revisit if · In one sentence (how I'd explain it in an interview).

Statuses: **Accepted** (decided), **Proposed** (decided in principle, to be confirmed by data), **Superseded** (replaced by a later record; keep it, and link to the replacement).

---

---

## How to add a new record (for Claude Code and contributors)

- **Whenever a choice between real alternatives is made** (a library, a data format, a service, an architecture change, a cost or security trade-off), write a new ADR **in the same commit**, using the next number.
- Don't edit an accepted ADR's decision. If it changes, write a new ADR that supersedes it, and mark the old one **Superseded by ADR-00XX**.
- Keep them public-safe: no personal data, no secrets, no private config values.
- Keep each one short: if the "In one sentence" line is hard to write, the decision probably isn't clear yet.

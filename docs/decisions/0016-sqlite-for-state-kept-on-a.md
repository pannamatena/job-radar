## ADR-0016: SQLite for state, kept on a `state` branch

**Status:** Proposed (the persistence method in CI is to be confirmed when building)

**Context:** The tool must remember which postings it has seen, their scores, alerts sent and feedback, between scheduled runs.

**Options considered:**
1. Flat JSON files: simple, but awkward to query and to change safely.
2. A hosted database (e.g. Postgres): powerful, but it costs money and is another service to secure.
3. **SQLite**, using the pure-Go `modernc.org/sqlite` driver (no C compiler needed, so it cross-compiles and runs in CI easily).

For keeping the file between CI runs:
- (a) `actions/cache`, which GitHub can evict, losing history;
- (b) commit the database to a dedicated `state` branch in the private repo.

**Decision:** SQLite, and (b) the `state` branch, unless building shows a problem.

**Trade-offs:** The repo grows over time with database commits (it may need occasional squashing). Only one run can write at a time, which is fine for a scheduled job. The database holds personal data, which is one reason the config repo must be private.

**In one sentence:** "A single-file database was all this needed, and I chose storage that can't be silently evicted."

---

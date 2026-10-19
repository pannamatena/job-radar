## ADR-0038: Read one posting when the job board offers it

**Status:** Accepted (the decision is made; it will be built in a separate, small change after the search phase)

**Context:** When a web search finds a job on a known job board ([ADR-0037](0037-find-jobs-by-search-read-them-through-board-apis.md)), job-radar reads it through the board's official API. Today it does that by downloading the company's **whole board** (every open job) and picking out the one found. That has three costs: it moves more data than needed, it fails outright on very large boards (an aggregator's Lever board is over the 10 MB response limit of [ADR-0022](0022-hard-limits-on-outbound-requests-enforced.md)), and "this job is closed" is only inferred from the job being missing from a list. In a test, about half of the links a search returned pointed at jobs that were already closed.

I tested the three boards' APIs on 2026-10-19:

| Board | Reading one job | Closed or unknown job |
|---|---|---|
| Greenhouse | `GET /v1/boards/{company}/jobs/{id}` returns the job (about 10 KB) | 404 |
| Lever | `GET /v0/postings/{company}/{id}` returns the job (200) | 404 |
| Ashby | no single-job endpoint (401); only the board list | not listed |

**Options considered:**
1. **Keep reading whole boards.** Simple, one code path, one request per company however many of its jobs were found. But large boards fail and closed jobs are only inferred.
2. **Read one job where the board offers it (chosen).** A direct answer for "is it open" and a small response. Ashby has no such endpoint and keeps using the board list.
3. **Always read one job, for every found job.** Tidy, but it can make more requests than a board read when several found jobs sit on one board, and Ashby cannot do it anyway.

**Decision:** Option 2, with one rule to keep the request count down.
- For Greenhouse and Lever, read each found posting with the single-job endpoint. A 404 means the job is closed and it is dropped. Any other error is reported as a failure to read that board, not as a closed job.
- If several found jobs belong to the same board (three or more), read that board's list once instead, because one larger request is fewer requests than many small ones.
- Ashby always reads the board list, as today.
- Both ways must produce the same posting, with the same id, so a job found by search and by the watch-list is one job. A test compares them.
- Every request still goes through the shared HTTP client and its per-host rate limit and request cap ([ADR-0022](0022-hard-limits-on-outbound-requests-enforced.md)).

**Trade-offs:**
- There are two ways to read a posting, and they must be kept in step. The test that compares them is how.
- A company with many found jobs can still cost a whole-board read, and that board may be too large for the size limit. The rule only helps when few jobs are found there.
- It relies on two endpoints that are public but documented less prominently than the board lists. If one changes, the 404 would look like "closed". The fix is to treat only a 404 on a job id that was seen on a live board as closed, and to keep the board read as a fallback.

**Why:** the question job-radar is asking is "is this one job still open, and what does it say?". Asking for exactly that is cheaper, answers it directly, and makes the very large boards readable.

**Revisit if:** a board changes or removes its single-job endpoint, or Ashby adds one.

**In one sentence:** "Ask the job board for the one job I found, not its whole catalogue, and treat 'not found' as 'closed'."

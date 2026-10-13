## ADR-0028: A hybrid role in a bare region is neutral, not "outside home"

**Status:** Accepted

**Context:** Some boards give only a country for a hybrid role ("Ireland", "Hybrid, Ireland"). With `hybrid: home_only`, the scorer flagged these `outside_home_area` and capped them at 2, although the office may well be in the user's home city. A real posting was wrongly capped this way. The AI scorer showed the same doubt ("office city unconfirmed").

**Options considered:**
1. Keep penalising: simple, but punishes a missing detail rather than a real mismatch.
2. Treat a bare region as neutral: no flag, no cap, no boost.
3. Let the AI look up the office on the web: could settle it, but is a bigger change with its own risks (see below).

**Decision:** Option 2. If a hybrid location is only a region the user can work in (a `remote_open_to` entry, with filler words like "hybrid" ignored), no home-area flag is raised and the score isn't capped. A named city outside the home area is still flagged. The AI rubric states the same rule, and the `lookup_office` tool plus the per-company `office` config field let the user settle individual companies.

**Trade-offs:** A few roles whose office really is outside the home area will be scored without penalty and will reach the user to check. The rule uses the user's own `remote_open_to` list, so it works for any country with no hard-coded places.

**Why:** the same reason as [ADR-0012](0012-pre-filter-rule-when-in-doubt.md): missing information shouldn't count against a role. A wrongly kept role costs one look, a wrongly capped one is never seen.

**Not decided:** letting the AI search the web for offices is deferred, not ruled out. It would need its own ADR covering untrusted search results, cost and repeatable evals.

**In one sentence:** "When an ad doesn't say which city, I treat it as unknown, not as wrong."

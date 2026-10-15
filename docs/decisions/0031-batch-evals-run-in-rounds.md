## ADR-0031: Batch evals run in rounds

**Status:** Accepted

**Context:** The provider's batch API costs about half as much but returns results later (minutes to a day). It takes a set of independent requests. The scoring agent isn't one request per ad: it makes several calls, each depending on the last (the model asks for the ad text, gets it, then records a score). A batch can't hold a conversation.

**Options considered:**
1. Don't support batch: simplest, but evals are the main recurring AI cost.
2. Make the agent single-shot for batch runs (put the ad in the first message, no tools): cheap and one round, but it would measure a different scorer than the one that runs for real.
3. Run the real agent in rounds: each round sends one request per unfinished scoring as one batch, waits, feeds the replies back, and repeats until every ad has a score.

**Decision:** Option 3. The agent loop was split into a step-by-step session (`Next` gives the next request, `Apply` takes the reply). The normal one-at-a-time `Score` is just a loop over it, and the batch eval runs many sessions together, one round per batch. Batch is for evals only, never live alerts.

**Trade-offs:** Slower (a wait per round, typically two or three) and latency can't be measured. Prompt caching is best-effort across rounds minutes apart. It adds a second way to drive the agent, which has to stay in step with the first (tests cover both, and one test checks that batch pricing is half the standard).

**Why:** an eval should measure the same agent that runs for real. Option 2 would have saved rounds but measured something else.

**In one sentence:** "I kept the real agent and adapted the batch around it, so the cheap eval still measures the thing I actually run."

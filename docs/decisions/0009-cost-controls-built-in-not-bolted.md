## ADR-0009: Cost controls built in, not bolted on

**Status:** Accepted

**Context:** Users pay for their own AI usage; a bug or a strange posting shouldn't be able to run up a bill.

**Decision:** Several layers:
- a cheap default model;
- the rules gate (ADR-0007);
- a 6-turn limit **and** a token budget per posting (fall back to the rules score if exceeded);
- a per-run limit and a monthly spend cap;
- each job scored once even if it arrives from several sources (cross-source duplicate matching);
- prices kept in config with a "last checked" date, so estimates don't go stale silently;
- the batch API (about 50% cheaper) for evaluation runs, which don't need instant results.

**Trade-offs:** More configuration and logic. Spend caps can mean some postings aren't AI-scored in a busy period (they keep their rules score, and the digest says so).

**In one sentence:** "Every way the tool could overspend has a cap, and every saving is measured rather than assumed."

---

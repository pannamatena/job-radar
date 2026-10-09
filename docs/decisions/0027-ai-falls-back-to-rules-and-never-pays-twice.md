## ADR-0027: When the AI can't score, the rules score stands; an unchanged ad is never paid for twice

**Status:** Accepted (phase 3)

**Context:** The plan said a posting the agent can't finish (turn limit, token budget) is "marked unscored". With `scorer: llm` on, two practical questions came up while building it:
1. What does the user see for a posting the AI didn't score — because it failed, or because a cap or the rules gate kept it away from the AI?
2. `run` re-scores every open posting on every run (scores aren't tied to "new to you", ADR-0025). With the AI on, that would mean paying again, twice a day, for ads that haven't changed.

**Options considered:**
- For (1): (a) leave it unscored — invisible in alerts and the digest until a later run; (b) keep the free rules score it already has, and record that the AI didn't score it and why.
- For (2): (a) only AI-score postings that are new to you; (b) reuse a stored AI score when the ad is unchanged; (c) re-score every run.

**Decision:**
- **(1b):** every posting always has a score. If the AI didn't score it, the rules score stands, and the stored row says why (`not_sent`, `skipped_cap` or `unscored`, with a note). Alerts show `(AI)` next to AI scores, so the user can always tell which scorer judged a role. The spend of a failed attempt still counts towards the monthly cap.
- **(2b):** an AI score is reused when the **ad content** (a hash of title, location, work model and text), the **model** and the **prompt version** (a hash of the full system prompt, so it includes the profile and rubric) are all unchanged. Changing any of them re-scores the posting on its next run.
- The monthly spend is the sum of the per-score cost estimates stored in the database, not a separate counter, so it survives restarts and can be audited.

**Trade-offs:**
- (1b) means a role the AI never saw can still reach an alert on its rules score. That's the safer error for a job search (a missed role is worse than an extra one, ADR-0012), and the `(AI)` label keeps it honest.
- (2b) costs a little storage and a cache lookup per posting. Postings first seen while a cap was hit are AI-scored on a later run if they're still open, instead of never.
- (2a) was rejected because an edited ad or an improved rubric should be re-judged, and because the eval (phase 3b) needs the prompt version on every score anyway.

**In one sentence:** "A posting always has a score and the user can see which scorer gave it, and the AI only gets paid again when something it would judge has actually changed."

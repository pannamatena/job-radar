## ADR-0010: Model choice decided by evaluation; escalation only if the data shows it's needed

**Status:** Proposed (the evaluation will confirm or change this)

**Context:** The cheapest capable model (Haiku-class) is likely fine for extraction and clear rules, but may be weaker on subtle judgement calls. The mid model (Sonnet-class) costs about twice as much.

**Options considered:**
1. Always use the cheap model.
2. Always use the bigger model.
3. Escalation: the cheap model scores everything, and only borderline results (a configurable band around the alert threshold, default 3–4) get a second opinion from the bigger model.

**Decision:** The cheap model is the default for all users. Compare both models on the same labelled cases, overall and on borderline cases only. **Build escalation (phase 3c) only if the evaluation shows the bigger model clearly wins on borderline cases** and using it for everything isn't worth the cost.

**Trade-offs:** Deciding later means a little uncertainty now. But it avoids building something that may not be needed.

**In one sentence:** "I didn't guess the model; I compared them on my own labelled data and chose on wrong-alert rate per euro."

---

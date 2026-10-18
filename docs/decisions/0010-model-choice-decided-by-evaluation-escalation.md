## ADR-0010: Model choice decided by evaluation; escalation only if the data shows it's needed

**Status:** Accepted

**Context:** The cheapest capable model (Haiku-class) is likely fine for extraction and clear rules, but may be weaker on subtle judgement calls. The mid model (Sonnet-class) costs more; the first evaluation measured it at about 13 times the cost per scored ad (see below).

**Options considered:**
1. Always use the cheap model.
2. Always use the bigger model.
3. Escalation: the cheap model scores everything, and only borderline results (a configurable band around the alert threshold, default 3–4) get a second opinion from the bigger model.

**Decision:** The cheap model is the default for all users. Compare both models on the same labelled cases, overall and on borderline cases only. **Build escalation (phase 3c) only if the evaluation shows the bigger model clearly wins on borderline cases** and using it for everything isn't worth the cost.

**Trade-offs:** Deciding later means a little uncertainty now. But it avoids building something that may not be needed.

**Evidence so far (first comparison):** on 14 hand-labelled cases, 3 repeats each, the larger model made no alert-level mistakes where the cheap model made four (3 missed good roles, 1 wrong alert, concentrated in two cases), and followed the output format better (1 invalid call against 9). It cost about $0.015 per ad against $0.0012. The set is small, curated for borderline cases and was tuned on the cheap model, so this is a weak signal. The default stays the cheap model; escalation (phase 3c) is **not** justified yet, and the cost of escalation can't be judged from a set where two thirds of the cases are borderline. Re-measure on real postings before deciding. Details: [eval-lessons.md](../eval-lessons.md).

**Evidence since (three models, new cases):** on the same prompt and labels, per scoring on 22 cases, the cheap model matched my label exactly 75% of the time, the Sonnet-class model 54% and the Opus-class model 41%; none missed a good role, and the cheap model made 4 wrong alerts against 2 and 3. On ten cases the prompt had never seen, exact matches were 7, 6 and 5. The bigger models cost about 9 to 27 times as much per scoring. My labels were written while watching the cheap model, which favours it, and the sets are small, so this is not proof that it is the better judge. It is enough to settle the default: **the cheap model is the default, a bigger model is an opt-in, and escalation (phase 3c) stays unbuilt** because no bigger model showed a gain worth the price. Details: [eval-lessons.md](../eval-lessons.md), section 12.

**In one sentence:** "I didn't guess the model; I compared them on my own labelled data and chose on wrong-alert rate per euro."

---

## ADR-0007: Rules first, AI second (the gate)

**Status:** Accepted (default threshold to be tuned from evaluation data)

**Context:** Most postings that pass the basic filter are still poor fits. Sending all of them to the AI is the biggest cost driver.

**Options considered:**
1. Send every posting that passes the pre-filter to the AI.
2. Run the rules scorer on everything, and only send postings scoring at or above a threshold (default 2) to the AI.

**Decision:** Option 2.

**Trade-offs:** A good role that the rules score badly would never reach the AI. The evaluation measures exactly this ("would the gate have blocked any role I labelled 4+?") and the threshold is tuned from that data.

**In one sentence:** "I put a free filter in front of the paid model, and used the eval to check it wasn't throwing away good roles."

---

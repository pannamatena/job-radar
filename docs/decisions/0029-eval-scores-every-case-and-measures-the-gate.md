## ADR-0029: The eval scores every case with the AI and measures the rules gate separately

**Status:** Accepted

**Context:** In real runs the rules gate (ADR-0007) hides ads with a low rules score from the AI to save money. An eval could mimic that and send only gated-in cases to the AI, or it could score every case.

**Options considered:**
1. Apply the gate in the eval, exactly like a real run: cheaper, but a good role hidden by the gate would silently vanish from the numbers.
2. Score every confirmed case with the AI and report the gate's effect on its own.

**Decision:** Option 2. The AI scores every confirmed case. Alongside, the rules score is recorded for each case, and the report says how many cases the gate would have hidden and whether any of them were roles the owner rated at or above the alert threshold.

**Trade-offs:** A few extra cheap AI calls per eval (cases the gate would have hidden). The monthly cost estimate is also reported both with and without the gate. Eval spend is checked against the monthly cap before a run but is not added to the spend `run` tracks, because eval cases are not stored postings.

**Why:** the question the gate raises is "does it lose good roles?", and that can only be answered by showing the AI the cases the gate would have hidden.

**In one sentence:** "I scored everything so I could measure what the cost-saving shortcut was losing."

## ADR-0030: Tune the scorer on hand-labelled cases, one change at a time, with repeats

**Status:** Accepted

**Context:** The AI scorer's behaviour depends on prose (rubric and profile) that I can edit freely. Without discipline, each edit risks fixing one ad and breaking another, and a single run of an LLM is noisy.

**Options considered:**
1. Edit the rubric until the ads I look at "feel right": fast, but I can't tell a real improvement from noise, or a regression from a fix.
2. Hand-label real ads and change one thing at a time, rerunning each case several times and comparing runs.
3. Generate synthetic test ads: cheap, but they don't contain the awkward wording real ads have.

**Decision:** Option 2. Labels are the human's alone. Every change (a rubric line, a profile line, a bug fix) is followed by a rerun with 3 repeats and a side-by-side comparison. When a rule's effect is unclear, I test by removing it. Cases I tuned against are treated as spent, and numbers measured on them are not published.

**Trade-offs:** Slower than editing by feel, and the set is small, so differences of one case are noise. It costs a few cents per run. The worst risk is overfitting to the cases I tune on, which is why published numbers wait for unseen cases. A relabel is not an improvement and must be tracked.

**Why:** the method found a rule that fixed one case and broke another, a profile line that spilled onto neighbouring skills, and bugs in my own harness. Each of those would have stayed hidden under option 1. The full account is in [eval-lessons.md](../eval-lessons.md).

**In one sentence:** "I changed one thing at a time and measured it, so I could tell a real fix from a lucky run."

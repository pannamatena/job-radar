## ADR-0011: Evaluate with a small hand-labelled set, measured at the alert threshold

**Status:** Accepted

**Context:** "It seems to work" isn't evidence. I need to know whether the scorer is right, and whether changes make it better or worse.

**Decision:**
- An eval set of real job ads, each labelled **by a human** (score, track, work model, flags, a one-line reason). Deliberately includes tricky cases: a rail-industry "Engineering Manager", a hybrid role outside the home area, ads that don't state the work model, a "lead" title with no leadership, and an ad with an injected instruction.
- The tool never writes or "fixes" labels; draft labels are excluded until a human confirms them.
- The key metric is at the **alert threshold**: wrong alerts (false positives) and missed good roles (false negatives). Also: within-±1 agreement, per-flag precision and recall, consistency across repeats, schema failures, cost per ad.
- The first comparison published is **rules vs AI**, then model vs model and prompt vs prompt.

**Trade-offs:** Labelling takes time, and a set of 20–30 cases is small, so the README states its size and limits honestly.

**In one sentence:** "I measured the thing that matters to the user, wrong alerts and missed roles, against my own labels, and I publish the sample size with the numbers."

---

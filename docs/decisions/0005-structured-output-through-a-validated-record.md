## ADR-0005: Structured output through a validated `record_score` tool

**Status:** Accepted

**Context:** Scores have to be stored, compared and evaluated, so free-text answers aren't good enough.

**Options considered:**
1. Ask for JSON in plain text and parse it.
2. A strict JSON schema on a `record_score` tool, validated in Go as well, with errors sent back so the model can correct itself.

**Decision:** Option 2. Every score also stores the model, a `prompt_version` (a hash of prompt + rubric), tokens and cost.

**Trade-offs:** A schema to maintain as fields change. In return: reliable data, measurable schema failures, and results that can be compared across prompt and model versions.

**In one sentence:** "Making the output a validated tool call turned model answers into data I could test and measure."

---

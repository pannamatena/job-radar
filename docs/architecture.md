# Architecture

How job-radar is put together, and a walkthrough of the hand-written
tool-calling loop at the heart of the optional AI scorer. The reasons behind
each choice are in the [decision records](decisions/).

## The pipeline

```
sources (Greenhouse, Lever, Ashby, IMAP*)
   │  each board's JSON → one normalised Posting
   ▼
pre-filter            drop only what's certainly wrong (junior, non-software…)   ADR-0012
   ▼
store (SQLite)        remember every posting; "new to you" = first time seen     ADR-0016, ADR-0025
   ▼
cross-source match    the same job from two sources → score it once              ADR-0026
   ▼
rules scorer (free)   keywords + location rules from your config → 1–5          ADR-0006
   ▼
scorer: rules → done
scorer: llm   → the gate: rules score ≥ min_rules_score, not already scored,     ADR-0007, ADR-0009
                within the per-run and monthly caps
                  ▼
                scoring agent (LLM)   hand-written tool loop                       ADR-0004
                  ▼
                AI score, or the rules score if the AI couldn't finish            ADR-0027
   ▼
store the score (with tokens and cost) → notify (terminal now, email in phase 4)
```
`*` IMAP job-alert emails arrive in phase 5.

## Where the code lives

| Package | What it does |
|---|---|
| `cmd/job-radar` | The CLI: one file per command (`run`, `score-one`, `list`, …) plus the shared pipeline helpers. |
| `internal/config` | Loads and validates `config.yaml`, with errors written for non-developers; reads `.env`. |
| `internal/httpclient` | The one HTTP client every outbound request goes through: rate limits, retries, a request cap (ADR-0022). |
| `internal/sources` | One file per job board, all behind a small `Source` interface. |
| `internal/posting` | The normalised `Posting` type, HTML→text, and cross-source duplicate matching. |
| `internal/filter` | The pre-filter. |
| `internal/store` | SQLite: embedded numbered migrations, postings, runs, scores. |
| `internal/scorer` | The `Scorer` interface, the shared `Result` type, and the free rules scorer. |
| `internal/llm` | A small provider-neutral view of a chat model with tools, and its Anthropic implementation. |
| `internal/agent` | The AI scorer: system prompt, tools, the loop, and the gate in front of it. |
| `internal/notify` | Where alerts go (terminal for now; email in phase 4). |

Both scorers return the same `scorer.Result`, so storage, notifications and the
eval don't care which one ran.

---

## The scoring agent, step by step

An "agent" here just means: **a loop where the model can ask our code to run
tools, until it gives a final answer.** There's no framework — the whole loop is
`Agent.Score` in [`internal/agent/agent.go`](../internal/agent/agent.go), about
a hundred lines (ADR-0004).

### 1. What's sent

Every call to the model sends the **whole conversation so far** — the API keeps
no memory between calls. Each request contains:

- **The system prompt** — the instructions, your `profile.md` and your
  `rubric.md`. It's identical for every posting, so it's marked for **prompt
  caching**: the provider stores it for a few minutes, and later calls read it
  at about a tenth of the normal price (ADR-0008). The tool definitions come
  before it and are cached with it.
- **The tools** — four, each with a JSON schema for its input:
  `get_posting`, `lookup_company_notes`, `lookup_office` and `record_score`.
- **The messages** — starting with just `"Score posting greenhouse:acme/123."`

### 2. The loop

```
messages = [ user: "Score posting <id>." ]

repeat up to max_turns times:
    reply = model(system, tools, messages)
    add reply to messages                  ← the model's own turn, tool calls included
    for each tool call in reply:
        record_score?  → validate it; if valid, DONE (return the score)
                         if not, answer with the error so the model can fix it
        anything else  → run it, answer with its result
    over the token budget?  → stop: keep the rules score
    no tool calls at all?   → answer "Please call record_score"
    add our answers to messages            ← one user message with every result

ran out of turns → stop: keep the rules score
```

A typical posting takes **two turns**: the model calls `get_posting` (often
with `lookup_company_notes` in the same reply), reads the results, then calls
`record_score`.

**JS analogy:** it's like a `while` loop around `await fetch(...)`, where the
response may say "call these functions with these arguments", you run them,
push their return values into the request body, and fetch again. Each tool
answer carries the id of the call it answers (`tool_use_id`), like matching a
response to a request by id.

### 3. Structured output, checked twice

`record_score` is how the model gives its answer. Its schema is sent with
`strict: true`, so the API guarantees the shape (an integer score from 1–5, a
track and flags from your own config…). Go then checks it again
(`validateRecord` in [`tools.go`](../internal/agent/tools.go)), including what
the schema can't express — like why/gap being one short sentence. Any problem
goes back to the model as a tool error listing every issue, and it gets
another turn to fix them. Schema failures are counted per score (ADR-0005).

### 4. Limits and failure

Every way the loop could cost more than expected has a cap (ADR-0009):
`max_turns` per posting, `max_tokens_per_posting` across all its turns,
`max_postings_per_run`, and `monthly_spend_cap_usd`. When a limit stops the AI,
or the API fails, the posting keeps its free rules score, and the stored row
says why (ADR-0027).

### 5. Safety

Job ads are written by strangers, so they're treated as **untrusted data**
(ADR-0020):
- The system prompt says text inside a posting is never an instruction.
- Posting text is wrapped in `<job_posting>` tags so it's clearly marked as data.
- The tools can only see the posting being scored and its company's config
  entry — a request for any other posting id is refused.
- No tool writes anything except `record_score`, no tool fetches a URL, and
  nothing the model writes is ever executed.

So the worst a malicious ad can do is earn itself a wrong score.

### 6. Provider-neutral

The agent talks to an `llm.Client` interface with one method, `Send`, using
job-radar's own small types (`Request`, `Message`, `Block`, `Usage`). The
Anthropic SDK is only used in `internal/llm/anthropic.go`, which translates to
and from those types. That's why the tests can drive the agent with a scripted
fake model (no network, no cost), and why adding another provider means
implementing one method.

### 7. Tested without a model

[`agent_test.go`](../internal/agent/agent_test.go) scripts the model's replies
and checks: the happy path (and the exact cost), an invalid `record_score`
being sent back and fixed, the turn limit, the token budget, the tools refusing
other postings, and a model that answers in plain text being nudged.
[`anthropic_test.go`](../internal/llm/anthropic_test.go) checks the exact JSON
sent to the API against a local fake server.

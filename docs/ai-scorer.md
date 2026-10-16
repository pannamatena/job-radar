# The optional AI scorer

The free [rules scorer](rules-scorer.md) only sees keywords. The AI scorer
reads the whole ad like a person would, checks it against your own profile and
rubric, and writes an honest one-line **why** and **gap**. It's optional, it
uses **your own API key**, and it **costs money** — a small amount per posting,
with caps so it can't run away.

> **What gets sent where:** with the AI scorer on, your `profile.md`, your
> `rubric.md`, your company notes and the job ad text are sent to the AI
> provider (Anthropic) for every posting it scores. Nothing else. Read
> [security-and-privacy.md](security-and-privacy.md) before switching it on.

---

## Setting it up

1. **Create an Anthropic API account** at the Anthropic Console. The API is
   billed separately from a Claude.ai subscription.
2. **Set a monthly spending limit in the Console.** This is your hard backstop;
   job-radar's own cap (below) is the second layer.
3. **Create an API key** and put it in a file called `.env` in the folder you
   run job-radar from (or in your config folder):
   ```sh
   ANTHROPIC_API_KEY=sk-ant-...
   ```
   `.env` is git-ignored, so it can't be committed by accident. Never paste the
   key into a chat, an issue or a screenshot. On macOS you can write the file
   straight from the clipboard without the key ever appearing on screen:
   ```sh
   printf 'ANTHROPIC_API_KEY=%s\n' "$(pbpaste)" > .env && chmod 600 .env
   ```
   **If you get** `This API key is not scoped to a workspace`, your key isn't
   tied to one Console workspace. Add the workspace's id (shown in the
   Console's workspace settings) to the same `.env` file:
   ```sh
   ANTHROPIC_WORKSPACE_ID=wrkspc_...
   ```
   Most keys don't need this. Creating the key inside a workspace avoids it.
4. **Write your `profile.md` and `rubric.md`** in your config folder. The AI
   can only judge fit against what's written there — see
   [writing-your-profile.md](writing-your-profile.md).
5. **Check the `llm:` block** in your `config.yaml` (the example config has a
   complete one), especially `monthly_spend_cap_usd` and the `pricing` table.
6. **Try it on one posting first:**
   ```sh
   job-radar score-one --scorer llm <posting-id-or-url>
   ```
   You'll see every step the agent took, the score, and what it cost.
   If job-radar prints `warning:` lines about your rubric, they mean the rubric
   and `config.yaml` disagree on a track or flag name. See
   [writing-your-profile.md](writing-your-profile.md#keep-the-names-the-same-as-in-configyaml).
7. When you're happy, set `scorer: llm` in `config.yaml` (or use
   `job-radar run --scorer llm` for a single run).

---

## How it works

```
every posting ──► free rules scorer ──► score below min_rules_score? ──► keep the rules score (free)
                                          │ no
                                          ▼
                       scored before (same ad, same model, same prompt)? ──► reuse it (free)
                                          │ no
                                          ▼
                       per-run limit or monthly cap reached? ──► keep the rules score
                                          │ no
                                          ▼
                                  the scoring agent (paid)
                                          │
                       finished within the limits? ── no ──► keep the rules score
                                          │ yes
                                          ▼
                                   the AI's score
```

The **scoring agent** is a small tool-calling loop, written by hand so every
step is visible ([ADR-0004](decisions/0004-a-hand-written-tool-calling-loop.md)):

1. job-radar sends the model a **system prompt** containing the instructions,
   your profile and your rubric, plus the message "Score posting <id>". The
   system prompt is identical for every posting, so it's
   **cached** — after the first posting, that part costs about a tenth of the
   normal price ([ADR-0008](decisions/0008-profile-and-rubric-in-a-cached.md)).
2. The model asks for what it needs through **tools**:

   | Tool | What it returns |
   |---|---|
   | `get_posting` | The ad: company, title, location, work model and text (very long ads are trimmed). |
   | `lookup_company_notes` | Your `notes` for that company in `config.yaml`, if any. |
   | `lookup_office` | Where the office is: your config's `office` for that company, else the board's location, else "not found". |
   | `record_score` | Records the final answer. The loop ends here. |

3. job-radar runs each tool and sends the results back; the model either asks
   for more or calls `record_score`.
4. `record_score` is checked strictly in Go: score 1–5, a track from your
   config, a work model, one-sentence why/gap, and only flags you allow (the
   built-in ones plus your `red_flags` names). If anything is wrong, the error
   goes back to the model so it can fix it
   ([ADR-0005](decisions/0005-structured-output-through-a-validated-record.md)).
5. If the model doesn't finish within `max_turns` model calls or the
   `max_tokens_per_posting` budget, or the API fails, the posting simply keeps
   its rules score. A posting is never left without a score
   ([ADR-0027](decisions/0027-ai-falls-back-to-rules-and-never-pays-twice.md)).

The tools can only see the posting being scored and its company's config entry.
They can't read other postings or files, and nothing the model writes is ever
run. Job ads are untrusted text: the system prompt tells the model to ignore any
instructions inside them ([ADR-0020](decisions/0020-untrusted-job-ads-can-t-make.md)).

---

## What it costs

You pay your AI provider per token (roughly ¾ of a word). job-radar keeps a
price table in your config so it can estimate every call.

With the default model (`claude-haiku-5-5`), a typical posting takes 2–3 model
calls. A rough estimate is **around a tenth of a US cent per posting** — so
20 postings a day is well under $1 a month. These are estimates, not
measurements: you can measure the real numbers on your own hand-labelled ads
with `job-radar eval` ([evaluation.md](evaluation.md)).

What keeps it cheap:
- **The rules gate** (`min_rules_score`): obvious misses never reach the AI.
- **No paying twice:** an AI score is stored with a fingerprint of the ad, the
  model and a version hash of your prompt. On the next run the same, unchanged
  ad reuses it for free. Edit your profile or rubric, or the company edits the
  ad, and it's scored again.
- **The same job from two sources is scored once**
  ([ADR-0026](decisions/0026-cross-source-duplicate-matching.md)).
- **Caps:** `max_postings_per_run`, `max_tokens_per_posting`, `max_turns` and
  `monthly_spend_cap_usd` (see [configuration.md](configuration.md#llm-fields)).
  When a cap stops AI scoring, those postings keep their rules score and the
  run summary says how many.

Every run prints what it sent to the AI and the estimated cost; every score is
stored with its token counts and cost.

> **Prices change.** Compare the `pricing` table in your config with the
> provider's pricing page now and then, and update `last_checked`. job-radar
> warns when it's more than 90 days old.

---

## Reading an AI score

```
$ job-radar score-one --scorer llm greenhouse:acme/4012345
Acme — Engineering Manager, Platform
https://boards.greenhouse.io/acme/jobs/4012345

Rules pre-score: 4/5 (the AI gate is 2)
Scoring with claude-haiku-5-5 (prompt 3f9c2a1b7e)…

Agent transcript:
  user: Score posting greenhouse:acme/4012345.
  model (turn 1, 41 in / 88 out / 3712 cache-read tokens, stop=tool_use)
    calls get_posting {"posting_id":"greenhouse:acme/4012345"}
    → Company: Acme Title: Engineering Manager, Platform Location (as listed): Manchester …
    calls lookup_company_notes {"posting_id":"greenhouse:acme/4012345"}
    → No notes for this company.
  model (turn 2, 2210 in / 141 out / 3712 cache-read tokens, stop=tool_use)
    calls record_score {"score":4,"track":"manager",…}
    → recorded score 4/5 (manager)

Score:      4/5  (AI, claude-haiku-5-5)
Track:      manager
Work model: hybrid
Office:     Manchester
Why:        Hands-on EM role leading a React/TypeScript team at a scale-up.
Gap:        Asks for production Go; the profile has none.
…
```

(The numbers above are illustrative.) The **prompt** hash changes whenever your
profile, rubric or the built-in instructions change, so scores from different
versions can be compared with `job-radar eval --compare` ([evaluation.md](evaluation.md)).

In `run` output, AI-scored matches are marked `(AI)` next to the score, so you
always know who judged a role.

---

## Its honest limits

- It's only as good as your `profile.md` and `rubric.md`. Vague in, vague out.
- It coaches by design: it assumes you can learn a quick-to-learn gap, and
  says how to close it, while your rubric's hard limits still score 1–2. It
  never invents experience. If you want it stricter, say so in your rubric
  ([ADR-0032](decisions/0032-the-scorer-coaches-by-default.md)).
- It can still be wrong. `job-radar eval` measures its accuracy on ads you've
  judged yourself, and compares it with the free rules scorer, so you can see
  what the paid option actually buys ([evaluation.md](evaluation.md)).
- `lookup_office` doesn't browse the company's website (yet); if neither your
  config nor the job board says where the office is, it reports "not found"
  rather than guessing.
- Only Anthropic is supported. The code keeps the provider behind a small
  interface (`internal/llm`) so others, including free local models, can be
  added.

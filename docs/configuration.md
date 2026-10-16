# Configuration reference

Your settings live in `config.yaml` inside your config folder (by default
`private/config.yaml`, which is never committed to git). This page documents
every field. It's kept honest by a test that fails if a config field is added
without being listed here.

> New to YAML? It's a simple text format. Indentation (spaces, never tabs) shows
> nesting, `- ` starts a list item, and `# ` starts a comment. See the
> [glossary](glossary.md).

A complete, working example is in [`examples/config.yaml`](../examples/config.yaml)
(the fictional persona). `job-radar init` copies it into your folder.

---

## Top-level fields

### `scorer`
How postings are scored. One of:
- `rules` — the free, keyword-based scorer (no API key, no cost). **The default.**
- `llm` — the optional AI scorer (needs your own API key; costs money).

Default: `rules`. Both `run` and `score-one` accept `--scorer rules|llm` to
override this for one command.

### `companies`
A list of the companies you want to watch. May be empty. Each entry is a
[company](#company-fields). Find a company's board with `job-radar discover`.

### `location`
Your [location rules](#location-fields), used by the pre-filter.

### `prefilter`
The [pre-filter rules](#pre-filter-fields) that drop obvious non-matches before
scoring. Optional — sensible software-role defaults are used if you omit it.

### `rules`
The [rules-scorer settings](#rules-fields): your tracks, keyword boosts, red
flags and thresholds. The rules scorer runs on every posting in both modes —
with `scorer: llm` it's the free first pass that decides what reaches the AI.

### `llm`
The [AI-scorer settings](#llm-fields): model, cost caps and prices. Only
checked when `scorer` is `llm`. See [ai-scorer.md](ai-scorer.md).

### `notice`
Records that you've read the security-and-privacy notice. Set by `job-radar init`;
you don't normally edit it. See [notice](#notice-fields).

---

## Company fields

Each item under `companies:`:

| Field | What it does | Example |
|---|---|---|
| `name` | A label for you (anything). Required. | `Acme` |
| `ats` | Which job board it uses: `greenhouse`, `lever`, `ashby`, or `none`. Required. | `greenhouse` |
| `slug` | The company's identifier on that board (its "board token"). Required for greenhouse/lever/ashby. | `acme` |
| `careers_url` | The careers-page link. Used when `ats` is `none`. | `"https://acme.com/careers"` |
| `notes` | Free text about the company. The AI scorer can read it (`lookup_company_notes`). Optional. | `"Great React team"` |
| `office` | Where the company's office is, if its job board is vague. The AI scorer's `lookup_office` tool checks this first. Optional. | `"Manchester city centre"` |
| `filters` | Narrow which roles are kept — see below. Optional. | |

### Company `filters`

Applied in Go before scoring. All are optional; within `filters`, a role is kept
if it matches **any** configured location condition (office/location/remote) and,
separately, matches a department if `departments` is set.

| Field | What it does | Notes |
|---|---|---|
| `departments` | Keep roles whose department/team contains one of these words. | Case-insensitive. Works for all three boards. |
| `office_ids` | Keep roles in these office IDs. | **Greenhouse only** (Greenhouse exposes numeric office IDs). |
| `location_names` | Keep roles whose location text contains one of these words. | e.g. `["Manchester"]` |
| `include_remote_open_to` | Also keep remote roles whose location mentions one of these regions. | e.g. `["United Kingdom"]` |

---

## Location fields

Under `location:`:

| Field | What it does | Example |
|---|---|---|
| `home_area` | The place you can work from day to day. | `"Manchester"` |
| `counts_as_home` | Location names that count as home (boards phrase places differently). | `["Manchester", "Greater Manchester"]` |
| `not_home_despite_sounding_close` | Places that sound nearby but you won't commute to; never treated as home. | `["Naas", "Maynooth"]` |
| `allowed_work_models` | Which work models are acceptable — see below. | |

### `allowed_work_models`

| Field | What it does | Values |
|---|---|---|
| `hybrid` | When a hybrid role is acceptable. `home_only` (default) means the office must be one of your `counts_as_home` cities, otherwise the role is flagged `outside_home_area` and capped at 2. A location that is only a region from `remote_open_to` (e.g. just "Ireland") names no city, so it's treated as neutral and kept. `any` accepts it wherever the office is. `never` means you don't take hybrid roles at all — flagged `work_model_not_accepted` and capped at 2. | `home_only`, `any` or `never` |
| `onsite` | When a fully on-site role is acceptable — same values as `hybrid`. Set `never` if you don't take on-site roles anywhere, not even in your home city. | `home_only`, `any` or `never` |
| `remote_open_to` | The regions/territories **you** can work remotely in. A remote role that names a region *not* in this list (e.g. a US-only or UK-only role) is flagged `outside_eligible_region` and capped at 2; a bare "Remote" with no region is kept. Leave empty to accept any remote role. | e.g. `["United Kingdom", "Europe", "EMEA"]` |
| `not_stated` | What to do when the work model isn't stated. | `keep_if_home_or_unknown` (never drop just for a missing model) |

> Location rules are applied by the scorer, not the pre-filter. The pre-filter
> is conservative and does not drop on location (when in doubt, keep — see [ADR-0012](decisions/0012-pre-filter-rule-when-in-doubt.md)).

---

## Pre-filter fields

Under `prefilter:`. The pre-filter keeps by default and only drops roles it's
sure about ([ADR-0012](decisions/0012-pre-filter-rule-when-in-doubt.md)); it
never considers a posting's age ([ADR-0025](decisions/0025-score-fit-independently-of-posting-age.md)).
All matching is case-insensitive and on whole words. Omit any list to use the
built-in software-role defaults.

| Field | What it does | Example |
|---|---|---|
| `title_keep` | Titles to keep even if a drop rule would catch them (an override). | `["engineering manager"]` |
| `title_drop` | Titles to drop (wrong level or kind). | `["junior", "director", "vp"]` |
| `not_software` | Keywords that mark a non-software-engineering role, matched in the title or description. | `["railway", "chartered engineer"]` |

Precedence: a `not_software` match drops (even over `title_keep`); otherwise a
`title_keep` match keeps; otherwise a `title_drop` match drops; otherwise the
posting is kept.

---

## Rules fields

Under `rules:` — these drive the free scorer (see [rules-scorer.md](rules-scorer.md)
for the full walkthrough). The score formula is:

```
1  + 2 if a track title matches
   + up to 2 for must_have_any matches (+1 each, capped)
   + up to 1 for nice_to_have matches
   − 1 per distinct red flag matched
clamped to 1–5. A posting's age never changes the score (ADR-0025).
```

| Field | What it does |
|---|---|
| `tracks` | A map of track name → `titles`. A posting whose title matches a track's `titles` gets the +2 track boost and is tagged with that track. |
| `titles` | (inside each track) The title phrases for that track, e.g. `["engineering manager", "head of engineering"]`. |
| `must_have_any` | Keywords that each add a small boost when found in the title or description (capped). |
| `nice_to_have` | Keywords that add a smaller boost (capped). |
| `red_flags` | A map of flag name → trigger phrases. Each distinct flag found lowers the score by one and is recorded on the posting. |
| `thresholds` | The score cut-offs — see below. |

### `thresholds`

| Field | What it does | Default |
|---|---|---|
| `alert` | Score at or above which a posting is a strong match (alerted). | `4` |
| `digest` | Score at or above which a posting appears in the daily digest. | `3` |

---

## LLM fields

Under `llm:` — the optional AI scorer (see [ai-scorer.md](ai-scorer.md)). Only
validated when `scorer` is `llm`, so rules-only users can ignore it. Every
setting that can cost money has a cap ([ADR-0009](decisions/0009-cost-controls-built-in-not-bolted.md)).

| Field | What it does | Default |
|---|---|---|
| `model` | The model to score with. Must have an entry under `pricing.models`. | — (required) |
| `min_rules_score` | The rules gate: only postings the free rules scorer scores at or above this go to the AI; the rest keep their rules score ([ADR-0007](decisions/0007-rules-first-ai-second-the-gate.md)). | `2` |
| `max_postings_per_run` | At most this many postings are sent to the AI in one run. The rest keep their rules score and are counted as skipped. | `25` |
| `max_tokens_per_posting` | Token budget for one posting across all turns (input + output, cached included). Past it, the posting keeps its rules score. | `40000` |
| `max_turns` | Model calls allowed per posting before giving up. | `6` |
| `monthly_spend_cap_usd` | Once the estimated AI spend this calendar month reaches this, AI scoring stops until next month. Set a limit with your AI provider too. | `5` |
| `pricing` | The price table used for every cost estimate and the cap — see below. | |

### `pricing`

Prices live in config, not code, because they change. All prices are **USD per
million tokens**.

| Field | What it does |
|---|---|
| `last_checked` | The date (`YYYY-MM-DD`) you last compared these with the provider's pricing page. job-radar warns when it's more than 90 days old. |
| `source` | The pricing page the numbers came from. |
| `models` | A map of model id → prices: `input`, `output`, `cache_write` (writing the cached prompt), `cache_read` (reading it), `batch_input`, `batch_output` (the batch API, used for evaluation later). |

---

## Notice fields

Under `notice:` — set by `job-radar init`, not normally edited by hand:

| Field | What it does |
|---|---|
| `accepted` | `true` once you've confirmed you read the security-and-privacy notice. |
| `version` | The notice version you accepted, so a changed notice can be re-shown. |

---

## Request limits (not configurable)

The outbound-request limits (identify ourselves, rate-limit per host, cap
requests per run, back off when asked) are enforced in code and **cannot be
lowered or disabled** — they keep job-radar a polite guest of every site it
contacts. See [ADR-0022](decisions/0022-hard-limits-on-outbound-requests-enforced.md).


## `email` fields

Under `email:` — alerts and the daily digest ([email-setup.md](email-setup.md)).
Email is **off until you list at least one recipient** in `to`. The sending
account's address and app password are secrets: they live in `.env`
(`RADAR_EMAIL_ADDRESS`, `RADAR_EMAIL_APP_PASSWORD`), never in this file.

| Field | Default | What it does |
|---|---|---|
| `to` | none (email off) | Who receives alerts and the digest, usually your main inbox. More than one address is allowed. |
| `match_alerts` | `true` | One email at the end of a run listing new strong matches (score at or above `thresholds.alert`). |
| `digest` | `true` | One daily summary of everything at or above `thresholds.digest` from the last 24 hours, grouped by track. |
| `digest_time` | `"08:00"` | Local time after which the first run of the day sends the digest. |
| `near_misses` | `5` | How many "just below the line" jobs (scored one point under `thresholds.digest`) the digest adds, so you can reply 👍 to any that should have scored higher. `0` turns it off. See [giving-feedback.md](giving-feedback.md). |
| `smtp_host` / `smtp_port` | `smtp.gmail.com` / `587` | The server used to send. |
| `imap_host` / `imap_port` | `imap.gmail.com` / `993` | The server used to read your replies. |

Each track under `rules.tracks` may also have a `label`, the heading the
digest uses for that track (for example `label: "Lead / hands-on roles"`).
Without one, the track's name is tidied up (`lead_ic` becomes "Lead IC").

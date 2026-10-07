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

Default: `rules`. (The rules scorer lands in phase 2b, the LLM scorer in phase 3.)

### `companies`
A list of the companies you want to watch. May be empty. Each entry is a
[company](#company-fields). Find a company's board with `job-radar discover`.

### `location`
Your [location rules](#location-fields), used by the pre-filter.

### `prefilter`
The [pre-filter rules](#pre-filter-fields) that drop obvious non-matches before
scoring. Optional — sensible software-role defaults are used if you omit it.

### `rules`
The [rules-scorer settings](#rules-fields) used when `scorer` is `rules`: your
tracks, keyword boosts, red flags and thresholds.

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
| `notes` | Free text about the company (the AI scorer can read this later). Optional. | `"Great React team"` |
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
| `hybrid` | When a hybrid role is acceptable. | `home_only` (office in your home area) or `any` |
| `onsite` | When an on-site role is acceptable. | `home_only` or `any` |
| `remote_open_to` | A remote role is kept only if open to one of these regions. | e.g. `["United Kingdom", "Europe", "EMEA"]` |
| `not_stated` | What to do when the work model isn't stated. | `keep_if_home_or_unknown` (never drop just for a missing model) |

> Location rules are fully applied by the rules scorer in phase 2b. In phase 2
> the pre-filter is conservative and does not drop on location (when in doubt,
> keep — see [ADR-0012](decisions/0012-pre-filter-rule-when-in-doubt.md)).

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

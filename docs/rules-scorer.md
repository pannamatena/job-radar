# The free rules scorer

job-radar's default scorer is free, deterministic and runs entirely on your
computer — no API key, no cost, no data sent anywhere. It reads each posting's
title and description and gives it a **1–5 score** using the keyword rules in
your `config.yaml`. This page explains how it works and how to tune it.

For the field-by-field reference, see [configuration.md](configuration.md#rules-fields).

---

## The formula

```
start at 1
  + 2   if the title matches one of your tracks
  + up to 2   for must_have_any keywords found (+1 each, capped at 2)
  + up to 1   for nice_to_have keywords found
  − 1   for each distinct red_flag matched
→ clamped to 1–5
```

Then two more adjustments:
- **Work model + location:** a role that breaks your location rules (e.g. an
  on-site role outside your home area) is capped at 2 and flagged
  `outside_home_area`.
- **Age is ignored.** A role posted two years ago is scored exactly like one
  posted today ([ADR-0025](decisions/0025-score-fit-independently-of-posting-age.md)).

All keyword matching is **case-insensitive** and on **whole words**, so `ai`
won't match "maintain" and `node` matches "Node.js".

---

## Reading a score: `score-one`

To see exactly why a posting got its score:

```sh
job-radar score-one <posting-id-or-url>
```
Get an id from `job-radar list --ids` or the `run` output, or just paste the
posting's URL. (If an id contains a space — some Ashby slugs do — quote it, or
use the URL.) You'll see the breakdown:

```
Score:      4/5  (rules)
Track:      lead_ic
Work model: remote
Why:        mentions react, typescript, frontend (rules)
Gap:        no obvious gaps from keywords (rules)

How the score was reached:
  +1 base
  +2 must-have (react, typescript, frontend)
  +1 nice-to-have (startup)
```

The `(rules)` tag is there so you always know a keyword scorer produced this,
not the AI.

---

## Tuning it

Everything lives under `rules:` in your `config.yaml`. Edit, then re-run
`job-radar list` / `score-one` to see the effect.

**Tracks** — the strongest signal. A title matching a track's `titles` adds +2
and tags the posting with that track:
```yaml
rules:
  tracks:
    manager:
      titles: ["engineering manager", "head of engineering"]
    lead_ic:
      titles: ["staff engineer", "senior frontend", "tech lead"]
```
If a title matches more than one track, the **longest matching phrase wins**.

**Boosts** — `must_have_any` are the skills that matter most (each +1, capped at
+2 total); `nice_to_have` are smaller pluses (+1 total):
```yaml
  must_have_any: ["react", "typescript", "frontend", "node"]
  nice_to_have:  ["startup", "remote", "ai"]
```

**Red flags** — things that make a role a worse fit. Each distinct flag found
subtracts 1 and is recorded on the posting:
```yaml
  red_flags:
    wants_go:           ["golang", "hands-on go"]
    cs_degree_required: ["degree in computer science required"]
```

**Thresholds** — the cut-offs for alerts and the digest:
```yaml
  thresholds: { alert: 4, digest: 3 }
```

---

## Its honest limits

The rules scorer only sees keywords. It can't tell an "Engineering Manager" at
a software startup from one in railway engineering (the **pre-filter** catches
the obvious non-software cases — see [configuration.md](configuration.md#pre-filter-fields)),
and it will happily score a mid-level "Software Engineer II" highly if the ad
mentions your keywords. That nuance — reading the whole ad like a person,
checking requirements against your profile — is what the optional **AI scorer**
adds in a later release ([ai-scorer.md](ai-scorer.md)). The rules scorer is the
free baseline that the AI has to beat, and for many roles it's plenty.

See [writing-your-profile.md](writing-your-profile.md) for how to choose good
keywords.

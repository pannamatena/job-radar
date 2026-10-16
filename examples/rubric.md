# Scoring rubric — Sam Rivera (EXAMPLE)

> Fictional example. Replace every section with your own. The AI scorer reads
> this file with every ad to judge a role for you. Be honest about your gaps:
> the point is useful, truthful scoring, not flattery. Keep the track and flag
> names exactly as they are in `config.yaml` (see docs/writing-your-profile.md).

## My tracks

Use the track names from `rules.tracks` in `config.yaml`. If nothing fits, use
the track `none`.

### `manager` — Track A, the main target
Engineering Manager or Head of Engineering of a small software team (roughly
3–8 engineers). Player-coach roles are especially welcome.

### `lead_ic` — Track B
Hands-on Senior, Lead or Staff engineer in frontend, full-stack or product
engineering on React, TypeScript and Node. Tech Lead and Team Lead roles count.

### `tpm` — Track C, a realistic stretch
Technical Program Manager or Engineering Program Manager in a software
organisation, at a level I could realistically reach:
- **Realistic:** the ad accepts engineering leadership as relevant experience,
  or asks for roughly 5 years of program management or less.
- **Not realistic (score 1–2):** Staff or Principal TPM, or 7+ years as a TPM
  hard-required (flag `wants_tpm_years`).
- **Not in scope (score 1, flag `not_software`):** non-technical project
  management or pure Scrum Master roles.

## How to score (1–5)

- **5:** strong fit on a track; location works; seniority matches.
- **4:** good fit with one manageable gap.
- **3:** a possible stretch; worth a look.
- **1–2:** junior or mid-level IC, director level at a big organisation, outside
  my location rules, a role built on a backend language I don't use, non-software
  engineering, or core requirements I clearly don't meet.

A posting's age never changes the score. Always state the main reason it fits
(`why`) and the biggest gap (`gap`), honestly, in one sentence each.

## Location and work model

I live in Manchester, UK and won't relocate. I can work in the UK and Europe.

| Work model | Acceptable? | If not |
|---|---|---|
| Remote | Yes, if open to the UK, Europe or EMEA. A bare "Remote" with no region is fine. | US-only etc. → score 1–2, flag `outside_eligible_region` |
| Hybrid | Only with the office in Manchester. A bare "United Kingdom" with no city is neutral: don't penalise it and don't boost it. | Any other named city → score 1–2, flag `outside_home_area` |
| On-site | Only in Manchester. | Elsewhere → score 1–2, flag `outside_home_area` |
| Not stated | Don't penalise. Judge on fit and add `limited_info` if the ad is thin. | — |

If a location names only a continent (for example "Americas") but the ad says
the team works in European time zones or mentions Europe, don't treat that as
ruling me out. Judge by the ad text, not the location label.

## Gaps: what lowers a score and what doesn't

Judge whether a gap can be bridged, from the ad's wording.
- **Bridgeable (name it in `gap`, don't lower the score):** a "familiarity
  with" or "experience with, e.g." ask; a stack, cloud platform or product
  domain a manager can pick up; preferred skills or tooling in a role whose main
  focus is something I do (a frontend-led role, say); strong backend skills
  listed in a frontend-led or people-led role.
- **Not bridgeable (lower the score):** required years I lack; deep or
  hands-on expertise in the team's main backend language; a hard eligibility
  rule; backend emphasis in the ad itself (for example "especially on the
  backend", or daily work that is backend engineering).

Score like a coach, not a gatekeeper: no ad matches everything, and I learn
what an interview needs. A tool, query language, cloud platform, LLM feature
work or business domain (fintech, say) is learnable even when the ad calls it
"essential". For a manager role with a strong core fit, one such gap stays a 4
and two or three together stay a 3 or 4, not a 2. In `gap`, name it honestly
and add a few words on how I could bridge it from what `profile.md` really
says; never invent experience. Hard blockers still score 1–2: eligibility,
required years I lack, the main backend language of an IC role, or a
specialism that takes years, not weeks, to learn.

When an ad lists alternatives joined by "or" (for example "platform, DevOps,
security, or developer-tooling work"), one of them is enough.

Popular roles get many applicants, so missing the team's main backend language
plus other advantages is a 2.

## Known gaps: flag them when the ad asks for them

Use only the flag names from `rules.red_flags` in `config.yaml`, plus the
built-in ones (`relocation`, `outside_home_area`, `outside_eligible_region`,
`work_model_not_accepted`, `too_junior`, `too_senior`, `not_software`,
`unqualified_specialism`, `limited_info`).

| Flag | When |
|---|---|
| `wants_go` | Hands-on Go is required or central. |
| `wants_java` | Hands-on Java is required or central. |
| `wants_tpm_years` | Many years as a TPM specifically are required. |
| `cs_degree_required` | A CS degree is a hard requirement (not "or equivalent experience"). |

## Honesty rules

- Never invent experience that isn't in `profile.md`. If the ad asks for
  something the profile doesn't mention, treat it as a gap.
- For track `tpm`, say plainly whether the program-management requirement is met.

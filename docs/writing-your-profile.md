# Writing your profile and rules

How well job-radar scores roles depends on what you tell it about yourself.
There are two places that live in your config folder:

- **`config.yaml` → `rules:`** — the keywords the **free scorer** uses *today*.
- **`profile.md` and `rubric.md`** — plain-English files the **AI scorer** reads
  (a later release). Worth writing now; they're also just a good record of what
  you're looking for.

This page shows how to make both work well, with good and bad examples.

---

## Tuning the rules (free scorer)

The free scorer only knows the keywords you give it, so **be specific and
honest**. See [rules-scorer.md](rules-scorer.md) for the mechanics; here's how to
choose good words.

**Tracks** — list the real titles you'd take, not aspirational ones. Include the
variations boards actually use.

🚫 Too vague:
```yaml
  tracks:
    manager: { titles: ["manager"] }        # matches Account Manager, Product Manager…
```
✅ Specific:
```yaml
  tracks:
    manager:
      titles: ["engineering manager", "head of engineering", "software development manager"]
    lead_ic:
      titles: ["tech lead", "staff engineer", "senior frontend", "senior full stack"]
```

**must_have_any** — the handful of skills that genuinely signal a fit. Don't
stuff it; every extra word makes the score noisier.

🚫 `["react", "javascript", "web", "software", "engineer", "developer"]` — so
broad almost everything scores high.
✅ `["react", "typescript", "frontend", "node"]` — the things that actually
distinguish *your* roles.

**red_flags** — be honest about your gaps. These keep the score truthful instead
of flattering.

A red flag's only job is to **pull down a role that would otherwise score
high** — so only list the catches that can hide *inside an otherwise attractive
role*. Example: a React/TypeScript manager role at a startup that also wants
hands-on Go is a near-miss worth flagging (`wants_go`). But you don't need to
list every language or skill you lack: a role built around one of those won't
match your tracks or `must_have_any` in the first place, so it already scores 1
— there's nothing to pull down, and flagging it just adds noise (or risks
docking a good role that mentions the word in passing).

Each red flag is `name: [phrases]`. The name is your label; the phrases are
matched (whole word) against the title and body; each distinct flag found is
−1 and is recorded on the posting. Use phrases specific enough not to misfire:

🚫 `wants_go: ["go"]` — matches "go-getter", "go-to", "ongoing".
✅ `wants_go: ["golang", "hands-on go", "strong go"]`

Watch out for flags that misfire on roles you *could* take. A broad
`["authorised to work", "visa sponsorship"]` would also dock a local role that's
perfectly fine for you — whether that line is a dealbreaker depends on the
role's location versus where you're eligible, which your `remote_open_to`
location rule already handles. Flags work best for **unconditional**
dealbreakers (e.g. a required security clearance or citizenship you don't hold).

> Tip: after editing, run `job-radar score-one <id>` on a few roles you know
> well and check the scores match your gut. Adjust the words until they do.

---

## Writing `profile.md` (for the AI scorer)

`profile.md` is the single source of truth about you. The AI scorer treats it as
fact and **will never invent experience that isn't in it** — so write it
carefully and keep it current.

A good profile is **specific and honest**:

✅ Good:
> Lead Engineer, 9 years' experience, last 4 leading teams of 4–6. Strong in
> React/TypeScript and Node; comfortable owning a frontend codebase end to end.
> Light on Go and large-scale distributed systems. Based in Manchester; open to
> hybrid locally or remote within the UK/Europe.

🚫 Too vague to score with:
> Experienced engineer and leader looking for exciting opportunities at a
> forward-thinking company.

Include: your level and years, the languages/frameworks you're actually strong
in, the kinds of teams/roles you've held, your location and work-model
preferences, and — importantly — your **honest gaps**.

---

## Writing `rubric.md` (for the AI scorer)

`rubric.md` is your scoring guide in plain English: your tracks, what makes a 5
vs a 3, and the gaps to call out. The AI reads it alongside each ad.

A good rubric:
- names your **tracks** and what counts for each ("Track A: Engineering Manager
  of a small software team; player-coach roles especially welcome");
- says what makes a **5, 4, 3** and what's a **1–2**;
- lists your **known gaps** and tells the scorer to flag them honestly
  ("if the role hard-requires 8+ years as a TPM, say so in the gap");
- reminds it to **never invent experience** not in `profile.md`.

See [`examples/profile.md`](../examples/profile.md) and
[`examples/rubric.md`](../examples/rubric.md) for a complete worked example
(the fictional persona).

> The AI scorer itself arrives in a later release — see [ai-scorer.md](ai-scorer.md).
> Until then, `profile.md` and `rubric.md` aren't used by the free scorer, but
> writing them now means you're ready, and they double as a clear record of
> what you want.

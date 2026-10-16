# Writing your profile and rubric

job-radar judges every job against what *you* tell it. This page walks you
through writing that down. You don't need to be technical — if you can write an
email, you can do this.

**Contents**

1. [What these files are and how to edit them](#part-1--what-these-files-are-and-how-to-edit-them)
2. [Writing `profile.md`](#part-2--writing-profilemd) (with a template)
3. [Writing `rubric.md`](#part-3--writing-rubricmd) (and how it lines up with `config.yaml`)
4. [Tuning the keyword rules](#part-4--tuning-the-keyword-rules-free-scorer) (free scorer, a bit more technical)
5. [Check that it worked](#part-5--check-that-it-worked)

---

## Part 1 — What these files are and how to edit them

`job-radar init` creates a folder called **`private/`** inside the job-radar
folder (or wherever `JOB_RADAR_HOME` points). It holds three files you edit:

| File | What it is | Who reads it |
|---|---|---|
| `profile.md` | Facts about you: experience, skills, gaps, preferences. | The AI scorer |
| `rubric.md` | Your scoring guide: what you want and how to rate a role. | The AI scorer |
| `config.yaml` | Companies to watch, location rules and keyword rules. | Everything |

- **`.md`** means *Markdown*: a plain text file. `# Title` makes a heading and
  `- item` makes a bullet. That's all you need.
- **Any plain text editor works**: TextEdit (Mac, use *Format → Make Plain
  Text*), Notepad (Windows), VS Code, or similar. Don't use Word or Google
  Docs; they add hidden formatting.
- **There are two scorers.** The free **rules scorer** reads only `config.yaml`.
  The optional **AI scorer** ([ai-scorer.md](ai-scorer.md)) reads `profile.md`
  and `rubric.md` too. If you'll only use the free scorer, you can skip Parts 2
  and 3, but writing them is still a good record of what you want.
- The files `init` creates start as a **fictional example person** (Sam
  Rivera). Replace them entirely with your own details.

> **Privacy:** `profile.md` and `rubric.md` are sent to the AI provider with
> every posting the AI scorer scores. Keep them free of anything you wouldn't
> want sent: see [what to leave out](#what-to-leave-out).

---

## Part 2 — Writing `profile.md`

`profile.md` is the single source of truth about you. The AI scorer treats it as
fact and **never invents experience that isn't in it**. So be accurate, and be
honest about your gaps. A flattering profile gives flattering (useless) scores.

### Turn your CV into a profile in 4 steps

1. **Paste your CV into `profile.md`** as a starting point.
2. **Cut the noise.** Delete contact details, references and filler such as
   "passionate team player". Shorten each job to 2–4 bullets of what you did
   and the scale (team size, users, results).
3. **Add what a CV leaves out:** the skills you're *strong* in versus only
   *light* on, where you can work, and what you *don't* want.
4. **Check the length.** Aim for **one to two pages (roughly 300–700 words)**.
   The whole file is sent with every posting, so a longer profile costs more
   per posting and dilutes the signal.

### Template

Copy this and fill it in. A finished example is in
[`examples/profile.md`](../examples/profile.md).

```markdown
# Career profile — <your name>

## Summary
Your level, years of experience, what you're strongest at and what you're
looking for, in 3–4 sentences.

## Experience
**<Job title> — <Company> (<years>)**
- What you did, with scale: team size, users, results.
- ...

## Skills
- **Strong:** things you could do in your sleep.
- **Working knowledge:** things you've used but wouldn't lead on.
- **Light / learning:** honest gaps. This section matters most.
  Also write down what you've *touched* or are learning now (a database you
  used at work, a side project with AI). The scorer can only encourage you
  about what it can see, and these are what you can say in an interview.

## Education
- <Degree, institution, year>

## Preferences
- Location: where you live, and where you'd work (onsite, hybrid, remote).
- Interested in: the kinds of companies and work you want.
- Not interested in: dealbreakers.
```

✅ Good:
> Lead Engineer, 9 years' experience, last 4 leading teams of 4–6. Strong in
> React/TypeScript and Node; comfortable owning a frontend codebase end to end.
> Light on Go and large-scale distributed systems. Based in Manchester; open to
> hybrid locally or remote within the UK/Europe.

🚫 Too vague to score with:
> Experienced engineer and leader looking for exciting opportunities at a
> forward-thinking company.

### What to leave out

- Phone number, email, home address, date of birth.
- Current or past salary (put a *target* in the rubric only if you want it used).
- Anything confidential about an employer: client names, internal numbers.
- Anything you wouldn't be comfortable having sent to an AI provider.

---

## Part 3 — Writing `rubric.md`

`rubric.md` is your scoring guide in plain English: which kinds of role you
want, what makes a 5 versus a 3, and the gaps to call out. The AI reads it
alongside each job ad. A finished example is in
[`examples/rubric.md`](../examples/rubric.md).

**Don't start from a blank page.** `job-radar init` already gives you a complete
rubric in [`examples/rubric.md`](../examples/rubric.md): tracks, scoring bands,
a location table, how to treat gaps, and a flag table. Edit each section for
you. Two parts are worth understanding, because they come from lessons we
learned the hard way:

- **Gaps: bridgeable or not.** A missing "familiarity with…" skill, or a stack a
  manager can pick up, shouldn't sink a role; a missing main backend language
  or a hard requirement should. Without this the AI tends to treat every
  unmet requirement the same way. The example spells it out.
- **"or" lists.** If an ad asks for "A, B, **or** C", one is enough.
- **Coaching, not gatekeeping.** No ad matches everything, and you'll learn
  what an interview needs. job-radar's AI scorer is built to think that way
  whatever your rubric says: a gap you could learn quickly (a tool, SQL, a
  business domain, building with AI) lowers a score only a little, even when the
  ad calls it "essential", and the **gap** line says what's missing *and how you
  could close it or talk about it*, using only what's in your profile. Your
  rubric still sets the hard limits (where you can work, a language you won't
  use, a specialism that takes years), and those score 1–2. Years of
  experience are a soft signal: if you do that kind of work, being a year or
  three short never sinks a role on its own, and the more senior the role, the
  less it matters ([ADR-0033](decisions/0033-years-of-experience-are-a-soft-signal.md)). If you want a stricter scorer, say so in the rubric
  ([ADR-0032](decisions/0032-the-scorer-coaches-by-default.md)).

A good rubric:

- names your **tracks** (the kinds of role you'd take) and what counts for each
  ("Track A: Engineering Manager of a small software team; player-coach roles
  especially welcome");
- says what makes a **5, 4, 3** and what's a **1–2**;
- lists your **known gaps** and tells the scorer to flag them honestly ("if the
  role hard-requires 8+ years as a TPM, say so in the gap");
- reminds it to **never invent experience** that isn't in `profile.md`.

### Keep the names the same as in `config.yaml`

The AI can only record tracks and flags it knows about, and it learns them from
your `config.yaml`. So the names you use in `rubric.md` must match. Here is how
they line up (Sam's example):

| In `config.yaml` | In `rubric.md` |
|---|---|
| `tracks:`<br>`  manager:`<br>`    titles: [...]`<br>`  lead_ic:`<br>`    titles: [...]` | `**Track A — manager:** Engineering Manager of a small team…`<br>`**Track B — lead_ic:** hands-on Senior/Lead engineer…` |
| `red_flags:`<br>`  wants_go: [...]` | `Hands-on Go is light for me; flag it.` (the AI records it as `wants_go`) |

- **Track names** are the keys under `tracks:` in `config.yaml` (`manager`,
  `lead_ic`, `tpm` in the example). Use exactly those words.
- **Flag names** are the keys under `red_flags:` in `config.yaml`, plus these
  built-in ones: `relocation`, `outside_home_area`, `outside_eligible_region`,
  `work_model_not_accepted`, `too_junior`, `too_senior`, `not_software`,
  `unqualified_specialism`, `limited_info`.

If you rename a track in one file, rename it in the other.

You don't have to catch mismatches by eye. Whenever the AI scorer starts
(`run --scorer llm`, `score-one --scorer llm` or `eval`), job-radar compares the
two files and prints a warning for:

- a flag the rubric describes that `config.yaml` doesn't define (the AI can never
  set it; add it under `rules.red_flags`, with an empty list if you want the AI
  alone to use it, like `wants_java: []`), and
- a track or flag in `config.yaml` that the rubric never mentions (write its name
  in `backticks` so the AI knows when to use it).

> Any edit to `profile.md` or `rubric.md` changes the AI scorer's prompt
> version, so postings are re-scored against the new wording on the next run.

---

## Part 4 — Tuning the keyword rules (free scorer)

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

## Part 5 — Check that it worked

Don't trust the files until you've tested them against roles you already have
an opinion on.

1. Pick **3–5 postings you know well**: some you'd love, some you'd never take.
   Run `job-radar list` to find their ids.
2. Score each one:
   ```sh
   job-radar score-one --scorer llm <posting-id-or-url>
   ```
   (Add no flag to see the free rules scorer's score instead.)
3. Compare each score with your gut. Read the **why** and **gap** lines: do
   they describe the role and *you* accurately?
4. If a score is off, the fix is almost always in the files:
   - Scored too high → add the missing dealbreaker to the rubric's "1–2" list
     or to your gaps.
   - Scored too low → check the rubric's tracks cover that kind of role.
   - The gap line invents something about you → add the true fact to
     `profile.md`.
5. Edit, re-run, repeat. Aim for 4 out of 5 matching your gut.


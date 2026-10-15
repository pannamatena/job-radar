# Evaluation: is the scorer actually right?

An AI scorer that *sounds* sensible isn't the same as one that's *right*. The
`eval` command measures how often a scorer agrees with **your own judgement** on
a set of job ads you've scored by hand. Use it to answer three questions:

- Is the AI scorer better than the free rules scorer, and by how much?
- Did my last change to the rubric, the prompt or the model make things better
  or worse?
- What does the AI scorer really cost per ad?

The set is small (aim for 20–30 ads), so treat the numbers as a rough guide,
not a measurement. The report says so whenever you have fewer than 20 cases.

---

## 1. Make your eval cases

Each case is one folder in `evals/cases/` inside your config folder (so
`private/evals/cases/`). It holds two files:

```
private/evals/cases/
  frontend-team-lead-example/
    ad.txt        the job ad as plain text
    label.yaml    your judgement of it
```

**`ad.txt`**: paste the ad. If the first line looks like
`Job title — Company (Location)`, the text in the last brackets is used as the
location the scorer sees, just like the location a job board gives. The line is
then removed from the ad body. For example:

```
Senior Frontend Engineer (Team Lead) — Meadowlark (Manchester, hybrid)
```

**`label.yaml`**: your judgement. **You write the labels, never the tool.**

```yaml
status: confirmed       # draft until you've finished; only confirmed cases are measured
company: Meadowlark
title: Senior Frontend Engineer (Team Lead)
source: https://…       # optional: where the ad came from
score: 4                # 1–5, how good a fit it is for you
track: lead_ic          # one of your tracks from config.yaml, or none
work_model: hybrid      # remote | hybrid | onsite | not_stated
office: Manchester      # or leave empty
flags: []               # flags that apply, e.g. [wants_go, outside_home_area]
note: >
  One line on why you gave this score.
why_tricky: ""          # optional: what makes this case a good test
```

Cases with `status: draft` are listed in the report but left out of the
numbers. A *confirmed* case with an incomplete label is an error that names the
case, so a half-filled label can't quietly change your results.

**Which ads to pick.** A mix is more useful than a pile of obvious ones:

- clear yeses and clear nos,
- an engineering-manager role that wants hands-on Go,
- a hybrid role outside your home city,
- an ad that doesn't say the work model,
- a junior role with a lead-ish title,
- a strong fit with an old posting date (age should not lower the score),
- a role that *looks* like a fit but needs a specialism you lack.

You can also add a case from an alert you disagreed with: copy the ad text into
a new folder and label it.

> Your cases describe *you*, so they stay in your private config. The folder
> `examples/evals/` has two draft cases for the fictional persona; label them
> yourself to try the command without any personal data.

---

## 2. Run it

```sh
# Free: the rules scorer
job-radar eval --scorer rules

# The AI scorer. It prints an estimate and asks before spending anything.
job-radar eval --scorer llm

# Same, but approve the cost up front (for scripts)
job-radar eval --scorer llm --yes
```

| Flag | What it does |
|---|---|
| `--scorer rules\|llm` | Which scorer to measure. Default `llm`. |
| `--model NAME` | Use a different model for this run, e.g. to try a larger one. It must have a price in `llm.pricing`. |
| `--repeats N` | Score each case N times to see how consistent the scores are. Costs N times as much. |
| `--batch` | Use the batch API: about half the price, but it waits for the provider (see below). |
| `--yes` | Skip the "go ahead?" question. |
| `--compare A B` | Compare two saved runs (see below). |

### `--batch`: half price, slower

`--batch` sends the work through the provider's batch API, which costs about
half as much but doesn't answer immediately: a batch usually finishes within
the hour and can take up to a day. That suits an eval (you can leave it running)
but would be wrong for live alerts, so only `eval` uses it.

How it works, since the agent needs several model calls per ad (read the ad,
look something up, record the score) and a batch is a set of *independent*
requests: the eval works in **rounds**. Each round sends one request for every
scoring that isn't finished as one batch, waits, feeds the replies back, and
repeats. A typical ad needs two or three rounds, so an eval takes two or three
batch waits. You'll see a line per round, and Ctrl-C cancels the batch so
nothing keeps running (and nothing more is billed).

In a first test each round took about five minutes, so a two-round eval took
about eleven minutes, however few ads there were. The wait is the provider's
queue, not your ad count.

What to know:
- **Latency isn't measured** in a batch run, and the report says so.
- **Cost is reported at batch prices.** The estimate before the run is halved,
  and the real figure uses `batch_input` and `batch_output` from `llm.pricing`.
- **Prompt caching is best-effort in batches.** Because rounds can be minutes
  apart and the cache only lives for a few minutes, you may pay for cache writes
  more often than in a normal run. It stays cheaper overall.
- The saved file is named `…_batch.json`, so you can tell it from a normal run.
  A batch run and a normal run of the same prompt should give much the same scores
  (the model and prompt are the same); `--compare` them to check.
- `--batch` needs `--scorer llm`.

Notes on how it runs:

- The AI scores **every** confirmed case, not only the ones the rules gate would
  let through. That way the report can tell you honestly what the gate would
  have hidden (see below).
- Every run is saved as JSON in `evals/results/`, named
  `<date>_<prompt-version>_<model>.json` (or `<date>_rules.json`). A second run
  on the same day gets `-2`, never overwriting the first.
- Eval spend isn't added to the monthly spend that `run` tracks, but an eval is
  refused if it would take you past `llm.monthly_spend_cap_usd`. Keep an eye on
  your Anthropic Console as well.

---

## 3. Reading the report

**Score accuracy**: *exact match* is how often the scorer gave exactly your
score; *within ±1* allows one point either way.

**At the alert threshold** is the number that matters most. A score at or above
`rules.thresholds.alert` triggers an alert, so what counts is the two kinds of
mistake:

- **wrong alerts**: it would interrupt you for a role you'd have said no to,
- **missed good roles**: you'd have wanted it, but it would have stayed quiet.

Missing a good role is usually worse than one extra alert, so look at this pair
before the exact-match percentage.

**Track and work model right** are plain accuracy. **Flags** show, per flag,
*precision* (of the flags it set, how many you agree with) and *recall* (of the
flags you expected, how many it set).

**Cost and reliability** (AI only): turns per ad, invalid `record_score` calls
the model had to correct, tokens including cache hits, cost per ad, latency,
and an estimated monthly cost at 10 and 20 ads a day, with and without the
rules gate.

**The rules gate**: in real runs an ad whose rules score is below
`llm.min_rules_score` is never sent to the AI. The report counts how many of
your cases that would hide, and **warns if any of them was a good role**. If it
does, lower the gate.

**Consistency** (with `--repeats`): how many cases got the same score every
time, and the average spread.

**Disagreements** list every case where the scorer's score differs from yours,
with the scorer's *why* and *gap* next to your note. This is the most useful
part for improving your rubric.

If the AI didn't finish a scoring (an API error, or it hit its turn limit), that
scoring is counted but left out of the accuracy numbers, since its score would
be the rules fallback, not the AI's.

---

## 4. Comparing runs

```sh
job-radar eval --compare 2026-10-13_rules.json 2026-10-13_a74a39ddd3_claude-haiku-5-5.json
```

File names are looked up in `evals/results/`. You get the summary numbers side
by side, then each case's score from you, from A and from B, with `≠` marking
the cases where A and B differ.

`--compare` also prints a **borderline cases only** block: the cases you rated
3–4, or where either run scored 3–4. Models usually agree on the obvious
cases, so that is where a cheaper or pricier model shows its difference.

Useful comparisons, changing **one thing at a time**:

1. **Rules against AI** on the same cases: what does the paid option buy you?
2. **Before and after a rubric or prompt change**: the prompt version in the
   file name changes whenever your profile, rubric or the built-in instructions
   change.
3. **One model against another** (`--model`): is the bigger model worth the
   price, especially on the borderline cases you scored 3–4?

---

## How I used this on my own set

See [eval-lessons.md](eval-lessons.md) for the process I followed and the
problems I hit (a rule that fixed one case and broke another, a misleading
location label, a bug the eval exposed in the agent).

## Limits

- 20–30 cases can't prove much. Differences of one or two cases are noise.
- The cases are scored by you, so the eval measures agreement with your taste,
  and your taste can change.
- An eval is only as good as its labels: keep the notes honest.

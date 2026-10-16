# Telling job-radar when it gets a job wrong

job-radar can score a job too high (you got an alert for a job you'd never take)
or too low (a job you'd have loved never reached you). Both are worth telling it.
This page shows how, from your phone and from the terminal.

**What your feedback does.** It is recorded so *you* can see it, and so you can
turn the clearest mistakes into test cases for the scorer (see
[evaluation.md](evaluation.md)). A vote does not change any score by itself, and
nothing is "trained" behind your back: you decide what to change in your rubric
and profile, then measure whether it helped.

You'll need email set up first: [email-setup.md](email-setup.md).

---

## 1. A job that was scored too high

It reached you in an alert or the digest, and it isn't a good fit.

**Reply to the email** with the job's number and 👎:

```
2 👎
```

You can send several at once, in any order: `1 👍, 2 👎, 5 👎`. Also accepted:
`1 up 3 down`, `+1` / `-1`, `yes` / `no`. Only the numbers and these words are
read; everything else in your reply is ignored.

A 👍 on a job that was scored high means "good call". It is recorded, nothing else happens.

## 2. A job that was scored too low

There are two ways to find these.

### In the daily digest: "Just below the line"

At the bottom of the digest is a short section of jobs that scored *one point
under* your digest threshold (normally 2), best first, up to 5, each with the
reason it scored low. If one of them should have scored higher, **reply with its
number and 👍**:

```
7 👍
```

Change the number with `email.near_misses` in `config.yaml` (`0` switches the
section off).

### In the terminal: `job-radar list --low`

```sh
job-radar list --low             # last 14 days, up to 30 jobs
job-radar list --low --days 30 --limit 50
```

This reads your saved jobs (it doesn't fetch anything from the internet) and
shows everything scored below your digest threshold, with each job's **id**, its
score, your current vote if any, and why it scored low. To say one deserved more:

```sh
job-radar feedback "greenhouse:acme/12345" up
```

(Copy the id from the table. Quote it, because ids contain a colon.)

> **Jobs the pre-filter dropped** aren't saved, so they can't be voted on. If
> good jobs keep disappearing there, look at `job-radar list --all` and loosen
> `pre_filter` in `config.yaml` (see [configuration.md](configuration.md)).

## 3. Did my vote count? Changing your mind

- `job-radar run` and `job-radar feedback sync` print a line for each vote they
  record, e.g. `Recorded 👎 Acme — Staff Engineer`, plus "(changed from 👍)" if
  you flipped it.
- If a reply held no vote job-radar could read, `feedback sync` says so.
- The next digest lists the votes recorded since the last one.
- `job-radar report` shows your **current vote on each job**.
- Vote again any time. **The latest vote counts**; earlier ones stay in the history.

## 4. Turn a mistake into a test case

A test case is one job ad plus *your* judgement of it. Once you have a few, the
`eval` command can tell you whether a change to your rubric helped or hurt (see
[evaluation.md](evaluation.md)).

```sh
job-radar eval draft "greenhouse:acme/12345"     # one job, by id
job-radar eval draft --from-votes                # every job where your vote disagrees with its score
```

"Disagrees" means a 👍 on a job below your **alert** threshold, or a 👎 on a job
at or above it.

Each draft is a folder in `evals/cases/` with the ad text and a `label.yaml` whose
judgement fields are **blank**. job-radar never writes a label for you. Open
`label.yaml`, fill in your own score, track and work model, and change
`status: draft` to `status: confirmed`. Delete the folder if you don't want that
case. Unfinished drafts are ignored by `eval`.

## Quick reference

| I want to… | Do this |
|---|---|
| Say an alert/digest job was a bad call | Reply `N 👎` |
| Say a "just below the line" job should have scored higher | Reply `N 👍` |
| Find low-scored jobs from the terminal | `job-radar list --low` |
| Vote from the terminal | `job-radar feedback "<id>" up` or `down` |
| Read my replies right now | `job-radar feedback sync` |
| See my current votes | `job-radar report` |
| Make a test case from a job | `job-radar eval draft "<id>"` or `--from-votes` |

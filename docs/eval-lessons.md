# How I evaluated the AI scorer, and what went wrong

This is the process I followed to check whether the AI scorer was right, and
the problems I hit along the way. It is a record of method, not a benchmark:
the set was small (5 cases at first, then 14), so none of the numbers here are
published as results. For how to run an eval yourself, see
[evaluation.md](evaluation.md). The decision behind the method is in
[ADR-0030](decisions/0030-tune-on-labelled-cases-one-change-at-a-time.md).

## The loop

1. **Label real ads by hand** (score 1–5, track, work model, flags, a one-line
   note). Labels are always the human's; the tool never writes one.
2. **Run both scorers** on the same cases: the free rules scorer and the AI.
3. **Read the disagreements**, not just the percentages. The AI's *why* and *gap*
   next to my note shows whether the scorer or the label is wrong.
4. **Change one thing**: a line in the rubric, a line in the profile, or a bug.
5. **Rerun with 3 repeats** and compare the two runs with `eval --compare`.

The number that mattered most was at the alert threshold: *wrong alerts* and
*missed good roles*, not exact-match percentage.

## What went wrong, and what I changed

### 1. A rule that fixed one case broke another
A rubric rule that lowered the score when several "advantages" were missing
fixed a role built on a backend language I don't know, and broke a role I'd
rated well. An ablation (the same eval on a scratch copy without the rule)
showed the rule helped one case and hurt the other. The fix was to narrow the
rule: lower the score only when the *team's main backend language* is missing,
and not for missing preferred tooling in a role whose focus is something the
candidate does.

**Lesson:** test a rule by removing it. Both directions of the trade-off show up.

### 2. The same words meant different things
Two ads both asked for backend skills. One was a UI-led role listing "strong
backend engineering" among its requirements; I rated it a good fit. The other
was a people-leadership role saying "solid technical foundation, especially on
the backend"; I rated it poorly. The difference was not the word "backend" or
whether it sat under "required", but where the ad put its emphasis. The rubric
now separates **bridgeable** gaps (familiarity asks, a stack or platform a
manager can pick up, backend listed in a UI-led role) from **not bridgeable**
ones (required years, deep expertise in the team's main language, hard
eligibility rules, backend emphasis in the ad itself).

**Lesson:** when a scorer and a human disagree, ask what the human is really
reading. Writing that down as a rule exposed the inconsistency in my own labels.

### 3. A profile line was over-applied
The profile said the candidate had no infrastructure or DevOps experience. An ad
asked for experience owning "platform, DevOps, security, **or** developer-tooling
work", and the model treated the whole list as unmet. The candidate did have
developer-tooling and security-process experience, but the profile never called
it that. Fixes: name that experience in the profile, and add a rubric line that
an "or" list needs only one item.

**Lesson:** the model reads the profile literally and only infers what's
written. A "don't claim X" line can spill over onto neighbouring skills.

### 4. The board's location label disagreed with the ad
One ad's board field said "Americas", while the ad text said the team spans
European time zones. The AI followed the label and ruled the role out. The
case header was changed to match what the ad says, and a rubric line says a
continent-only label doesn't rule out a role whose text mentions Europe.

**Lesson:** an eval case must show the scorer what the real tool would see.
Job boards' structured fields can be wrong or coarser than the ad.

### 5. Labels are not ground truth
Rereading one ad, I decided my own label was wrong (a cloud-platform ask I'd
treated as a blocker was clearly bridgeable) and relabelled it. The scorer had
been right. Because each results file stores the labels as they were at run
time, a later run looked like it had fewer wrong alerts, but that gain came
from the relabel, not from any change to the scorer.

**Lesson:** when comparing runs, check whether the labels changed. Never count
a relabel as an improvement. Consider the ad's exact wording; a summary of the
ad on an aggregator site can drop words like "essential".

### 6. The eval itself had gaps
- It first scored every ad directly, including ones the pre-filter drops in a
  real run, which overstated the rules scorer's wrong alerts. The eval now
  applies the pre-filter first and reports dropped cases separately.
- It scores every case with the AI, even those the rules gate would hide, so it
  can measure what the gate loses ([ADR-0029](decisions/0029-eval-scores-every-case-and-measures-the-gate.md)).

### 7. The eval found real bugs
- The model occasionally returned an *empty* reply, and the agent loop sent it
  back as an empty message, which the API rejects with a 400. A placeholder now
  stands in, with a regression test.
- The per-ad token budget counted cache-read tokens at full weight, so a long
  ad with four turns exceeded the cap while costing under a cent. The budget
  was raised in config; counting cache reads at reduced weight is a possible
  later change.

### 8. Single runs are noisy
Borderline cases flipped between neighbouring scores from run to run. Repeating
each case 3 times, and reading "same score every time" as a metric, stopped me
reading a one-run difference as a real one.

### 9. Overfitting is the main risk
I tuned the rubric and profile against the same few cases that I then measured
on. A high score on those cases is optimistic. I did not publish numbers from
them; tuned cases are "spent", and the next measurement needs ads the rubric
has not seen.

## What it cost

Every run was on a cheap model (Haiku class), roughly a tenth of a US cent per
ad, so a full 14-case, 3-repeat run cost a few cents. The whole tuning process
was well under a dollar. Cost was never the constraint; careful labelling was.

## If someone asks "how did you evaluate it?"

I hand-labelled real ads, measured wrong alerts and missed roles at the alert
threshold, compared the AI with the free rules scorer, changed one thing at a
time, repeated each run three times, and removed rules to see if they were
really the cause. The evaluation found bugs in my own code and in my labels as
well as in the prompt, and the numbers stayed unpublished until I have cases I
haven't tuned against.

# Command reference

Every job-radar command on one page, with examples you can copy. Replace the
example ids (like `greenhouse:acme/1234567`) with real ones from `job-radar list --ids`
or `job-radar list --scored`.

**Ids.** An id looks like `greenhouse:acme/1234567`. If the company name has a space, such as `ashby:Acme Labs/ab12…`, put the id in quotes.

**Flags that every command shares.** `--home <folder>` says where your config
lives. The default is the `JOB_RADAR_HOME` environment variable, or `./private`
if that isn't set. `-h` after any command prints its flags.

Commands marked 🌐 contact the internet (job boards, your email provider, the AI
provider). Commands marked 💰 can spend money on the AI scorer. All the others
read only your own files and database.

## Set up

| Command | What it does |
|---|---|
| `job-radar init` | Creates your config folder from a worked example. `--force` overwrites an existing one, `--yes` confirms you've read the [security notice](security-and-privacy.md). |
| `job-radar discover https://careers.example.com` | 🌐 Finds out which job board a careers page uses and prints the config entry to paste. `--name "Acme"` sets the company name in it. |
| `job-radar email-test` | 🌐 Sends one short test email, to check [email setup](email-setup.md). |
| `job-radar version` | Prints the version. |
| `job-radar help` | Lists the commands. |

## Look at jobs

| Command | What it does |
|---|---|
| `job-radar list` | 🌐 Fetches current postings for your watch-list and marks the new ones. |
| `job-radar list --ids` | Same, with each posting's id (needed by `score-one`, `show`, `feedback`). |
| `job-radar list --all` | Also shows postings the pre-filter dropped. |
| `job-radar list --companies` | Prints your watch-list, without fetching. |
| `job-radar list --no-store` | 🌐 Fetches without recording anything in the database. |
| `job-radar list --scored` | Stored jobs scoring 3 or more in the last 14 days, best first. No network. `--min 4`, `--days 30` and `--limit 50` change it. Jobs that have left their company's board are marked `[closed]`. |
| `job-radar list --low` | Stored jobs that scored *below* your digest line, to spot scores that are too low. `--days` and `--limit` work here too. |
| `job-radar show greenhouse:acme/1234567` | One stored job's score, which scorer gave it, why, the gap, flags, whether it has closed, and your vote. |

## Run it

| Command | What it does |
|---|---|
| `job-radar run` | 🌐💰 The full pipeline: fetch, search the web (if switched on), filter, score, record, email alerts, send the digest when it is due, and read your reply votes. |
| `job-radar run --dry-run` | 🌐💰 The same, but it marks nothing as seen and sends no email. AI spend is still recorded because it is real. Scores are saved. |
| `job-radar run --no-search` | 🌐💰 Skips web search for this run, when `search.enabled` is on. Search costs money; see [finding-jobs-by-search.md](finding-jobs-by-search.md). |
| `job-radar run --scorer llm` | 🌐💰 Uses the AI scorer for this run, whatever your config says (`--scorer rules` for the free one). |
| `job-radar score-one greenhouse:acme/1234567` | 💰 Scores one stored job (an id or its link) and shows why. With `--scorer llm` it shows the AI's whole transcript and its cost. |

## Email and votes

| Command | What it does |
|---|---|
| `job-radar digest` | 🌐 Sends the daily digest now. `--dry-run` prints it instead; `--hours 72` looks back 72 hours instead of since the last digest. |
| `job-radar feedback sync` | 🌐 Reads your email replies now and records the 👍/👎 votes in them. |
| `job-radar feedback greenhouse:acme/1234567 up` | Records a vote yourself (`up` or `down`). A later vote replaces an earlier one; the history is kept. |
| `job-radar report` | Recent runs, emails sent, AI cost this month against your cap, and your current vote on each job. `--days 30` for a longer period. |

More about voting in [giving-feedback.md](giving-feedback.md).

## Measure the scorer

| Command | What it does |
|---|---|
| `job-radar eval` | 💰 Scores your hand-labelled cases with the AI scorer and reports how often it agrees with you. `--scorer rules` uses the free scorer. |
| `job-radar eval --repeats 3` | 💰 Scores each case three times, to see how steady the scores are. |
| `job-radar eval --model claude-sonnet-5-5` | 💰 Uses another model for this run. It needs a price in `llm.pricing` in your config. |
| `job-radar eval --batch` | 💰 About half price; results take minutes to hours. |
| `job-radar eval --compare evals/results/a.json evals/results/b.json` | Compares two saved result files side by side. |
| `job-radar eval --skip-prefilter` | 💰 Scores every case, even ones the pre-filter would drop. |
| `job-radar eval --yes` | Skips the "this will cost about…" question. |
| `job-radar eval draft greenhouse:acme/1234567` | Makes a draft test case from a stored job. The label is left blank: only you write labels. |
| `job-radar eval draft --from-votes` | Drafts a case for every job where your latest vote disagrees with its score. |

More in [evaluation.md](evaluation.md) and [giving-feedback.md](giving-feedback.md).

## Common recipes

```sh
# What scored 3 or more recently, without waiting for the digest?
job-radar list --scored

# Why did this one score what it did?
job-radar show greenhouse:acme/1234567

# This score looks too low. Tell it, then keep it as a test case.
job-radar feedback greenhouse:acme/1234567 up
job-radar eval draft greenhouse:acme/1234567

# Try a run without sending email or marking anything as seen.
job-radar run --dry-run

# What have I spent on the AI this month?
job-radar report
```

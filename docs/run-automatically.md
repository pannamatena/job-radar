# Running it automatically (GitHub Actions)

Once this is set up, job-radar checks for jobs **twice a day on GitHub's servers**.
Your computer can be off. You only need it to edit your settings (which you can also
do in the browser on github.com).

It's free: a private repository gets 2,000 free Actions minutes a month, and a run
takes a couple of minutes (about 120 minutes a month at two runs a day). The only
possible cost is the AI scoring and web search you switch on, which have spend caps
([ai-scorer.md](ai-scorer.md), [finding-jobs-by-search.md](finding-jobs-by-search.md)).

**Prefer not to use GitHub?** See [Running it yourself](#running-it-yourself-without-github).

---

## How it works

You keep **two repositories**:

- the **public code repository** (job-radar itself): code, docs and the released programs;
- your **private config repository**: your `config.yaml`, `profile.md`, `rubric.md`,
  test cases, and a small workflow file. Nobody else can see it.

Every run, the workflow in your private repository:

1. downloads the newest released job-radar in the version series you chose, and
   **checks it** (published checksum, and GitHub's signed record that this project's
   release workflow built it). A file that fails is never run;
2. restores your database from the `state` branch of your private repository;
3. runs `job-radar run` (fetch, score, email you);
4. saves the database back to the `state` branch, even if the run failed, so alerts
   already sent aren't sent twice.

The Actions log holds counts, ids and errors only. The readable report (job titles,
scores, reasons) is not written to it.

---

## Set it up

You need a GitHub account with two-factor authentication on, and job-radar working
on your computer first ([getting-started.md](getting-started.md)).

### 1. Create the private repository

1. On github.com click **+** (top right) → **New repository**.
2. Name it, for example, `job-radar-private`. Choose **Private**. Leave README,
   .gitignore and licence **unticked**. Click **Create repository**.
3. Don't add collaborators.

### 2. Make your config folder the private repository

Your **config folder** is the folder that holds `config.yaml`, `profile.md` and
`rubric.md`. If you've been trying job-radar on your computer, that is the `private/`
folder next to the program (or the folder named in `JOB_RADAR_HOME`). If you haven't
created one yet, run `job-radar init --home ~/job-radar-private` first and use that
folder.

There is no copying: you turn that same folder into the repository, so there is
**one** copy of your config, whether job-radar runs on your computer or on GitHub.

Download the contents of the project's [`template/`](../template/) folder from GitHub
and keep them handy: two workflow files (`job-radar.yml` and `check-no-secrets.yml`
in `.github/workflows/`) and the `scripts/` folder (the two state scripts and a
`git-hooks/pre-commit` safety hook). Then, in a terminal:

```sh
cd /path/to/your/config-folder
printf '.env\n*.bak*\nradar.db*\n.radar-state\n' > .gitignore
cp -R /path/to/template/.github .
cp -R /path/to/template/scripts .
chmod +x scripts/*.sh scripts/git-hooks/pre-commit
git init -b main
git config core.hooksPath scripts/git-hooks
git add -A && git status --short
```

`git config core.hooksPath ...` switches on the safety hook (see
[Guardrails](#guardrails-so-a-secret-cant-be-committed) below). Install `gitleaks`
too if you can (`brew install gitleaks` on a Mac).

Read the `git status` list. It should show your config, profile and rubric, and
**no** `.env`, `.db` or `.bak` files (the `.gitignore` keeps them out). Then:

```sh
git commit -m "My job-radar config"
git remote add origin git@github.com:YOUR-NAME/job-radar-private.git
git push -u origin main
```

Open `.github/workflows/job-radar.yml` and check the lines under `env:`: the program's
repository (`JOB_RADAR_REPO`) and the version series (`JOB_RADAR_SERIES`).

If your config folder isn't `private/` right next to where you run job-radar, tell
job-radar where it is by adding this line to your shell profile (for example
`~/.zshrc`) and opening a new terminal: `export JOB_RADAR_HOME="$HOME/job-radar-private"`.
Keep your `.env` (passwords) inside that folder; the `.gitignore` above keeps it out
of git.

### 3. Add your secrets

In your private repository: **Settings → Secrets and variables → Actions → New
repository secret**. Add these, one at a time, pasting each value from your local
`.env` file:

| Name | What it is |
|---|---|
| `RADAR_EMAIL_ADDRESS` | the job-radar email address ([email-setup.md](email-setup.md)) |
| `RADAR_EMAIL_APP_PASSWORD` | its app password |
| `ANTHROPIC_API_KEY` | only if you use AI scoring or web search |
| `ANTHROPIC_WORKSPACE_ID` | only if your key belongs to a workspace |

GitHub never shows a secret's value again after you save it. Never paste these into a
chat, a config file or a commit.

### 4. Give it your database (or start fresh)

The database remembers which jobs you've seen, so you aren't emailed old jobs again.

**If you already use job-radar on your computer** (recommended, so nothing looks
new), send your existing `radar.db` to the cloud once. From inside your config folder:

```sh
scripts/state-push.sh
```

You should see `Pushed radar.db to the cloud`. A branch called `state` now holds it.

**If this is a brand-new setup**, skip this and tick **start_fresh** when you start the
first run in step 5. Every open job will look new on that first run.

### 5. Run it once by hand

1. Go to the **Actions** tab of your private repository. If GitHub asks, enable workflows.
2. Choose **job-radar** → **Run workflow** (tick **start_fresh** only for a brand-new setup).
3. Open the run. All steps should turn green. A `state` and a `state-previous` branch now exist.

From then on it runs by itself at 08:15 and 16:15 UTC (GitHub can start it a few
minutes late when it's busy).

---

## Guardrails so a secret can't be committed

Your private repository holds personal data and sits next to files that must never be
committed (`.env` with your passwords, and `radar.db`). Four layers protect them:

1. **`.gitignore`** keeps `.env`, the database, backups and the sync marker out of
   `git add`. It can be overridden with `git add -f`, so there is more.
2. **A pre-commit hook** (`scripts/git-hooks/pre-commit`, switched on by the
   `git config core.hooksPath` line above) refuses to commit those files even when
   forced, and runs `gitleaks` over what you're committing to catch an API key or
   password pasted into any file. It is local, so it doesn't protect a clone where you
   haven't switched it on, and `git commit --no-verify` skips it.
3. **A check on GitHub** (`check-no-secrets.yml`) runs on every push and fails, with
   an email from GitHub, if a secret file or the database is tracked. It can't stop the
   push, only tell you at once. If it fires, remove the file, **change the password or
   key**, and treat the old one as leaked.
4. **The cloud run and the state scripts refuse to work** in a public repository, and
   the scripts also refuse to run outside a folder with `config.yaml`, so your database
   can't be pushed to the wrong repository by accident.

GitHub's own automatic secret blocking on push is free only for public repositories;
for private ones it is a paid add-on, so we don't rely on it.

---

## Versions and upgrades

`JOB_RADAR_SERIES: '0.1'` means "the newest 0.1.x". Small fix releases arrive by
themselves. A new minor or major version (0.2, 1.0) can change settings or
behaviour, so it only happens when **you** edit that line, after reading the release
notes. Releases below 1.0.0 are published as *pre-releases*: they work, but aren't
promised to be stable yet.

---

## The two copies of your database

The cloud run keeps its database on the `state` branch of your private repository.
If you also run job-radar on your computer, you have a second, **separate** copy
(`radar.db` in your config folder). They are never merged automatically.

Your config is different: it's one folder that is also your git repository, so your
computer and GitHub always read the same files (edit, `git commit`, `git push`).

> ### ⚠️ Read this before running job-radar on your computer
>
> - **The cloud schedule keeps running** (08:15 and 16:15 UTC). Nothing stops you
>   running job-radar locally as well, but if you do it carelessly the two databases
>   fall out of step. Then job-radar **emails you jobs it already sent**, and the
>   **monthly spend caps** (AI and web search, counted per database) are not shared,
>   so together you can spend more than the cap you set.
> - **Safe local use needs `git` and a terminal**: you pull before, you push after,
>   and you read a message if the push is refused. If that doesn't sound comfortable,
>   **stick to the schedule.** To get a run right now, go to **Actions → job-radar → Run
>   workflow**. You lose nothing by never running it locally.
> - The scripts need a bash shell (macOS, Linux, or Git Bash/WSL on Windows).
> - Don't start a local run or a push within about ten minutes of 08:15 or 16:15 UTC.

### Just looking (safe, no care needed)

`list`, `show`, `report` and `feedback sync` don't search or send email. To see
current data, pull first:

```sh
cd /path/to/your/config-folder
scripts/state-pull.sh
job-radar list --scored
```

### A real local run, step by step

Use this when you want to run job-radar yourself and keep the result (a local `run`,
or votes you give from the terminal with `job-radar feedback`).

1. **Pull** the cloud database to your computer:
   ```sh
   scripts/state-pull.sh
   ```
   It says `Pulled the cloud database`. If it says your local `radar.db` has changes
   never pushed, you have unfinished local work: push it (step 3) or, if you're sure
   you don't need it, run `scripts/state-pull.sh --discard`.
2. **Do your local run** the usual way, for example `job-radar run`.
3. **Push** the result back:
   ```sh
   scripts/state-push.sh
   ```
   It says `Pushed radar.db to the cloud`. The next scheduled run starts from your copy.
4. **If the push says `REFUSED`**, a scheduled run happened while you were working.
   Nothing was overwritten and your local changes were **not** sent. Redo from a fresh
   copy: `scripts/state-pull.sh --discard`, then repeat your local run, then push.
   (Emails your local run already sent can be sent again by the cloud; that's the cost
   of the collision.)

`job-radar run --dry-run` sends no email and saves no postings, but if it uses AI or web
search it still records the **spend** in your local `radar.db`. Push afterwards if you
want the cloud's spend total to include it.

### Safety nets

- `state-push.sh` only overwrites the cloud database if it is **still the version you
  pulled**. Otherwise it refuses.
- The cloud workflow does the same check when it saves. If you pushed during a
  scheduled run, the run's database is kept on a branch called `state-conflict`, the
  run is marked failed, and your push is not erased. Pull `state-conflict`'s
  `radar.db` if you want that run's data, or just run again.
- The branch `state-previous` holds the database as it was before the last save, from
  either side. To roll back, copy its `radar.db` onto `state`.

---

## Running it yourself, without GitHub

GitHub is only a free place to run the schedule. job-radar is one program that keeps its
state in a local `radar.db`, so you can run `job-radar run` by hand, or from your own
scheduler. Nothing else is needed.

```sh
# Linux or macOS: crontab -e, twice a day at 08:15 and 16:15
15 8,16 * * * cd /path/to/your/folder && /path/to/job-radar run >> run.log 2>&1
```

On Windows use Task Scheduler with the same command. Your computer (or server) has to
be on at those times. Keep your `.env` file next to your config, and don't put it, or
the database, in a public place.

---

## Good to know

- **Delays:** GitHub may start a scheduled run late. Alerts can arrive a little after the
  scheduled time.
- **Switching off:** Actions → job-radar → **⋯ → Disable workflow**. Delete the repository
  to remove everything ([security-and-privacy.md](security-and-privacy.md)).
- **Inactivity:** GitHub switches off scheduled workflows after 60 days without
  activity in **public** repositories. A private repository like yours isn't affected.
- **A run failed:** you get at most one problem email a day. Open the run on the
  Actions tab; the log shows which step failed and the error, without job details.
- **Rolling back:** the `state-previous` branch holds the database as it was before the
  last save. To restore it, check it out, copy its `radar.db` into your config folder
  and run `scripts/state-push.sh` (use `scripts/state-pull.sh --discard` first).
- **Why a hand-written release and checks:** [ADR-0039](decisions/0039-build-releases-with-a-hand-written-workflow.md)
  and [ADR-0040](decisions/0040-run-in-the-cloud-verified-binary-state-branch.md).

# Glossary

Plain-English definitions of the terms you'll meet in job-radar's docs. If a
word isn't here and you think it should be, please open an issue.

- **API** — a way for programs to talk to each other. job-radar uses job
  boards' public APIs to ask "what jobs are open right now?".

- **ATS (Applicant Tracking System)** — the software a company uses to post
  jobs and manage applications. Greenhouse, Lever and Ashby are examples.
  job-radar reads jobs from these when a company uses one with a public feed.

- **Board token / slug** — a company's short identifier on a job board, used in
  the web address of its careers page. For example, in
  `boards.greenhouse.io/acme`, the slug is `acme`.

- **CLI (command-line interface)** — a program you run by typing commands in a
  terminal, rather than clicking buttons. `job-radar` is a CLI.

- **Config / configuration** — your settings: which companies to watch, your
  location rules, and so on. They live in `config.yaml`.

- **cron / schedule** — a way to run something automatically at set times (for
  job-radar, twice a day by default).

- **Digest** — a once-a-day summary email of everything worth a look.

- **Environment variable** — a setting stored outside your files, used to hold
  secrets like passwords and API keys so they never end up in a file you might
  share. Often kept in a `.env` file locally.

- **GitHub Actions** — GitHub's free service for running tasks on a schedule on
  their servers, so job-radar can run without your computer being on.

- **LLM (large language model) / AI scorer** — the optional, paid scoring mode
  that reads a job ad like a person would. Needs your own API key.

- **Posting** — one job ad, after job-radar has tidied it into a standard shape.

- **Pre-filter** — a quick, free check that drops obvious non-matches (e.g.
  internships) before any scoring. When in doubt, it keeps the role.

- **Rubric** — your written rules for what makes a role a good or bad fit.
  job-radar reads `rubric.md` to score roles your way.

- **Rules scorer** — the free, keyword-based scoring mode. The default. No API
  key, no cost.

- **Score** — a 1–5 rating of how well a role fits you, with a short "why" and
  the biggest "gap".

- **Secret** — a password, token or API key. Secrets go in environment
  variables, never in config files or git.

- **Source** — a place job-radar gets jobs from: a job board, or (later) your
  job-alert emails.

- **Track** — one of the kinds of role you're open to (for example: manager,
  hands-on lead, program manager). You define your own.

- **YAML** — the simple text format `config.yaml` is written in. Indentation
  matters, and it must be spaces, not tabs.

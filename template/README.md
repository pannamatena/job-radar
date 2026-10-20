# job-radar private config repository (template)

Your config folder (the one with `config.yaml`, `profile.md` and `rubric.md`) becomes
your **private** repository. Copy these into it:

- `.github/workflows/job-radar.yml`: the twice-a-day cloud run.
- `.github/workflows/check-no-secrets.yml`: fails if a secret file or your database is committed.
- `scripts/state-pull.sh` and `scripts/state-push.sh`: for the rare time you run
  job-radar on your computer too and want to keep the database in step with the cloud.
- `scripts/git-hooks/pre-commit`: stops you committing `.env`, the database or a pasted key.
  Switch it on once with `git config core.hooksPath scripts/git-hooks`.

The full step-by-step guide is [docs/run-automatically.md](../docs/run-automatically.md).

Check the `env:` lines at the top of the workflow:

- `JOB_RADAR_REPO`: the repository the released program comes from.
- `JOB_RADAR_SERIES`: the version series to follow, such as `0.1` for the newest `0.1.x`.

Never commit `.env`, `radar.db`, `.radar-state` or backup files. Put passwords and keys in
**Settings → Secrets and variables → Actions**.

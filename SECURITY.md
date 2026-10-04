# Security policy

## Reporting a vulnerability

If you find a security problem in job-radar itself, **please don't open a public
issue.** Instead, report it privately so it can be fixed before it's disclosed:

- Use GitHub's **private vulnerability reporting** on this repository
  (Security → Report a vulnerability), or
- email the maintainer (see the GitHub profile for contact).

Please include what you found, how to reproduce it, and the impact as you see
it. You'll get an acknowledgement, and credit if you'd like it.

This is a small open-source project maintained in spare time, so please allow
reasonable time for a fix before any public disclosure.

## If a secret or personal data gets committed or pushed

job-radar is designed to keep your personal data and secrets out of git
(`.gitignore`, a pre-commit hook, and a CI check — see
[ADR-0018](docs/decisions/0018-catch-personal-data-and-secrets-before.md)). If
something slips through anyway:

1. **Rotate the secret immediately.** Assume anything pushed to a public repo
   has been copied, even if you delete it seconds later.
   - Email app password: your email account → Security → App passwords → remove
     and create a new one.
   - AI API key: revoke it in your provider's dashboard and issue a new one.
2. **Remove the file from git history, not just the latest commit.** Deleting a
   file in a new commit leaves it in history. Use a history-rewriting tool
   (e.g. `git filter-repo`) and force-push, or delete the repository if it's
   easier and it's early days.
3. **Update the secret** in your `.env` or GitHub Actions secrets.
4. Turn on **secret scanning and push protection** for the repo
   (Settings → Code security) so it's caught next time before the push lands.

## Scope

job-radar runs entirely on your own machine or your own GitHub account. There is
no job-radar server or hosted service. The security considerations for *using*
the tool (email app passwords, API keys, prompt injection in job ads, being a
polite client of job boards) are explained for users in
[docs/security-and-privacy.md](docs/security-and-privacy.md).

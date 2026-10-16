# third_party: email libraries, copied in unchanged

These three libraries read your replies from the job-radar mailbox (see
[ADR-0034](../docs/decisions/0034-read-email-replies-over-imap-read-only.md)).
They are copied here, unchanged, instead of being downloaded at build time, so
that any upgrade shows up as a diff in a pull request that a person can read.

| Folder | Upstream | Version | Licence |
|---|---|---|---|
| `go-imap/` | github.com/emersion/go-imap/v2 | v2.0.0-beta.8 | MIT |
| `go-message/` | github.com/emersion/go-message | v0.18.2 | MIT |
| `go-sasl/` | github.com/emersion/go-sasl | v0.0.0-20241020182733-b788ff22d5a6 | MIT |

`go.mod` points at these folders with `replace` lines. Nothing is edited.

## Check that the copy is unchanged

Run this from the repo root. It downloads the upstream version and compares it
with the copy; no output means identical.

```sh
for m in go-imap/v2@v2.0.0-beta.8:go-imap go-message@v0.18.2:go-message \
         go-sasl@v0.0.0-20241020182733-b788ff22d5a6:go-sasl; do
  src="$(go env GOMODCACHE)/github.com/emersion/${m%%:*}"
  [ -d "$src" ] || (cd "$(mktemp -d)" && go mod init x >/dev/null 2>&1 && go mod download "github.com/emersion/${m%%@*}@${m#*@}" 2>/dev/null)
  diff -r "$src" "third_party/${m##*:}" && echo "${m##*:}: identical"
done
```

## Upgrade (by hand, never automatic)

1. Pick the new version and read the upstream changes since the current one.
2. Replace the folder's contents with the new version, unchanged.
3. Update the table above and the `go.mod` version lines, run the tests.
4. In the pull request, read the diff of the copied code before merging.
Wait at least a week after an upstream release before upgrading, so a bad
release has time to be noticed and pulled.

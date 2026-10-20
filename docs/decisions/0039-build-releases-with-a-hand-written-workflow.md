## ADR-0039: Build releases with a hand-written workflow, not GoReleaser

**Status:** Accepted

**Context:** Users run job-radar from a released program, downloaded by the scheduled workflow in their private repo ([ADR-0015](0015-two-repos-per-user-public-code.md)). That program runs with their email password and, if they use AI scoring, their API key. So whatever builds the release is part of what they have to trust. A release needs binaries for several systems, a checksum file, and proof of where the binaries came from.

**Options considered:**
1. **A hand-written GitHub workflow (chosen).** About 60 lines: test, build each platform with `go build`, write `SHA256SUMS`, sign a build attestation with GitHub's own `actions/attest`, publish with the `gh` command that is already on the runner.
2. **GoReleaser.** A separate tool, configured in a `.goreleaser.yaml`, that does the building, archives, checksums, changelog and optionally package managers (Homebrew) in one go.

**Decision:** Option 1.

**Why:** it is the more secure choice. The release workflow uses only GitHub's own actions (each pinned to a full commit SHA, [ADR-0018](0018-catch-personal-data-and-secrets-before.md)) and `go build`, so there is less third-party code running with permission to publish and to sign. It is also short enough to read in one sitting, so every step can be checked and explained.

**Trade-offs:**
- We maintain the build matrix and the release steps ourselves.
- No archives, no changelog tooling, no Homebrew formula out of the box. Each would be added by hand if wanted.
- Releases below 1.0.0 are marked as pre-releases by the workflow, and only tags on `main` are built.

**Revisit if:** we want Homebrew, installers or many more platforms. Switching to GoReleaser is easy: the program's build is a plain `go build` with one `-ldflags` setting, and the users' workflow only looks at the release's file names, the checksum file and the attestation. A new tool can produce the same three things, and the old and new workflows can be swapped in one release.

**In one sentence:** "The thing that builds the program is also something users must trust, so I kept it small and made of GitHub's own parts, and I can swap in a bigger tool later without users noticing."

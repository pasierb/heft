---
name: release-heft
description: Prepare and publish a heft release by inferring the next SemVer version from user-facing changes, tagging main, and monitoring GitHub Actions. Use when asked to release heft or determine its next release version.
---

# Release heft

Publish releases only from the `main` commit currently on `origin/main`. GitHub
Actions builds and creates the release; do not build or upload release assets
locally.

## Preflight

1. Run `gh auth status`, `git fetch origin main --tags`, and `git status --short`.
2. Stop if authentication fails, the checkout is not on `main`, the worktree is
   dirty, or `HEAD` differs from `origin/main`.
3. Run `go test ./...`; stop on failure.
4. Find the newest stable SemVer tag reachable from `origin/main`. Ignore
   prerelease tags when selecting the comparison baseline.
5. Inspect both the commits and full diff from that tag to `origin/main`. With
   no stable tag, inspect the repository as the initial release.

## Choose the version

Infer the bump from the highest-impact change, not from commit-message prefixes:

- Breaking command, flag, configuration, or other user-facing behavior: bump
  major, except below 1.0 bump minor.
- Backward-compatible user-facing capability: bump minor.
- Compatible fixes, dependencies, documentation, tests, or release tooling:
  bump patch.
- With no stable tag, propose `v0.1.0`.

Default to a stable version. When the user explicitly requests a prerelease,
append `-rc.1`, or increment the highest existing `-rc.N` tag for the proposed
core version. Release tags must match
`vMAJOR.MINOR.PATCH` or `vMAJOR.MINOR.PATCH-PRERELEASE`; build metadata is not
supported.

Present the baseline, proposed version, target commit, categorized changes, and
bump rationale. The user may confirm the proposal or supply a corrected valid
version. Stop when there are no commits since the baseline release.

## Publish

Immediately before publishing, ask for explicit confirmation of the version and
commit. After confirmation:

1. Confirm the chosen tag does not exist locally or on `origin`.
2. Run `git tag -a VERSION -m "Release VERSION"`.
3. Run `git push origin VERSION`; push no branches or other tags.
4. Find and watch the tag-triggered Release workflow with `gh run list` and
   `gh run watch --exit-status`.
5. Use `gh release view VERSION` to verify the release contains Linux `amd64`
   and `arm64` archives plus `SHA256SUMS`.

On failure, report the exact local tag, remote tag, workflow, and release state.
Do not delete or replace tags, force-push, recreate a release, or retry a
mutation without new user authorization.

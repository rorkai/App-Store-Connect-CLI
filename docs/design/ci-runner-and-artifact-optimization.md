# CI Runner and Artifact Optimization

## Current behavior

Every non-Wall pull request runs platform-independent formatting, documentation,
lint, and unit-test work on macOS. One macOS build job then cross-compiles five
binaries concurrently and uploads roughly 105 MB of development artifacts.
The main-branch workflow repeats the build and upload. No workflow downloads
either artifact; release binaries are built independently by `release.yml`.

## Proposed behavior

- Run platform-independent formatting, documentation, lint, Wall validation,
  and unit-test shards on Ubuntu.
- Build macOS, Linux, and Windows binaries on their native hosted runners.
- Run the Darwin-only screenshot tests on the macOS build runner.
- Preserve a stable aggregate `build` job for required-check compatibility.
- Keep cross-platform compilation on pull requests and `main`.
- Remove pull-request and main-branch artifact uploads and development
  checksums. Official artifact publication remains owned by `release.yml`.

This changes CI execution only. CLI commands, flags, output, exit codes,
authentication, and API behavior remain unchanged.

## Alternatives

- A larger runner would require paid compute even for this public repository.
- Parallel build steps on one macOS runner retain CPU and memory contention.
- Removing cross-platform compilation would be faster but would weaken the PR
  compatibility gate.

## Verification

- Run the workflow contract test before and after the change.
- Run formatting, documentation, lint, and the full Go test suite locally.
- Let the pull request exercise the native Linux, macOS, and Windows runners.
- Compare total and per-job duration with recent successful PR runs.

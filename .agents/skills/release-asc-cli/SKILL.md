---
name: release-asc-cli
description: Publish and verify a new release of the App-Store-Connect-CLI repository. Use only when the user explicitly asks to release, tag, or publish a specific ASC CLI version, including the full GitHub release, artifact, Homebrew, WinGet, cleanup, and release-announcement workflow.
---

# Release the ASC CLI

Treat a release request as an end-to-end publishing operation, not merely a tag push.

## Prove the release target

1. Resolve the requested plain semantic version without a `v` prefix.
2. Fetch `origin`, verify the tag does not already exist locally or remotely, and inspect the release-worthy delta since the previous tag.
3. Query open PRs and confirm the intended changes are already merged.
4. Create a clean detached worktree from the exact current `origin/main` commit. Do not release from a dirty user checkout or an unmerged branch.
5. Inspect `.github/workflows/release.yml` and current repository guidance instead of assuming an older release procedure still applies.

## Run the release gate

Run and record:

```bash
make format
make check-docs
make check-wall-of-apps
make lint
ASC_BYPASS_KEYCHAIN=1 make test
make build
./asc version
```

If formatting changes files, inspect and commit only intentional release-related changes before continuing. Stop on unexplained failures; do not tag around a broken gate.

## Publish

1. Reconfirm the worktree HEAD equals the intended `origin/main` commit.
2. Create an annotated tag using the exact requested version and push that tag explicitly.
3. Locate the tag-triggered GitHub Actions run and watch it through completion.
4. Do not retry by silently moving or recreating a published tag. Diagnose failures and preserve the immutable release history.

## Verify consumer-visible state

Do not declare success from CI alone. Verify:

- The GitHub release is published, not draft or prerelease unless requested.
- Expected macOS, Linux, Windows, and checksum assets exist.
- A downloaded macOS binary reports the requested version and passes `codesign --verify`.
- Downloaded artifact hashes match the published checksum file.
- The Homebrew formula points to the new version and hashes.
- The expected WinGet submission or PR exists and references the new version.
- Any notarization step required by the current workflow succeeded.

## Draft the announcement

Read [references/release-announcement.md](references/release-announcement.md). Create the Typefully draft only after the release is visibly published. Never schedule or publish the post unless the user explicitly asks.

## Clean up and hand off

Remove only the temporary release worktree created by this run. Report the released commit and tag, workflow run, release URL, artifact verification, Homebrew state, WinGet state, Typefully review URL or connector blocker, commands run, and any residual state.

## Automation boundary

Do not schedule unattended releases. After an explicitly initiated tag push, a thread heartbeat may reuse this skill to watch the release run and finish downstream verification.

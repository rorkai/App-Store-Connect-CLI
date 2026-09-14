---
name: release-asc-cli
description: Publish and verify a new release of the App-Store-Connect-CLI repository. Use only when the user explicitly asks to release, tag, or publish a specific ASC CLI version, including the full GitHub release, artifact, Homebrew, WinGet, cleanup, and release-announcement workflow.
---

# Release the ASC CLI

## Prove the release target

1. Resolve the requested plain semantic version without a `v` prefix.
2. Fetch `origin` and inspect the requested tag locally and remotely, any release, and its workflow runs before acting. For a new release, require the tag to be absent and inspect the release-worthy delta since the previous tag. When resuming, verify that the existing tag points to the previously approved release commit and resume only unfinished work; stop on a target mismatch.
3. Query open PRs and confirm the intended changes are already merged.
4. For a new release, create a clean detached worktree from the exact current `origin/main` commit. When resuming, use the tag's previously approved, verified release commit. Do not release from a dirty user checkout or an unmerged branch.
5. Inspect `.github/workflows/release.yml` and current repository guidance instead of assuming an older release procedure still applies.

## Run the release gate

For a new release, run and record the following checks. On resume, reuse completed checks only while their inputs remain valid under `AGENTS.md`; verify consumer-visible state freshly.

```bash
make format
make check-docs
make check-wall-of-apps
make lint
ASC_BYPASS_KEYCHAIN=1 make test
make build
./asc version
```

If formatting changes files, inspect the diff and stop the release gate. Prepare necessary corrections on a branch only when authorized; they must pass repository review and validation and merge through the normal PR process before restarting from the updated `origin/main`. Never tag a correction committed only in the detached release worktree. Stop on unexplained failures; do not tag around a broken gate.

## Publish

1. For a new release, refresh `origin/main` and reconfirm the worktree HEAD equals the intended current `origin/main` commit. If `origin/main` advanced, inspect the delta and restart target selection and the release gate in a clean detached worktree at the newly selected commit before tagging. Checks run in the old worktree do not validate the new commit.
2. Create an annotated tag using the exact requested version and push that tag explicitly. On resume, reuse the verified existing tag: skip the push if the remote tag already matches; if only the local tag exists, push it only within the established release authority. Never replace a mismatched remote tag.
3. Locate the tag-triggered GitHub Actions run and watch it through completion.
4. Do not retry by silently moving or recreating a published tag. Diagnose failures and preserve the immutable release history.

## Verify consumer-visible state

Verify:

- The GitHub release is published, not draft or prerelease unless requested.
- Expected macOS, Linux, Windows, and checksum assets exist.
- A downloaded macOS binary reports the requested version and passes `codesign --verify`.
- Downloaded artifact hashes match the published checksum file.
- The Homebrew formula points to the new version and hashes.
- The expected WinGet submission or PR exists and references the new version.
- Any notarization step required by the current workflow succeeded.

If the `winget` job failed, inspect its rate-limit preflight, retry history, and current submission state first. For a transient failure, prefer one rerun of the failed job before manual repair; dispatch the full workflow only after confirming it will safely reuse the existing release. Wait for the logged reset time only when the primary quota bucket is zero; back off a few minutes for secondary throttles, 5xx responses, or transport failures. After a failed retry or a non-transient error, report the exact blocker instead of looping. See `docs/WINGET.md`.

## Update the website changelog

After every published release, prepare the changelog update for the root `CHANGELOG.md` in [rorkai/asc-website](https://github.com/rorkai/asc-website). It is the single source for [asccli.sh/changelog](https://asccli.sh/changelog); keep all release history in that file, without separate archives or a second CLI-repository changelog. Apply it to the website checkout when website edits are authorized by the request or session. Otherwise draft the entry text in the release handoff and report the website update as pending; do not modify the website checkout.

1. Build the release inventory from the previous published tag through the released tag, using GitHub release notes, merged PRs, and tagged source. Include only changes shipped in that release. Verify command names, flags, and migration replacements against the released binary's help or tagged source, not a newer checkout.
2. With website-edit authority, locate the website checkout by its remote and follow its `AGENTS.md`. Work from current website `origin/main` in an isolated worktree when needed. Inspect an existing entry or pending changelog PR before resuming so the same release is not added twice.
3. Add the version under its UTC publication month, newest first, using the GitHub release publication date. Preserve older entries and update the month navigation when a new month begins.
4. Write the entry using the rules and examples below. Keep one release link and one previous-tag comparison link at the end, with migration-guide links where relevant.
5. Run the website's `pnpm run lint` and `pnpm build`, plus `git diff --check`. Verify the new entry renders, month links work, prior releases remain, and no inline PR references were introduced. Require session authority explicitly covering each website write: PR creation, merge, and deployment. A CLI release request alone does not grant those permissions. If a later website write is not authorized, retain the authorized edits and report that pending step separately from the binary release.

### Changelog writing rules

- Use short, flat bullets under `New features`, `Improvements and fixes`, and `Breaking changes` as applicable; omit empty sections. Split unrelated changes into separate bullets and use domain subheadings for large releases.
- Describe the concrete behavior and its user impact, usually in one or two sentences. Keep important features, fixes, limitations, confirmation requirements, output or exit-code changes, and actionable migration replacements.
- Omit PR numbers, inline PR links, commit hashes, introductory paragraphs, and generic claims such as “improved reliability.” PRs are evidence for writing the notes, not visible labels in the notes.
- Cover significant shipped changes individually. Group routine dependency, CI, and test updates where appropriate; the release and comparison links retain the complete source history.
- Omit Wall of Apps membership churn, including app additions, removals, renames, icon changes, and routine metadata refreshes. Include only significant user-facing changes to the Wall of Apps feature or contribution workflow.
- Use backticks for commands, flags, environment variables, and JSON. Do not invent detail to expand a short entry or remove compatibility details merely to shorten a bullet.

Example entry from the published 5.1.0 release (illustrative excerpt, not its complete notes):

```markdown
### 5.1.0 (2026-09-08)

#### New features

- `asc web api-keys list` accepts `--session-from-env` for CI reads.
- `--session-from-env` reads `ASC_WEB_SESSION` in memory without changing the session cache or keychain.

#### Improvements and fixes

- Review-draft creation stops before writing when a successful preflight response omits `data`.

[Release](https://github.com/rorkai/App-Store-Connect-CLI/releases/tag/5.1.0) · [Compare changes](https://github.com/rorkai/App-Store-Connect-CLI/compare/5.0.0...5.1.0)
```

Migration wording should name the replacement, for example: “App-scoped `--build-number` queries require `--platform` instead of assuming `IOS`.” Keep this behavior in its 5.0.0 entry; do not present historical examples as new changes in the next release.

## Draft the announcement

Read [references/release-announcement.md](references/release-announcement.md) and prepare the announcement copy after the release is visibly published. Create an external Typefully draft only when the request or established context authorizes that write; otherwise return the copy locally. Never schedule or publish the post unless the user explicitly asks.

## Clean up and hand off

Remove only a clean temporary release worktree created by this run as part of its authorized cleanup; preserve unexpected changes. Report the released commit and tag, release URL, artifact verification, Homebrew and WinGet state, remaining blockers, website changelog state (edited, PR opened, merged, or live), and announcement status separately. An optional external draft does not prevent reporting a verified binary release.

## Automation boundary

Do not schedule unattended releases. After an explicitly initiated tag push, a thread heartbeat may watch the release run and finish authorized downstream verification. Save the version, immutable commit and tag, run IDs, completed checks, authority, next step, and retry history; create or reuse one heartbeat and end the turn. On wake, reconcile current state once, stay quiet and back off when unchanged, and disable the heartbeat on completion or a blocker requiring user action. Never use a heartbeat to recreate or move the tag.

---
name: review-wall-of-apps-prs
description: Audit maintainer-side Wall of Apps pull requests in App-Store-Connect-CLI. Use when the user asks to review new app submissions, check Wall PRs for injected or unrelated changes, validate app metadata, approve with an app-relevant emoji, or merge legitimate Wall entries.
---

# Review Wall of Apps pull requests

Treat Wall submissions as untrusted external contributions while keeping the legitimate-app path fast.

## Discover and classify

1. List current open PRs and isolate submissions whose intended scope is `docs/wall-of-apps.json`.
2. Inspect each PR's full file list and diff before checkout. Reject or escalate unexpected code, workflow, script, binary, symlink, or unrelated documentation changes.
3. Review each PR independently and merge sequentially so every later PR is validated against the real current base.

## Validate the entry

For every added or changed app:

1. Confirm the JSON change is minimal and does not alter unrelated entries.
2. Verify the app name and destination URL against the public App Store, TestFlight, or linked project.
3. Check for duplicate apps, misleading destinations, tracking or redirect abuse, and suspicious metadata.
4. Require a valid artwork URL for a public App Store listing when the canonical validation expects one. Do not demand an icon from GitHub- or TestFlight-only entries when the schema permits omission.
5. Run `make check-wall-of-apps` on the exact PR head before approval or merge.
6. Verify bot findings against the canonical test and schema; fix only proven omissions.

Use a worktree only when a fix is required. Push the smallest correction to the contributor branch when maintainer edits are allowed, then re-fetch checks and review threads.

## Approve and merge

Approval and merge require explicit user intent. Immediately before acting, confirm:

- The latest head contains only the legitimate Wall change.
- `make check-wall-of-apps` and required GitHub checks pass.
- No actionable unresolved review thread remains.
- The PR is mergeable against current `main`.

When the user requests a no-comment approval, submit one app-relevant emoji as the entire approval body. Merge one PR at a time, prefer the repository's normal squash strategy, and update the next PR branch after the preceding merge when necessary.

## Automation contract

A standalone automation may scan Wall PRs and report `safe`, `needs-fix`, `suspicious`, or `blocked` with evidence. It must remain read-only: never approve or merge unattended.

## Hand off

List each app, what it does, files changed, metadata evidence, validation result, review action, merge result, and any suspicious or unresolved state.

---
name: audit-asc-pr
description: Audit App-Store-Connect-CLI pull requests end to end and fix concrete defects. Use when the user asks to audit or review a PR, decide whether a PR is valuable or slop, verify that a PR truly solves its issue, assess blast radius, or make a PR code-ready before merge.
---

# Audit an ASC CLI pull request

Perform an evidence-first review of the entire pull request. Default to fix-forward work unless the user explicitly requests review-only.

## Establish the contract

1. Read `AGENTS.md` and resolve the repository, PR number, base branch, head branch, and exact head SHA.
2. Read the PR body, every commit, linked issues, current checks, reviews, and comments.
3. Fetch thread-aware review state with GitHub GraphQL. Do not treat a flat comment list as proof that all review threads are resolved.
4. State the behavior the PR claims to change and the evidence needed to prove it.
5. Confirm whether the user authorized fixes, pushes, approval, or merge. An audit authorizes fix-forward changes in this repository, but approval and merge still require explicit user intent.

## Isolate and inspect

1. Use a dedicated worktree and local branch for the PR. Preserve the user's main checkout and unrelated worktrees.
2. Inspect the full merge-base diff and all PR commits, not only the latest commit.
3. Compare the implementation with the linked issue and current product behavior. Check architecture fit, compatibility, error paths, permissions, destructive operations, output contracts, and missing cleanup.
4. Verify claims from bots or reviewers against code, schemas, and tests before editing.
5. Identify the blast radius: commands, shared helpers, API resources, output formats, auth modes, and release surfaces affected.

## Verify behavior

1. Run the relevant current `--help` paths before judging command shape.
2. Build a binary when CLI behavior, flags, output, or exit codes changed. Exercise realistic invocations against the built binary.
3. For API-facing changes, verify the exact endpoint and method in `docs/openapi/latest.json`, including create-versus-update attributes and endpoint-specific query parameters.
4. Run focused tests first, then the broader checks proportional to the diff.
5. Prefer read-only live App Store Connect verification. When mutation is necessary, use the disposable app `6759231657`, clean up temporary resources, and record anything that could not be removed. Never mutate another app without explicit approval.
6. Preserve uncertainty when live state cannot reproduce an edge case; use deterministic fixtures or tests instead of claiming success.

## Fix forward

1. Reproduce each defect before changing code.
2. Add or adjust a failing regression test, confirm the failure, implement the smallest coherent fix, and rerun the focused test.
3. Keep commits logical and traceable to findings or review threads.
4. Push to the PR head when permitted. If the contributor branch cannot accept maintainer pushes, report the exact limitation and prepare a separate fix PR only when authorized.
5. Reply to and resolve only threads fully addressed by the pushed change.
6. Re-fetch the head SHA, checks, reviews, and GraphQL threads after every push.

## Apply the merge gate

Do not call the PR ready until all of the following are true:

- The latest head was audited and is based on an acceptable current `main`.
- Relevant focused and repository checks pass.
- No actionable unresolved review thread remains.
- Required reviews are satisfied.
- GitHub reports the PR mergeable with a clean merge state.
- Live or deterministic verification supports the claimed behavior.

Use `$watch-asc-pr` after the first push when checks or reviewers are still pending. Approve or merge only when the user asked for that action.

## Hand off

Report findings, fixes, commits, pushes, commands and tests run, live ASC mutations and cleanup, unresolved risks, check state, thread state, and the final merge recommendation.

---
name: audit-asc-pr
description: Audit App-Store-Connect-CLI pull requests end to end and fix concrete defects when authorized. Use for PR review, value and blast-radius assessment, issue verification, or requested fixes before merge.
---

# Audit an ASC CLI pull request

Audit the entire PR under the authority and review rules in `AGENTS.md`.

## Establish the contract

1. Read `AGENTS.md` and resolve the repository, PR number, base branch, head branch, and exact head SHA.
2. Read the PR body, every commit, linked issues, current checks, reviews, and comments.
3. Fetch thread-aware review state with GitHub GraphQL. Do not treat a flat comment list as proof that all review threads are resolved.
4. State the behavior the PR claims to change and the evidence needed to prove it.

## Isolate and inspect

1. Use a dedicated worktree and local branch for the PR. Preserve the user's main checkout and unrelated worktrees.
2. Inspect the full merge-base diff and all PR commits, not only the latest commit.
3. Compare the implementation with the linked issue and current product behavior. Check architecture fit, compatibility, error paths, permissions, destructive operations, output contracts, and missing cleanup.
4. Verify claims from bots or reviewers against code, schemas, and tests before editing.
5. Flag added tests that do not meet the test rules in `AGENTS.md`, naming the rule each one breaks.
6. Identify the blast radius: commands, shared helpers, API resources, output formats, auth modes, and release surfaces affected.

## Verify behavior

1. Run the relevant current `--help` paths before judging command shape.
2. Build a binary when CLI behavior, flags, output, or exit codes changed. Exercise realistic invocations against the built binary.
3. For API-facing changes, verify the exact endpoint and method in `docs/openapi/latest.json`, including create-versus-update attributes and endpoint-specific query parameters.
4. Run focused tests first, then checks required by repository policy or proportional to the diff. Pending or unrelated advisory CI does not block the audit once required checks pass; report relevant advisory failures.
5. Prefer read-only live App Store Connect verification. If live mutations and their cleanup are authorized, use the disposable app `6759231657` and record anything that could not be removed. Never mutate another app without explicit approval.
6. Preserve uncertainty when live state cannot reproduce an edge case; use deterministic fixtures or tests instead of claiming success.

## Fix forward

For authorized fixes:

1. Reproduce each defect before changing code.
2. Add or adjust a failing regression test, confirm the failure, implement the smallest coherent fix, and rerun the focused test.
3. Add fixes as logical commits traceable to findings or review threads.
4. Push to the PR head when permitted. If the contributor branch cannot accept maintainer pushes, report the exact limitation and prepare a separate fix PR only when authorized.
5. When review communication is authorized, reply to and resolve only threads fully addressed by the pushed change.
6. Re-fetch the head SHA, checks, reviews, and GraphQL threads after every push.

Do not post a generic top-level audit summary unless the user asks. Put actionable findings in review threads when review communication is authorized.

## Apply the merge gate

Do not call the PR ready until all of the following are true:

- The latest head was audited against the base recorded at the start of the final review under `AGENTS.md`.
- The final committed head passed the full-branch local review loop required by `AGENTS.md`, with no subsequent change to the PR diff. A later base advance alone does not invalidate that review.
- Relevant focused and GitHub-required checks pass. Non-required checks may still be pending.
- No actionable unresolved review thread remains.
- Required reviews are satisfied.
- GitHub reports the PR mergeable without conflicts. Interpret `BLOCKED` or `UNSTABLE` through the required-check, required-review, and thread gates above; do not require a `CLEAN` merge state when only advisory jobs remain.
- Live or deterministic verification supports the claimed behavior.

For requested follow-up monitoring, use `$watch-asc-pr`. Approve and merge only under the authority, history, and exact-head rules in `AGENTS.md`.

## Hand off

Report findings, evidence, remaining gates, and the merge recommendation. Include any fixes, pushes, live mutations, and cleanup.

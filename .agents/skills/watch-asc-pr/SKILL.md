---
name: watch-asc-pr
description: Recheck an in-progress App-Store-Connect-CLI pull request for new review feedback, CI results, head changes, and merge readiness. Use when the user says to check for new PR comments, continue polling, fix and loop, babysit a PR, or keep working until the PR is clean.
---

# Watch an ASC CLI pull request

Preserve the PR context and authority under `AGENTS.md`: status reads once; watch repeats until a terminal state; fix-and-watch also applies authorized fixes. Watching alone grants no edit, commit, push, or reply authority. A one-time check ends after reporting its snapshot; do not start a full readiness audit or heartbeat unless requested or already authorized.

## Recheck current state

1. Resolve the exact PR and compare its current head SHA with the last audited or pushed SHA.
2. Fetch checks, reviews, top-level comments, and GraphQL review threads in parallel where possible. Separate required checks from advisory jobs.
3. Separate new actionable feedback from resolved, outdated, informational, duplicate, or bot-noise comments.
4. If the head changed outside this workflow, inspect the new diff before relying on prior conclusions.
5. Refresh GitHub mergeability. A newer `main` alone does not invalidate prior local review or validation; follow the branch-update rules in `AGENTS.md`.

## Address actionable feedback

Apply fixes and external writes only when authorized. Otherwise report the verified finding and proposed fix; if it prevents progress, return `blocked` with the exact missing authority.

1. Verify new claims against code, API schemas, and existing behavior.
2. Reproduce a valid defect and add or update a focused test before changing behavior.
3. Implement the smallest coherent fix, run the affected checks, and commit when authorized. Complete the final full-branch review loop in `AGENTS.md` on the final committed head, then push when authorized.
4. When review communication is authorized, reply to and resolve only the threads fully addressed by that push.
5. Re-fetch the PR after pushing and confirm the live head, checks, and thread state.
6. Continue from the fresh head while required checks or reviews remain pending; apply new authorized fixes in additive commits.

## Return one state

- `observed`: a one-time check completed; report the requested facts and mark readiness unverified unless separately established. An actionable finding does not make a completed read-only check blocked.
- `changed`: pushed a fix; include commit and validation.
- `pending`: required checks, required reviews, or actionable reviews are still running; identify exactly what remains. This is an intermediate state during a user-requested loop, not the final handoff.
- `clean`: the final full-branch local review in `AGENTS.md` is clear for the current head and recorded review base, required checks pass, required reviews are satisfied, the latest head is mergeable, and no actionable unresolved thread remains. A later base advance alone does not invalidate that review. Report advisory jobs without treating them as blockers.
- `blocked`: user input, permissions, an external outage, or an unsafe product decision prevents progress.

For an authorized merge, reapply `$audit-asc-pr`'s complete gate immediately before merging and follow the history rules in `AGENTS.md`.

## Automation contract

When only an expected external state change remains in an authorized watch, save a checkpoint with the objective, PR, head and base SHAs, authority, validation results and inputs, unresolved feedback, last checked time, next step, stop condition, and retry history. Create or reuse one thread heartbeat, verify it, and end the turn. If scheduling is unavailable, report the pending state and checkpoint without implying that monitoring continues.

On each wake, inspect decisive state once. Reuse still-valid evidence, stay quiet when unchanged, and back off the next check. Resume work on completion, failure, or a stall. After an uncertain write, reconcile remote state before a bounded retry. Disable the heartbeat when the PR is clean, blocked, merged, closed, superseded, or awaiting a material user decision. Advisory jobs may remain pending after the clean gate passes. Do not create an unattended auto-merge loop.

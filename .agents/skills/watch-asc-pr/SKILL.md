---
name: watch-asc-pr
description: Recheck an in-progress App-Store-Connect-CLI pull request for new review feedback, CI results, head changes, and merge readiness. Use when the user says to check for new PR comments, continue polling, fix and loop, babysit a PR, or keep working until the PR is clean.
---

# Watch an ASC CLI pull request

Run one idempotent follow-up pass. Preserve the existing PR context instead of restarting the full audit.

## Recheck current state

1. Resolve the exact PR and compare its current head SHA with the last audited or pushed SHA.
2. Fetch checks, reviews, top-level comments, and GraphQL review threads.
3. Separate new actionable feedback from resolved, outdated, informational, duplicate, or bot-noise comments.
4. If the head changed outside this workflow, inspect the new diff before relying on prior conclusions.

## Address actionable feedback

1. Verify every new claim against the codebase, API schema, and existing behavior. Do not follow automated feedback blindly.
2. Reproduce a valid defect and add or update a focused test before changing behavior.
3. Implement the smallest coherent fix, run the focused check, commit, and push.
4. Reply to and resolve only the threads fully addressed by that push.
5. Re-fetch the PR after pushing and confirm the live head, checks, and thread state.

## Return one state

- `changed`: pushed a fix; include commit and validation.
- `pending`: checks or reviews are still running; identify exactly what remains.
- `clean`: latest head is green, mergeable, and has no actionable unresolved threads.
- `blocked`: user input, permissions, an external outage, or an unsafe product decision prevents progress.

Do not approve or merge unless the user explicitly requested it. If merge was requested, reapply the complete merge gate from `$audit-asc-pr` immediately before merging.

## Automation contract

Use this skill from a thread heartbeat when the same PR conversation should continue every few minutes. Each wake-up must run one pass, report only changes or blockers, and stop the loop when the PR is clean, merged, closed, superseded, or awaiting a material user decision. Do not create an unattended auto-merge loop.

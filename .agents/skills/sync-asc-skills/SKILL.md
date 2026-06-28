---
name: sync-asc-skills
description: Audit and update rorkai/app-store-connect-cli-skills against the current ASC CLI surface. Use when the user asks whether ASC agent skills need updates, requests a post-release skills sync, or wants command, flag, output, auth, or workflow examples checked for drift.
---

# Synchronize ASC workflow skills

Compare the external workflow-skill repository with live CLI behavior and make only proven, minimal corrections.

## Establish both sources

1. Resolve the current App-Store-Connect-CLI source commit and latest released version relevant to the request.
2. Resolve a clean checkout of `rorkai/app-store-connect-cli-skills` and read its repository guidance.
3. Inventory every skill and identify which commands, flags, environment variables, outputs, or workflows it claims to use.
4. Run the current CLI's `--help` at each relevant command path. Use `asc search`, `asc schema`, or `asc capabilities` only when their own current help confirms they are appropriate.

## Prove drift

Check for:

- Renamed, moved, deprecated, or removed commands.
- Added, removed, or changed flags and accepted values.
- Incorrect long-form examples, pagination, output, confirmation, or pretty-print behavior.
- Auth, environment-variable, timeout, and default-output changes.
- Workflow ordering changes that affect builds, TestFlight, metadata, review, signing, or release operations.
- Examples that parse but no longer perform the claimed workflow.

Do not update skills merely because a newer CLI version exists. Record exact help or runtime evidence for every edit.

## Update minimally

1. Change only affected skills and examples; avoid release-number churn and speculative prose.
2. Keep each `SKILL.md` concise and move lengthy reference material behind progressive disclosure.
3. Preserve the skills repository's naming, metadata, validation, and style conventions.
4. Validate every changed command example against a current built or released `asc` binary.
5. Run the skills repository's validators and the current skill-creator validator when available.
6. Open a separate draft PR in the skills repository with the CLI commit or release used as evidence and a command-by-command validation summary.

Never modify the CLI merely to make stale skill documentation pass.

## Automation contract

A weekly or post-release automation may run a read-only drift audit and report proven mismatches. It may open a draft PR only when the automation has explicit write authorization and every change is deterministic; otherwise leave a precise change plan.

## Hand off

Report the CLI version and commit, skills commit, drift found, files changed, commands verified, validators run, PR URL, and any workflow that could not be tested safely.

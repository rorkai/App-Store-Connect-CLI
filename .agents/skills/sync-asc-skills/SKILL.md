---
name: sync-asc-skills
description: Check rorkai/app-store-connect-cli-skills against the current ASC CLI surface and correct proven drift when authorized. Use for skill-drift questions, post-release syncs, or checks of command, flag, output, auth, and workflow examples.
---

# Synchronize ASC workflow skills

Drift questions produce a read-only report; update requests authorize proven corrections. Follow `AGENTS.md` for commit, push, and PR-creation authority.

## Establish both sources

1. Resolve the current App-Store-Connect-CLI source commit and latest released version relevant to the request.
2. Resolve a clean checkout of `rorkai/app-store-connect-cli-skills` and read its repository guidance.
3. For a narrow request, inventory the requested command or skill surface and its workflow dependencies. For broad or post-release audits, inventory every skill. Identify the commands, flags, environment variables, outputs, and workflows in scope.
4. Run the current CLI's `--help` at each relevant command path. Use `asc search`, `asc schema`, or `asc capabilities` only when their own current help confirms they are appropriate.

Follow `AGENTS.md` for parallel reads and serialized writes. Validate mutation-capable examples with help and schemas first; execution and cleanup require authority, and dry runs must be side-effect-free.

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
4. Validate every changed command example against current built or released `asc` help and the relevant schema. Exercise runtime behavior when safe and authorized; report any path not executed.
5. Run the skills repository's validators and the current skill-creator validator when available.
6. When PR creation is authorized, open a separate draft PR in the skills repository with the CLI commit or release used as evidence and a summary of validation and unexecuted paths.

Never modify the CLI merely to make stale skill documentation pass.

## Automation contract

A weekly or post-release automation may run a read-only drift audit and report proven mismatches. It may open a draft PR only when the automation has explicit write authorization and every change is deterministic; otherwise leave a precise change plan.

## Hand off

Report the CLI version and commit, skills commit, drift found, files changed, commands verified, validators run, PR URL, and any workflow that could not be tested safely.

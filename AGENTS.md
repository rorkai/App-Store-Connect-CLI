# AGENTS.md

Unofficial, fast, lightweight, agent-assisted, reviewer-owned CLI for the App Store Connect API. Built in Go with [ffcli](https://github.com/peterbourgon/ff).

## Skills

Skills for using `asc` in app workflows live at https://github.com/rorkai/app-store-connect-cli-skills.

Repository-maintainer workflows live under `.agents/skills/`:

- `$develop-asc-change`: design, implement, and verify commands, flags, endpoints, bug fixes, and behavior-changing refactors.
- `$audit-asc-pr`: audit a complete PR and, when authorized, fix proven defects.
- `$watch-asc-pr`: recheck PR comments, checks, head changes, and merge readiness.
- `$triage-asc-issue`: reproduce, classify, label, and scope an issue.
- `$review-wall-of-apps-prs`: validate, approve, and merge Wall of Apps submissions safely.
- `$release-asc-cli`: publish and verify an end-to-end CLI repository release.
- `$sync-asc-skills`: check the external ASC workflow skills for CLI-surface drift.

Use these skills for their matching workflows instead of expanding this always-loaded file with task-specific procedures.

## Authority and follow-through

- Audits, reviews, research, triage, status checks, and composing draft text are read-only. Creating or updating a remote draft is an external write. Edits, commits, pushes, PR creation, comments, labels, approval, merge, publication, external sends, and deletion require authority from the request or session context. One request may authorize several actions; do not ask again for granted authority. Creating a PR for agreed changes includes the necessary edits, checks, commit, branch push, and PR creation, but not merge.
- User instructions override skill guidelines within system and developer constraints; skill selection grants no authority. Make routine choices from repository conventions. Ask only about material scope, compatibility, or authority gaps, after preparing authorized work for review; continue independent work while awaiting answers.
- If a skill causes a pause or scope change, link its `SKILL.md`, quote the instruction, and explain the unresolved decision. Preserve the objective, targets, authority, and verified progress across follow-ups and skill handoffs unless the user changes scope. Switching skills continues the current task; a failing gate blocks dependent actions, not independent authorized work. Inspect state before retrying an interrupted or uncertain write.

## Core CLI contract

- Use long-form flags in docs, tests, and examples (`--app`, `--output`).
- Output defaults are TTY-aware: `table` in terminals and minified `json` for pipes or CI. Explicit `--output` wins.
- Do not add interactive prompts. Require `--confirm` for destructive operations.
- Use `--paginate` when callers request every page.
- Write data to stdout and errors or diagnostics to stderr.
- Never accept and silently ignore an unsupported flag or value.
- JSON output shape is part of the command contract: reads of ASC collections/resources print Apple's envelope unmodified; mutation receipts and computed results use exported camelCase structs in internal/asc/output_*.go with a registered renderer; added fields are additive and removals follow the stability ladder.
- Website changelog entries must omit Wall of Apps membership churn, including app additions, removals, renames, icon changes, and routine metadata refreshes. Document only significant user-facing changes to the Wall of Apps feature or contribution workflow.

## Discover current behavior

Never rely on memorized command shapes. Before implementing, testing, or documenting a command, inspect its current help:

```bash
asc --help
asc builds --help
asc builds list --help
```

For App Store Connect API documentation, prefer the `sosumi.ai` mirror over `developer.apple.com`.

Use the offline OpenAPI snapshot for endpoint and schema truth:

- `docs/openapi/latest.json`: complete snapshot.
- `docs/openapi/paths.txt`: quick endpoint index.
- `docs/openapi/README.md`: update procedure.

Validate attributes against the exact create or update request schema. Validate filters and includes against the specific endpoint; related top-level and relationship endpoints often differ.

## Development workflow

- Work on a branch and use an isolated worktree when the main checkout is dirty or other work is in progress.
- Do not push directly to `main`, bypass hooks, use `--no-verify`, or skip checks to force a result.
- Use TDD for behavior changes: reproduce or establish RED, implement the smallest coherent change, then reach GREEN.
- **IMPORTANT:** Before ending every implementation session, run `/review`. In a non-interactive environment, use the stable local Codex CLI command `codex exec --ignore-user-config review --disable apps --disable plugins --disable remote_plugin -c 'model_provider="openai"' -c 'model="gpt-5.6-sol"' -c 'review_model="gpt-5.6-sol"' --uncommitted` as a pre-commit review of working-tree changes.
- Run reviews from the local worktree with the exact provider, model, config-isolation, and connector-disable flags shown. Immediately before each review, confirm `codex login status` shows ChatGPT authentication and both `CODEX_API_KEY` and `OPENAI_API_KEY` are unset or empty. Stop on API-key authentication or overrides unless explicitly authorized; these can incur separate usage charges.
- Before calling a PR ready, refresh its authoritative base with `git fetch origin +refs/heads/<base-branch>:refs/remotes/origin/<base-branch>` and run `codex exec --ignore-user-config review --disable apps --disable plugins --disable remote_plugin -c 'model_provider="openai"' -c 'model="gpt-5.6-sol"' -c 'review_model="gpt-5.6-sol"' --base origin/<base-branch>` on the final committed head so the complete branch or PR diff is reviewed against the current base. If the applicable review mechanism is unavailable, report the review gate as blocked and do not call the PR ready. Fix and verify every valid finding, and explicitly verify and disposition any false positive or non-actionable finding. After any change, rerun the applicable review command and repeat until it reports no actionable findings.
- PR readiness requires a clear final full-branch `/review` and the GitHub gates below. Any subsequent diff change invalidates the review. An investigation may finish with findings or blocked gates without implying PR readiness.
- Keep one logical change per commit.
- Preserve additive PR history. Do not squash, rebase, force-push, or otherwise rewrite commits unless the user explicitly requests that strategy.
- Re-run the focused failing test after each fix before broad validation.
- Delegate substantive independent reads or focused validation with distinct ownership; verify conclusions centrally. Handle small or dependent work locally. Serialize edits, commits, pushes, review communication, approvals, merges, releases, and cleanup; agents must not concurrently edit shared branches or files.
- Treat repository-wide commands that compile, lint, or test broad package sets—including `make build`, `make lint`, `make test`, `go test ./...`, race tests over `./...`, and `golangci-lint run ./...`—as host-intensive gates. Coordinate them through one agent and run only one host-intensive gate at a time on the same host. Before starting one, check whether another task is already running a host-intensive gate; if so, wait instead of competing for the same CPUs. Never terminate another task's process without explicit authorization.
- Within a worktree, wait for each focused test to finish before starting a broad gate. Record the checked commit, relevant working-tree state, command, and environment. Reuse passing checks while their inputs remain unchanged, including during status-only follow-ups. Rerun affected checks when source, test inputs, commands, environment, toolchain, or requested verification changes; unrelated temporary files do not invalidate results. Complete every required gate for the final change, and preserve the separate full-branch review requirement after any diff change.
- Run host-intensive gates concurrently only when explicitly required. Assign each gate a CPU budget and keep the sum of concurrent gate budgets within the host's logical CPU count; tool flags are limits within a gate, not values to add together. Avoid multiplying Go package and in-binary concurrency: for a budget of `B`, use `GOMAXPROCS=1 go test -p=B -parallel=1 ./...` for package fan-out or `GOMAXPROCS=B go test -p=1 -parallel=B ./...` for one package at a time. Use `golangci-lint run --concurrency=B ./...` for a linter gate.

User-facing commands and flags are stable until deprecated or removed. Do not delete stable behavior directly. Deprecations require warning text, transition tests, migration guidance, and a release-note entry.

## Implementation invariants

- Put command implementations in `internal/cli/<domain>` and register new top-level commands in `internal/cli/registry/registry.go`.
- Set `UsageFunc: shared.DefaultUsageFunc` on command groups and subcommands.
- Use `shared.ContextWithTimeout` or `shared.ContextWithUploadTimeout` for outbound HTTP.
- Read and write repository-controlled or API-supplied paths through `internal/rootfs`, anchored to the operator-selected root for that command, instead of plain `os` file operations.
- Validate required flags before side effects and assert stderr messages in tests.
- Use `internal/cli/cmdtest` for CLI-level coverage and `httptest` for HTTP payload coverage.
- Remove shared wrappers or helpers made obsolete by a refactor.

## Build and validation

Every manual test command must use `ASC_BYPASS_KEYCHAIN=1` to prevent host keychain prompts and profile bleed-through. The `make test` target enforces the same environment internally.

Before opening or merging a substantive behavior PR, run `make build`, `make format`, `make check-docs`, `make lint`, and `ASC_BYPASS_KEYCHAIN=1 make test`. If command help changed, run `make generate-command-docs` and commit `docs/COMMANDS.md` before those checks. For a narrowly scoped documentation or skill change, run `make check-docs`, which includes the repository and skill validators, instead of the full Go suite unless the changed surface or repository policy requires more. For a Wall-only PR, run `make check-wall-of-apps` on the exact head.

Require GitHub-required checks before merge. Report relevant advisory failures, but do not wait for non-required jobs.

Do not weaken CI: formatting, documentation, lint, and tests must run on PR and `main` workflows.

## GitHub and issue guardrails

- Inspect thread-aware GitHub review state before declaring a PR clean; flat comments do not prove every thread is resolved.
- A PR is ready only when the latest head was reviewed, required checks pass, required reviews are satisfied, actionable threads are resolved, and GitHub reports it mergeable.
- If `main` advances, recheck the exact PR head, merge-base diff, duplicate or overlap risk, review threads, required checks, and mergeability against current `main` without changing the branch. Do not update, rebase, or merge `main` into a clean PR merely to refresh its base. Update a branch only when GitHub already reports an actual merge conflict, or when an explicitly authorized merge attempt made with every readiness gate passing is refused under strict up-to-date branch protection. Never bypass branch protection with an admin merge.
- For loop or babysit requests, follow `$watch-asc-pr` until ready or materially blocked, preserving the authorized actions. Use its checkpoint and heartbeat procedure when only waiting remains.
- When merge is explicitly authorized, preserve the PR commits with a regular merge commit, for example `gh pr merge <number> --merge --match-head-commit <sha>`. Do not squash unless the user explicitly requests squash for that PR.
- For read-only triage, recommend exactly one type (`bug`, `enhancement`, `question`), one priority (`p0`-`p3`), and one difficulty (`easy`, `medium`, `hard`). When issue creation or label changes are authorized, apply exactly one label from each bucket and remove conflicting labels.

## Authentication and live testing

App Store Connect API keys come from https://appstoreconnect.apple.com/access/integrations/api and must never be committed.

Tests touching auth must isolate relevant environment and config state. For live verification, prefer read-only calls. Live mutations require authorization, including on disposable app `6759231657`; a disposable target does not itself grant permission. Include temporary-resource cleanup in the authorized verification plan and record anything left behind. Never mutate a non-disposable app without explicit approval.

## Handoff contract

Report the outcome, decisive evidence, material unknowns, and next action concisely. Include design or command details only when useful. Distinguish source review, tests, remote checks, merge, release artifacts, and provider state. Report pre-existing failures and unverified acceptance criteria.

## References

- Go standards: `docs/GO_STANDARDS.md`
- Testing patterns: `docs/TESTING.md`
- Git workflow and CLI structure: `docs/CONTRIBUTING.md`
- API quirks: `docs/API_NOTES.md`
- Development setup, PRs, labels, and Wall submissions: `CONTRIBUTING.md`

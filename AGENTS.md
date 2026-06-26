# AGENTS.md

Unofficial, fast, lightweight, agent-assisted, reviewer-owned CLI for the App Store Connect API. Built in Go with [ffcli](https://github.com/peterbourgon/ff).

## Skills

Skills for using `asc` in app workflows live at https://github.com/rorkai/app-store-connect-cli-skills.

Repository-maintainer workflows live under `.agents/skills/`:

- `$develop-asc-change`: design, implement, and verify commands, flags, endpoints, bug fixes, and behavior-changing refactors.
- `$audit-asc-pr`: audit a complete PR and fix proven defects.
- `$watch-asc-pr`: recheck PR comments, checks, head changes, and merge readiness.
- `$triage-asc-issue`: reproduce, classify, label, and scope an issue.
- `$review-wall-of-apps-prs`: validate, approve, and merge Wall of Apps submissions safely.
- `$release-asc-cli`: publish and verify an end-to-end CLI repository release.
- `$sync-asc-skills`: check the external ASC workflow skills for CLI-surface drift.

Use these skills for their matching workflows instead of expanding this always-loaded file with task-specific procedures.

## Core CLI contract

- Use long-form flags in docs, tests, and examples (`--app`, `--output`).
- Output defaults are TTY-aware: `table` in terminals and minified `json` for pipes or CI. Explicit `--output` wins.
- Do not add interactive prompts. Require `--confirm` for destructive operations.
- Use `--paginate` when callers request every page.
- Write data to stdout and errors or diagnostics to stderr.
- Never accept and silently ignore an unsupported flag or value.

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
- Keep one logical change per commit. Do not mix unrelated refactors, fixes, and test rewrites.
- Re-run the focused failing test after each fix before broad validation.
- Preserve and report pre-existing failures honestly.
- Parallel exploration is allowed, but do not concurrently edit the same command group; integrate final changes in one coherent pass.

User-facing commands and flags follow `experimental` -> `stable` -> `deprecated` -> `removed`. Do not delete stable behavior directly. Deprecations require warning text, transition tests, migration guidance, and a release-note entry.

## Implementation invariants

- Put command implementations in `internal/cli/<domain>` and register new top-level commands in `internal/cli/registry/registry.go`.
- Set `UsageFunc: shared.DefaultUsageFunc` on command groups and subcommands.
- Use `shared.ContextWithTimeout` or `shared.ContextWithUploadTimeout` for outbound HTTP.
- Validate required flags before side effects and assert stderr messages in tests.
- Use `internal/cli/cmdtest` for CLI-level coverage and `httptest` for HTTP payload coverage.
- Remove shared wrappers or helpers made obsolete by a refactor.

## Build and validation

```bash
make build
make format
make check-docs
make lint
ASC_BYPASS_KEYCHAIN=1 make test
make install-hooks
```

Every manual test command must use `ASC_BYPASS_KEYCHAIN=1` to prevent host keychain prompts and profile bleed-through. The `make test` target enforces the same environment internally.

Before opening or merging a PR, run `make format`, `make check-docs`, `make lint`, and `ASC_BYPASS_KEYCHAIN=1 make test`. If command help changed, run `make generate-command-docs` and commit `docs/COMMANDS.md` before those checks. Run `make check-wall-of-apps` for Wall changes.

Do not weaken CI: formatting, documentation, lint, and tests must run on PR and `main` workflows.

## GitHub and issue guardrails

- Inspect thread-aware GitHub review state before declaring a PR clean; flat comments do not prove every thread is resolved.
- A PR is ready only when the latest head was reviewed, required checks pass, actionable threads are resolved, and GitHub reports it mergeable.
- Fix-forward is the default for `$audit-asc-pr`; approval and merge still require explicit user intent.
- Every newly created or triaged issue must end with exactly one type (`bug`, `enhancement`, `question`), one priority (`p0`-`p3`), and one difficulty (`easy`, `medium`, `hard`) label.

## Authentication and live testing

App Store Connect API keys come from https://appstoreconnect.apple.com/access/integrations/api and must never be committed.

Tests touching auth must isolate relevant environment and config state. For live verification, prefer read-only calls. When mutation is necessary during PR audits, prefer disposable app `6759231657`, clean up temporary resources, and record anything left behind. Never mutate a non-disposable app without explicit approval.

## Handoff contract

For substantial changes, explain the chosen approach, alternatives and trade-offs, expected invocations and outputs, compatibility impact, edge cases, failure modes, commands run, tests, live verification, commits or pushes, and unresolved risks.

## References

- Go standards: `docs/GO_STANDARDS.md`
- Testing patterns: `docs/TESTING.md`
- Git workflow and CLI structure: `docs/CONTRIBUTING.md`
- API quirks: `docs/API_NOTES.md`
- Development setup, PRs, labels, and Wall submissions: `CONTRIBUTING.md`

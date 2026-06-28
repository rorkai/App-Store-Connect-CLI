---
name: develop-asc-change
description: Design, implement, and verify behavior changes in App-Store-Connect-CLI. Use when adding or changing a command, flag, API endpoint, output format, exit code, shared CLI behavior, or when fixing or refactoring behavior that requires code and tests.
---

# Develop an ASC CLI change

Deliver one complete, reviewable behavior change through architecture, RED-GREEN implementation, realistic CLI verification, and PR-ready validation.

## Write the design note

Before implementation, record:

1. Placement in the existing command taxonomy and registry.
2. Current `--help` behavior and expected invocation shape.
3. Exact OpenAPI endpoint, method, request schema, query parameters, and response shape when API-facing.
4. Flags, output formats, stdout/stderr behavior, and exit codes.
5. Compatibility, lifecycle, migration, and deprecation impact.
6. RED-GREEN tests, black-box checks, live verification, edge cases, and failure modes.
7. One or two credible alternatives and why this shape is preferable.

Stop and align before coding if the public command shape or compatibility decision remains materially ambiguous.

## Establish RED

- For a bug, reproduce it first and add the smallest regression test that fails for the expected reason.
- For a feature, start with CLI-level tests for flags, output, errors, and exit behavior, then add unit or HTTP tests for core logic.
- For a behavior-changing refactor, add characterization coverage before moving code.
- Run the focused test and record the expected failure before implementation.

Read [references/test-matrix.md](references/test-matrix.md) for mandatory CLI, output, artifact, and auth cases.

## Validate API support

1. Search `docs/openapi/paths.txt`, then inspect the exact operation in `docs/openapi/latest.json`.
2. Validate attributes against the correct create or update request schema.
3. Validate filters and includes against the specific endpoint, not a related top-level or relationship endpoint.
4. If the API does not support the proposed behavior, do not ship a misleading flag. Use explicit client-side behavior or document the limitation.
5. Prefer the `sosumi.ai` mirror when explanatory App Store Connect API documentation is required.

## Implement narrowly

- Extend the correct `internal/cli/<domain>` package and register new top-level commands in `internal/cli/registry/registry.go`.
- Set `UsageFunc: shared.DefaultUsageFunc` for command groups and subcommands.
- Use `shared.ContextWithTimeout` or `shared.ContextWithUploadTimeout` for outbound HTTP.
- Validate required flags before side effects and return usage errors with exit code `2`.
- Write data to stdout and diagnostics to stderr. Never silently ignore accepted flags.
- Use long-form flags in documentation, tests, and examples.
- Require `--confirm` for destructive operations; do not add interactive prompts.
- Keep one logical change per commit and remove helpers made obsolete by the change.
- Deprecate stable commands or flags before removal, with warning text, transition tests, and an upgrade path.

## Reach GREEN and verify

1. Rerun the focused failing test after each small fix.
2. Run adjacent package and command tests.
3. Build `/tmp/asc` and verify realistic invocations, output streams, and exit codes against the built binary.
4. Run a minimal live smoke test when behavior depends on App Store Connect quirks. Prefer read-only calls; use disposable resources and clean them up for mutations.
5. Run the repository gate before opening or updating a PR:

```bash
make format
make check-docs
make lint
ASC_BYPASS_KEYCHAIN=1 make test
```

If command help changed, run `make generate-command-docs` and commit the resulting `docs/COMMANDS.md` update before the gate.

## Hand off

Explain the chosen approach, alternatives, expected invocations and outputs, compatibility impact, tests and live checks, commands run, and remaining limitations. Be explicit about pre-existing failures and anything not reproduced.

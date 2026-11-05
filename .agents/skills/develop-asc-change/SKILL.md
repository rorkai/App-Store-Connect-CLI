---
name: develop-asc-change
description: Design, implement, and verify behavior changes in App-Store-Connect-CLI. Use when adding or changing a command, flag, API endpoint, output format, exit code, shared CLI behavior, or when fixing or refactoring behavior that requires code and tests.
---

# Develop an ASC CLI change

Follow `AGENTS.md` for authority, CLI contracts, implementation invariants, validation, and review gates.

## Write the design note

For a small fix, record the reproduced failure, intended behavior, and focused check. For new public behavior or compatibility decisions, also record:

1. Placement in the existing command taxonomy and registry.
2. Current `--help` behavior and expected invocation shape.
3. Exact OpenAPI endpoint, method, request schema, query parameters, and response shape when API-facing.
4. Flags, output formats, stdout/stderr behavior, and exit codes.
5. Compatibility, lifecycle, migration, and deprecation impact.
6. RED-GREEN tests, black-box checks, live verification, edge cases, and failure modes.
7. Credible alternatives when a material design trade-off exists.

## Establish RED

- For a bug, reproduce it first and add the smallest regression test that fails for the expected reason.
- For a feature, start with CLI-level coverage of the changed observable contract; add unit or HTTP tests for distinct behavior that those tests do not cover.
- For a behavior-changing refactor, use existing characterization coverage where sufficient and add coverage for missing behavior before moving code.
- Run the focused test and record the expected failure before implementation.

Read [references/test-matrix.md](references/test-matrix.md) for applicable CLI, output, artifact, and auth cases. Reuse sufficient existing coverage; do not duplicate shared parser or renderer tests for every command. Every new test must meet the test rules in `AGENTS.md`.

## Validate API support

1. Search `docs/openapi/paths.txt`, then inspect the exact operation in `docs/openapi/latest.json`.
2. Apply the endpoint-specific schema checks in `AGENTS.md`.
3. If the API cannot support the behavior, implement explicit client-side behavior or document the limitation; do not ship a misleading flag.

## Implement narrowly

Apply `AGENTS.md`'s implementation invariants. Return usage errors with exit code `2`.

## Reach GREEN and verify

1. Rerun the focused failing test after each small fix.
2. Run adjacent package and command tests when useful for diagnosis; omit a separate run when the immediately following full validation gate covers them.
3. Apply `AGENTS.md`'s validation gate, including command-doc generation when help changes. Public CLI behavior, shared code, release surfaces, and meaningful defect or security fixes require the full gate.
4. Verify realistic invocations, output streams, and exit codes using a binary at a worktree-specific path. Reuse the required validation gate's build when source and build inputs match; otherwise build it once. Do not share a fixed `/tmp/asc` path with concurrent tasks.
5. Run a minimal live smoke test when behavior depends on App Store Connect quirks. Prefer read-only calls; live mutations and cleanup require authority under `AGENTS.md`.

Before handoff, compare the exact branch head with current `main` read-only. Follow the branch-update rules in `AGENTS.md`; a newer base alone does not justify changing the branch.

## Hand off

Use `AGENTS.md`'s handoff contract; include invocation and compatibility details for public-contract changes.

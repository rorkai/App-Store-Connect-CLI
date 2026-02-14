# AGENTS.md

A fast, lightweight, AI-agent-friendly CLI for App Store Connect. Built in Go with [ffcli](https://github.com/peterbourgon/ff).

## ASC Skills

Agent Skills for automating `asc` workflows including builds, TestFlight, metadata sync, submissions, and signing. https://github.com/rudrankriyam/app-store-connect-cli-skills

## Core Principles

- **Explicit flags**: Always `--app` not `-a`, `--output` not `-o`
- **JSON-first**: Minified JSON by default (saves tokens), `--output table/markdown` for humans
- **No interactive prompts**: Use `--confirm` flags for destructive operations
- **Pagination**: `--paginate` fetches all pages automatically

## Discovering Commands

**Use `--help` to discover commands and flags.** The CLI is self-documenting:

```bash
asc --help                    # List all commands
asc builds --help             # List builds subcommands
asc builds list --help        # Show all flags for a command
```

Do not memorize commands. Always check `--help` for the current interface.

## Documentation

For a compact command catalog and workflows, see `ASC.md`.

When looking up App Store Connect API docs, prefer the `sosumi.ai` mirror instead of `developer.apple.com`.
Replace `https://developer.apple.com/documentation/appstoreconnectapi/...` with `https://sosumi.ai/documentation/appstoreconnectapi/...`.

## OpenAPI (offline)

For endpoint existence and request/response schemas, use the offline snapshot:
`docs/openapi/latest.json` and the quick index `docs/openapi/paths.txt`.
Update instructions live in `docs/openapi/README.md`.

## Build & Test

```bash
make build      # Build binary
make test       # Run tests (always run before committing)
make lint       # Lint code
make format     # Format code
make install-hooks  # Install local pre-commit hook (.githooks/pre-commit)
```

## PR Guardrails

- Before opening or merging a PR, run `make format`, `make lint`, and `make test`.
- Use `make install-hooks` once per clone to enforce local pre-commit checks.
- CI must enforce formatting + lint + tests on both PR and `main` workflows.
- Remove unused shared wrappers/helpers when commands are refactored.

## Testing Discipline

- Use TDD for everything: bugs, refactors, and new features.
- Start with a failing test that captures the expected behavior and edge cases.
- For new features, begin with CLI-level tests (flags, output, errors) and add unit tests for core logic.
- Verify the test fails for the right reason before implementing; keep tests green incrementally.
- **Test realistic CLI invocation patterns**, not invented happy paths. For example, when testing argument parsing, always consider:
  - Flags before subcommands: `asc --flag subcmd` vs `asc subcmd --flag`
  - Flag values that look like subcommands: `asc --report junit completion`
  - Multiple flags with values: `asc -a val1 -b val2 subcmd`
- **Model tests on actual CLI usage**, not assumed patterns. Check `--help` output to understand real command structure before writing tests.

## Debugging & Bug Fixing

- **Reproduce first**: Before fixing, run the failing test locally to confirm the issue. Don't assume you understand the bug.
- **One change at a time**: Make one small fix, verify it works, then move to the next. Don't batch multiple changes.
- **One logical change per commit**: Keep commits narrowly scoped and reviewable. Avoid mixing refactor + bug fix + test rewrites in a single commit.
- **Never bypass checks**: Don't use `--no-verify`, don't push directly to `main`, don't skip tests to "get around" failures.
- **Be honest about pre-existing issues**: If a test was failing before your changes, say so. Don't claim credit for "fixing" something you didn't break.
- **Verify before claiming done**: Run the specific failing test again to confirm it's fixed, not just "all tests pass".
- **Avoid broad skip logic**: Don't skip tests with generic string matches (e.g., "Keychain Error") that can hide regressions. Match specific error codes instead.
- **Isolate test auth/env state**: Tests that touch auth must set/clear relevant env vars (`ASC_BYPASS_KEYCHAIN`, `ASC_PROFILE`, `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_PRIVATE_KEY_PATH`, `ASC_PRIVATE_KEY`, `ASC_PRIVATE_KEY_B64`, `ASC_STRICT_AUTH`) locally and restore exact original state.
- **Strict skip policy**: `t.Skip` is allowed only for specific, documented, reproducible conditions (exact error code/condition). Generic skip patterns are not allowed.
- **Use proper workflow**: Branch → change → test → PR. Not: main → change → push.

## Definition of Done (Single-Pass)

- Do not mark work as done until all checks below are complete.
- For CLI behavior changes (flags, exit codes, output/reporting), follow this sequence:
  - Add/adjust failing tests first (RED), then implement (GREEN).
  - Do not add placeholder tests; every new test must assert exit code, stderr/stdout, and/or parsed structured output.
  - For every new or changed flag, add:
    - one valid-path test
    - one invalid-value test that asserts stderr and exit code `2`
  - For argument/subcommand parsing, test edge cases: flags before subcommands, flag values that match subcommand names, mixed flag order.
  - Never silently ignore accepted flags; unsupported values must return an error.
  - For JSON/XML output tests, parse output (`json.Unmarshal`/`xml.Unmarshal`) instead of relying only on string matching.
  - For report/artifact file outputs, test both successful write and write-failure behavior.
  - Verify CLI exit behavior using a built binary (not only `go run`) for black-box checks:
    - `go build -o /tmp/asc .`
    - run `/tmp/asc ...` and assert exit code + stderr/stdout
- Before opening/updating a PR, always run:
  - `make format`
  - `make lint`
  - `make test`
- In the PR description or handoff, include:
  - commands run
  - key exit-code scenarios validated

## CLI Implementation Checklist

- Register new commands in `internal/cli/registry/registry.go`.
- Always set `UsageFunc: shared.DefaultUsageFunc` for command groups and subcommands.
- For outbound HTTP, use `shared.ContextWithTimeout` (or `shared.ContextWithUploadTimeout`) so `ASC_TIMEOUT` applies.
- Validate required flags and assert stderr error messages in tests (not just `flag.ErrHelp`).
- Add `internal/cli/cmdtest` coverage for new commands; use `httptest` for network payload tests.

## Authentication

API keys are generated at https://appstoreconnect.apple.com/access/integrations/api and stored in the system keychain (with local config fallback). Never commit keys to version control.

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_PRIVATE_KEY_PATH`, `ASC_PRIVATE_KEY`, `ASC_PRIVATE_KEY_B64` | Auth fallback |
| `ASC_BYPASS_KEYCHAIN` | Ignore keychain and use config/env auth |
| `ASC_STRICT_AUTH` | Fail when credentials resolve from multiple sources (`true/false`, `1/0`, `yes/no`, `y/n`, `on/off`) |
| `ASC_APP_ID` | Default app ID |
| `ASC_VENDOR_NUMBER` | Sales/finance reports |
| `ASC_TIMEOUT` | Request timeout (e.g., `90s`, `2m`) |
| `ASC_TIMEOUT_SECONDS` | Timeout in seconds (alternative) |
| `ASC_UPLOAD_TIMEOUT` | Upload timeout (e.g., `60s`, `2m`) |
| `ASC_UPLOAD_TIMEOUT_SECONDS` | Upload timeout in seconds (alternative) |
| `ASC_DEBUG` | Enable debug logging (set to `api` for HTTP requests/responses) |
| `ASC_DEFAULT_OUTPUT` | Default output format: `json`, `table`, `markdown`, or `md` |
| `ASC_NO_UPDATE` | Disable update checks and auto-update |

Explicit `--output` flags always override `ASC_DEFAULT_OUTPUT`.

## References

Detailed guidance on specific topics (only read when needed):

- **Go coding standards**: `docs/GO_STANDARDS.md`
- **Testing patterns**: `docs/TESTING.md`
- **Git workflow, CLI structure, adding features**: `docs/CONTRIBUTING.md`
- **API quirks (analytics, finance, sandbox)**: `docs/API_NOTES.md`
- **Development setup, PRs**: `CONTRIBUTING.md` (root)

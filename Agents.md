# AGENTS.md

A fast, lightweight, AI-agent-friendly CLI for App Store Connect. Built in Go with [ffcli](https://github.com/peterbourgon/ff).

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
```

## Testing Discipline

- Use TDD for everything: bugs, refactors, and new features.
- Start with a failing test that captures the expected behavior and edge cases.
- For new features, begin with CLI-level tests (flags, output, errors) and add unit tests for core logic.
- Verify the test fails for the right reason before implementing; keep tests green incrementally.

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
| `ASC_STRICT_AUTH` | Fail when credentials resolve from multiple sources |
| `ASC_APP_ID` | Default app ID |
| `ASC_VENDOR_NUMBER` | Sales/finance reports |
| `ASC_TIMEOUT` | Request timeout (e.g., `90s`, `2m`) |
| `ASC_TIMEOUT_SECONDS` | Timeout in seconds (alternative) |
| `ASC_UPLOAD_TIMEOUT` | Upload timeout (e.g., `60s`, `2m`) |
| `ASC_UPLOAD_TIMEOUT_SECONDS` | Upload timeout in seconds (alternative) |

## References

Detailed guidance on specific topics (only read when needed):

- **Go coding standards**: `docs/GO_STANDARDS.md`
- **Testing patterns**: `docs/TESTING.md`
- **Git workflow, CLI structure, adding features**: `docs/CONTRIBUTING.md`
- **API quirks (analytics, finance, sandbox)**: `docs/API_NOTES.md`
- **Development setup, PRs**: `CONTRIBUTING.md` (root)

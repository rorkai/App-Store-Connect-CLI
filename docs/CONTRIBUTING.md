# Contributing

For development setup, integration tests, and PR guidelines, see the root `CONTRIBUTING.md`.

This file covers patterns for AI agents working on the codebase.

## Git Workflow

- Branch from `main` and keep one logical change per branch
- Do not commit directly to `main` unless explicitly instructed; prefer PRs
- Prefer `git worktree add` for parallel tasks; remove with `git worktree remove` when done
- Rebase on `main` before merging; avoid merge commits
- Commit small, coherent changes; no WIP commits on shared branches
- Use concise, present-tense commit messages that match repo style
- Never commit secrets or local config files (keys, `.env`, `.asc/config.json`)

## Before Committing

```bash
make format     # Format code
make lint       # Check for issues
make check-docs  # Verify repo docs, website docs, and command docs stay in sync
ASC_BYPASS_KEYCHAIN=1 make test  # Run all tests without keychain prompts
git diff        # Review changes before staging
```

If `docs/wall-of-apps.json` is the only staged change, the local hook skips the full Go pipeline and only runs:

```bash
make check-wall-of-apps
```

## CLI Structure

- Command implementations live in `internal/cli/<domain>` packages
- Each domain exposes a top-level `XCommand() *ffcli.Command`
- `cmd/` only contains the root entry point; do not add wrapper files
- Register top-level commands in `internal/cli/registry/registry.go` (order matters)
- Shared CLI helpers go in `internal/cli/shared` (use `shared_wrappers.go` as needed)

## Adding a New Command

1. Add or extend a domain package in `internal/cli/<domain>`
2. Implement a command factory (e.g., `XCommand() *ffcli.Command`)
3. Register it in `internal/cli/registry/registry.go`
4. Write tests in the domain package (or `internal/cli/cmdtest` for root-level tests)
5. Update README.md with usage examples

## Adding a New API Endpoint

1. Add method to `internal/asc/client.go`
2. Add types for request/response structs
3. Add helper functions for table/markdown output
4. Create command in `internal/cli/<domain>` to expose the endpoint
5. Write HTTP client tests with mocked responses
6. If endpoint tests are repetitive, group the request-wiring cases, but keep at least one representative non-empty decode assertion and one representative output-structure assertion where formatting is user-facing

## Releases

Tag releases with plain semver like `0.1.0` (no `v` prefix).

### Pre-Release Checklist

Before tagging a release, verify:

```bash
# 1. Update release-facing docs
#    - refresh resources/changelog.mdx for the new version
#    - update any website pages affected by the release

# 2. Verify documentation
make check-docs
make check-release-docs VERSION=0.1.0

# 3. All tests pass
ASC_BYPASS_KEYCHAIN=1 make test

# 4. Audit help output for all parent commands
for cmd in auth analytics finance apps app-tags testflight builds versions \
           feedback crashes localizations \
           build-localizations sandbox submit xcode-cloud reviews; do
  echo "=== $cmd ===" && ./asc $cmd --help 2>&1
done

# 5. Check for duplicate sections (should see SUBCOMMANDS only once per command)
# 6. Verify bold formatting renders correctly
```

**Common issues to check:**
- No duplicate "Subcommands:" sections (don't list subcommands in LongHelp; DefaultUsageFunc handles it)
- All flags have descriptions
- Examples are up to date

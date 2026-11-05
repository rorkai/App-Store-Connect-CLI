# Contributing

For development setup, integration tests, and PR guidelines, see the root `CONTRIBUTING.md`.

This file covers patterns for AI agents working on the codebase.

## Git Workflow

- Branch from `main` and keep one logical change per branch
- Do not commit directly to `main` unless explicitly instructed; prefer PRs
- Prefer `git worktree add` for parallel tasks; remove with `git worktree remove` when done
- Compare the branch with current `main` before merging. Do not rebase or merge `main` into a clean branch merely because `main` advanced; update only when GitHub reports an actual merge conflict or refuses an explicitly authorized merge attempt made after every readiness gate passes under strict up-to-date branch protection, and never bypass protection with an admin merge
- Preserve additive PR commits. Do not force-push, rebase, squash, or otherwise rewrite shared history unless the user explicitly requests it. When merging, use a regular merge commit by default
- Commit small, coherent changes; no WIP commits on shared branches
- Use concise, present-tense commit messages that match repo style
- Never commit secrets or local config files (keys, `.env`, `.asc/config.json`)

## Before Committing

For substantive behavior changes, run:

```bash
make build      # Build the CLI
make format     # Format code
make lint       # Check for issues
make check-docs  # Verify repo docs, website docs, and command docs stay in sync
ASC_BYPASS_KEYCHAIN=1 make test  # Run all tests without keychain prompts
git diff        # Review changes before staging
```

For a narrowly scoped documentation or skill change, run `make check-docs`; it includes the repository and skill validators. Use broader checks when the changed surface or repository policy requires them.

If `docs/wall-of-apps.json` is the only staged change, the local hook skips the full Go pipeline and only runs:

```bash
make check-wall-of-apps
```

## Repository Agent Skills

Keep always-on repository invariants in `AGENTS.md` and task-specific maintainer workflows in `.agents/skills/<skill-name>/`.

Each skill must contain a `SKILL.md` with only `name` and `description` frontmatter plus matching `agents/openai.yaml` UI metadata. Keep trigger descriptions specific, link detailed references through progressive disclosure, and run:

```bash
make check-agent-skills
```

`make check-docs` includes this validation.

## CLI Structure

- Command implementations live in `internal/cli/<domain>` packages
- Each domain exposes a top-level `XCommand() *ffcli.Command`
- `cmd/` holds the root command and root-level dispatch (exit codes, root flag handling); do not add per-command wrapper files there
- Register top-level commands in `internal/cli/registry/registry.go` (order matters)
- Shared CLI helpers go in `internal/cli/shared`

## Adding a New Command

1. Add or extend a domain package in `internal/cli/<domain>`
2. Implement a command factory (e.g., `XCommand() *ffcli.Command`)
3. Register it in `internal/cli/registry/registry.go`
4. Write tests in the domain package (or `internal/cli/cmdtest` for root-level tests)
5. Update README.md with usage examples

## Adding a New API Endpoint

1. Add the method to the resource's `internal/asc/client_*.go` file
2. Add types for request/response structs
3. Register table/markdown rows in the matching `internal/asc/output_*.go` file
4. Create command in `internal/cli/<domain>` to expose the endpoint
5. Write HTTP client tests with mocked responses
6. If endpoint tests are repetitive, group the request-wiring cases, but keep at least one representative non-empty decode assertion and one representative output-structure assertion where formatting is user-facing

## Releases

Tag releases with plain semver like `0.1.0` (no `v` prefix).

### Pre-Release Checklist

Before tagging a release, verify:

```bash
# 1. Update release-facing docs
#    - update any website pages affected by the release
#    - release notes are auto-generated from merged PRs on tag push

# 2. Verify documentation
make check-docs

# 3. All tests pass
ASC_BYPASS_KEYCHAIN=1 make test

# 4. Audit help output for all parent commands
for cmd in auth analytics finance apps app-tags testflight builds versions \
           localizations build-localizations sandbox submit xcode-cloud reviews; do
  echo "=== $cmd ===" && ./asc $cmd --help 2>&1
done

# 5. Check for duplicate sections (should see SUBCOMMANDS only once per command)
# 6. Verify bold formatting renders correctly
```

**Common issues to check:**
- No duplicate "Subcommands:" sections (don't list subcommands in LongHelp; DefaultUsageFunc handles it)
- All flags have descriptions
- Examples are up to date

## Parity map

Remaining fastlane/ASC parity areas and the intentional non-goal list live in [PARITY.md](PARITY.md).

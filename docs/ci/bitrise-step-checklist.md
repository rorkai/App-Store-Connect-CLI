# Bitrise Step Checklist (`#712`)

Issue: https://github.com/rudrankriyam/App-Store-Connect-CLI/issues/712

## Naming Decision

- [x] Select canonical naming aligned with other CI integration issues.
  - Step repository: `rudrankriyam/steps-setup-asc`
  - Step ID (StepLib): `setup-asc`
  - Step title: `Setup asc CLI`
  - Rationale: aligns with existing `setup-asc` branding, stays short in Workflow Editor, and still supports both install-only and install+run modes.

## v1 Functional Scope

- [x] Define v1 behavior modes.
  - `mode=install` installs `asc` and exports outputs.
  - `mode=run` installs `asc`, sets optional auth env vars, runs provided command.
- [x] Define minimum required inputs.
  - `version`, `mode`, `command`, `working_dir`, `profile`, `debug`
- [x] Define optional auth inputs.
  - `key_id`, `issuer_id`, `private_key_path`, `private_key`, `bypass_keychain`
- [x] Define minimum outputs.
  - `ASC_CLI_PATH`, `ASC_CLI_VERSION`, `ASC_COMMAND_EXIT_CODE`

## Implementation Tasks

- [x] Scaffold Step repository files.
  - `step.yml`
  - `step.sh`
  - `README.md`
  - `bitrise.yml`
- [x] Implement deterministic installer logic.
  - Resolve `latest` tag from GitHub releases redirect.
  - Support pinned versions with and without `v` prefix.
  - Detect OS/arch for release asset selection.
  - Verify SHA-256 using release checksums.
- [x] Implement command execution mode.
  - Validate `command` is present when `mode=run`.
  - Execute from `working_dir` (default `$BITRISE_SOURCE_DIR`).
  - Export `ASC_COMMAND_EXIT_CODE` before exit.

## Security and UX

- [x] Mark secret-capable inputs as `is_sensitive` in `step.yml`.
- [x] Avoid echoing secret values in logs.
- [x] Keep defaults CI-safe and idempotent.
- [x] Expose clean and descriptive input docs for Workflow Editor.

## Test and Release Checklist

- [x] Add CI workflow automation for cross-platform validation.
  - GitHub Actions matrix: `ubuntu-latest` and `macos-latest`
  - Runs `stepman audit` and Bitrise workflows (`audit-this-step`, `test-install`, `test-run-help`)
- [x] Run local shell smoke tests on macOS host.
  - `mode=install` works and installs `asc`.
  - `mode=run` works and executes `asc --help`.
- [x] Run local audit workflow (`stepman audit --step-yml ./step.yml`).
- [ ] Validate `mode=install` on Linux stack.
- [x] Validate `mode=install` on macOS stack.
- [x] Validate `mode=run` with harmless command (`asc --help`).
- [ ] Tag and publish first release in `steps-setup-asc`.
- [ ] Submit StepLib PR with one new Step.
- [ ] Add usage snippet in main `asc` docs after StepLib publish.


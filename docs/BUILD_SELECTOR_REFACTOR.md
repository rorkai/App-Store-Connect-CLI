# Build Selector Refactor

## Goal

Unify build selection across the `builds` command family under one resource-first
model:

- explicit build selection uses `--build-id`
- inferred build selection uses `--app ... --latest`
- specific build lookup uses `--app ... --build-number`

This cleanup standardizes deprecated and inconsistent selector vocabulary while
preserving compatibility aliases until the broader `1.0.0` cleanup.

## Accepted Decisions

- `--newest` remains as a deprecated alias for `--latest` during transition.
- `--latest` is the only inferred-build selector.
- `--build-id` is the canonical explicit build selector everywhere.
- `--build` remains as a deprecated alias for `--build-id` during transition.
- `--id` remains as a deprecated alias for `--build-id` on build-related read
  surfaces during transition.
- `asc builds latest` will be removed as a fetch command.
- `asc builds find` is removed once `asc builds info` can resolve by
  `--build-number`.
- `asc builds next-number` will replace current `asc builds latest --next`.
- `test-notes` becomes build-scoped plus `--locale`, not localization-ID-first.

## Non-Goals

- No app-first taxonomy rewrite. `builds` remains top-level.
- No mutation-command expansion in the first wave unless explicitly scoped.
- No selector-alias expansion beyond the existing `--build`, `--id`, and
  `--newest` compatibility spellings.

## PR Plan

### PR 1: Selector Vocabulary + Core Resolver

Status: complete

Progress checklist:

- [x] Add refactor tracker/design note
- [x] Standardize shared resolver validation wording to `--build-id`
- [x] Convert `asc builds dsyms` from `--build` to `--build-id`
- [x] Convert `asc builds wait` from `--build` / `--newest` to
  `--build-id` / `--latest`
- [x] Keep legacy selector spellings as deprecated aliases that warn and forward
  to the canonical flags
- [x] Update focused tests and command docs for the PR 1 slice
- [ ] Decide whether `builds wait` should later reuse more of the shared
  selector engine instead of only sharing vocabulary
- [x] Extend `--build-id` vocabulary to the remaining read-oriented explicit
  build commands in `builds`

Scope:

- standardize explicit selector naming to `--build-id` in the shared
  resolver-facing commands
- standardize inferred selector naming to `--latest`
- keep `--newest` as a hidden deprecated alias for `--latest` on
  `asc builds wait`
- update shared resolver validation/error text to use `--build-id`
- keep command taxonomy unchanged for now

Commands in scope:

- `asc builds wait`
- `asc builds dsyms`
- `asc builds info`
- `asc builds app get`
- `asc builds pre-release-version get`
- `asc builds icons list`
- `asc builds beta-app-review-submission get`
- `asc builds build-beta-detail get`
- `asc builds links view`
- `asc builds metrics beta-usages`
- shared resolver helpers in `internal/cli/builds/resolve_build.go`

Files expected in scope:

- `internal/cli/builds/resolve_build.go`
- `internal/cli/builds/builds_wait.go`
- `internal/cli/builds/builds_dsyms.go`
- `internal/cli/builds/builds_dsyms_test.go`
- `internal/cli/cmdtest/builds_wait_test.go`
- `internal/cli/cmdtest/builds_dsyms_test.go`
- `internal/cli/cmdtest/commands_test.go`
- help/example updates in `internal/cli/builds/builds_commands.go` if touched

Design note:

1. Command placement in taxonomy
   Keep commands under `asc builds ...`. This PR only fixes selector vocabulary
   and shared resolution language.

2. OpenAPI / endpoint impact
   No endpoint shape changes. Existing build lookup and build fetch endpoints are
   reused; only CLI selector vocabulary and resolver plumbing change.

3. UX shape
   Canonical selector forms become:
   - `--build-id BUILD_ID`
   - `--app APP --latest`
   - `--app APP --build-number NUM`

4. Backward-compatibility / deprecation impact
   This PR keeps the existing selector aliases working in the touched commands
   while warning toward `--build-id` / `--latest`. Removal stays deferred to a
   later cleanup PR closer to `1.0.0`.

5. RED -> GREEN test plan
   - update command/unit tests to expect `--build-id`
   - update wait tests to prefer `--latest` while keeping `--newest` warnings
   - implement flag/help/warning changes
   - run focused tests for builds wait/dsyms/selector validation

### PR 2: Make `builds info` Canonical

Status: in progress

Scope:

- add shared selector support to `asc builds info`
- remove `asc builds find`

Design note:

1. Command placement in taxonomy
   Keep the entry point at `asc builds info`; do not create a parallel
   replacement command for `find`.

2. OpenAPI / endpoint impact
   Reuse `GET /v1/builds` filters (`filter[app]`, `filter[version]`,
   `filter[preReleaseVersion.platform]`) plus `GET /v1/builds/{id}` /
   `GET /v1/builds/{id}/preReleaseVersion`. No new endpoint surface is needed.

3. UX shape
   Canonical selector forms become:
   - `--build-id BUILD_ID`
   - `--app APP --latest`
   - `--app APP --build-number NUM [--platform IOS]`

4. Backward-compatibility / deprecation impact
   `asc builds find` is removed in this PR because its behavior becomes a strict
   subset of `asc builds info`. The compatibility cost is deliberate to avoid
   carrying two public entry points for the same lookup job.

5. RED -> GREEN test plan
   - replace `builds find` tests with `builds info` selector coverage
   - update validation/exit-code expectations for app-scoped `builds info`
   - remove `builds find` from help and command registration
   - run focused selector tests, then full required checks

### PR 3: Replace `builds latest` With `builds next-number`

Status: planned

Scope:

- move `--next` behavior into `asc builds next-number`
- remove `asc builds latest` as a fetch command

### PR 4: Redesign `builds test-notes`

Status: planned

Scope:

- make `test-notes` build-scoped plus `--locale`
- replace build-related `--id` usage with `--build-id`
- optionally keep `--localization-id` as a low-level escape hatch

### PR 5: Legacy Removal + Remaining Read Commands

Status: planned

Scope:

- delete `beta-build-localizations`
- remove `builds test-notes get`
- standardize remaining read-oriented build commands on `--build-id`

## Command Target Shape

Examples for the end state:

```bash
asc builds info --build-id "BUILD_ID"
asc builds info --app "123" --latest
asc builds info --app "123" --build-number "42" --platform IOS

asc builds wait --app "123" --latest
asc builds dsyms --app "123" --build-number "42" --platform IOS

asc builds test-notes list --app "123" --latest
asc builds test-notes view --app "123" --latest --locale "en-US"
asc builds test-notes create --app "123" --latest --locale "en-US" --whats-new "..."
asc builds test-notes update --app "123" --latest --locale "en-US" --whats-new "..."
asc builds test-notes delete --app "123" --latest --locale "en-US" --confirm
```

## Notes

- `builds latest` remains in the repo during PR 1 only to keep the change set
  narrow.
- Mutating commands like `expire`, `update`, `add-groups`, `remove-groups`, and
  `individual-testers` should be reviewed separately before inheriting inferred
  build selection.

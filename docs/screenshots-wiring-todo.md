# Screenshots Wiring TODO

## Goal
- Introduce top-level `asc screenshots` and `asc video-previews` command surfaces.
- Remove `shots` and nested `assets screenshots` / `assets previews` user-facing paths (pre-1.0, no deprecation aliasing).
- Keep behavior parity while improving discoverability.

## Command Surface Design
- [ ] Finalize `asc screenshots` verb set and names:
  - [ ] Local workflow: `capture`, `frame`, `run`, `review-generate`, `review-open`, `review-approve`, `list-frame-devices`
  - [ ] App Store workflow: `list`, `sizes`, `upload`, `delete`
- [ ] Finalize `asc video-previews` verbs: `list`, `upload`, `delete`
- [ ] Confirm help grouping labels for mixed workflow (`LOCAL WORKFLOW` vs `APP STORE`).

## Wiring Changes
- [ ] Create top-level `screenshots` command package that wires:
  - [ ] Existing `shots` local automation handlers
  - [ ] Existing `assets screenshots` API handlers
- [ ] Create top-level `video-previews` command package that wires existing `assets previews` handlers.
- [ ] Update `internal/cli/registry/registry.go`:
  - [ ] Add `screenshots` and `video-previews`
  - [ ] Remove top-level `shots`
  - [ ] Remove top-level `assets` (or strip screenshot/preview subcommands from it, depending on final scope)
- [ ] Update root help grouping in `cmd/root_usage.go` for new top-level commands.

## Help + Docs Updates
- [ ] Update command examples/help strings in migrated command files (`ShortUsage`, `LongHelp`).
- [ ] Update `ASC.md`.
- [ ] Update `internal/cli/docs/templates/ASC.md`.
- [ ] Update `README.md` references to old command paths.

## Test Updates (Required)
- [ ] Update command-path assertions in `internal/cli/cmdtest/commands_test.go`:
  - [ ] Replace `assets screenshots ...` with `screenshots ...`
  - [ ] Replace `assets previews ...` with `video-previews ...`
  - [ ] Replace `shots ...` with `screenshots ...`
- [ ] Update dedicated screenshot cmd tests:
  - [ ] `shots_capture_test.go`
  - [ ] `shots_frame_test.go`
  - [ ] `shots_frames_list_devices_test.go`
  - [ ] `shots_review_generate_test.go`
  - [ ] `shots_review_open_approve_test.go`
  - [ ] `shots_run_test.go`
- [ ] Update `assets_screenshots_sizes_test.go` for new command path.
- [ ] Add regression tests to ensure removed old top-level commands are not shown in root help.
- [ ] Add/update root help tests to verify new grouped entries (`screenshots`, `video-previews`).

## Validation / Done Criteria
- [ ] `make format`
- [ ] `make lint`
- [ ] `ASC_BYPASS_KEYCHAIN=1 make test`
- [ ] Manual smoke:
  - [ ] `go run . --help` shows new top-level commands and expected groups
  - [ ] `go run . screenshots --help`
  - [ ] `go run . video-previews --help`

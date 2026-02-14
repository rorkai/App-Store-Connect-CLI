# Screenshots Wiring TODO

## Goal
- Introduce top-level `asc screenshots` and `asc video-previews` command surfaces.
- Remove `shots` and nested `assets screenshots` / `assets previews` user-facing paths (pre-1.0, no deprecation aliasing).
- Keep behavior parity while improving discoverability.

## Command Surface Design
- [x] Finalize `asc screenshots` verb set and names:
  - [x] Local workflow: `capture`, `frame`, `run`, `review-generate`, `review-open`, `review-approve`, `list-frame-devices`
  - [x] App Store workflow: `list`, `sizes`, `upload`, `delete`
- [x] Finalize `asc video-previews` verbs: `list`, `upload`, `delete`
- [x] Confirm help grouping labels for mixed workflow (`LOCAL WORKFLOW` vs `APP STORE`).

## Wiring Changes
- [x] Create top-level `screenshots` command package that wires:
  - [x] Existing `shots` local automation handlers
  - [x] Existing `assets screenshots` API handlers
- [x] Create top-level `video-previews` command package that wires existing `assets previews` handlers.
- [x] Update `internal/cli/registry/registry.go`:
  - [x] Add `screenshots` and `video-previews`
  - [x] Remove top-level `shots`
  - [x] Remove top-level `assets` (or strip screenshot/preview subcommands from it, depending on final scope)
- [x] Update root help grouping in `cmd/root_usage.go` for new top-level commands.

## Help + Docs Updates
- [x] Update command examples/help strings in migrated command files (`ShortUsage`, `LongHelp`).
- [x] Update `ASC.md`.
- [x] Update `internal/cli/docs/templates/ASC.md`.
- [x] Update `README.md` references to old command paths.

## Test Updates (Required)
- [x] Update command-path assertions in `internal/cli/cmdtest/commands_test.go`:
  - [x] Replace `assets screenshots ...` with `screenshots ...`
  - [x] Replace `assets previews ...` with `video-previews ...`
  - [x] Replace `shots ...` with `screenshots ...`
- [x] Update dedicated screenshot cmd tests:
  - [x] `shots_capture_test.go`
  - [x] `shots_frame_test.go`
  - [x] `shots_frames_list_devices_test.go`
  - [x] `shots_review_generate_test.go`
  - [x] `shots_review_open_approve_test.go`
  - [x] `shots_run_test.go`
- [x] Update `assets_screenshots_sizes_test.go` for new command path.
- [x] Add regression tests to ensure removed old top-level commands are not shown in root help.
- [x] Add/update root help tests to verify new grouped entries (`screenshots`, `video-previews`).

## Validation / Done Criteria
- [x] `make format`
- [x] `make lint`
- [x] `ASC_BYPASS_KEYCHAIN=1 make test`
- [x] Manual smoke:
  - [x] `go run . --help` shows new top-level commands and expected groups
  - [x] `go run . screenshots --help`
  - [x] `go run . video-previews --help`

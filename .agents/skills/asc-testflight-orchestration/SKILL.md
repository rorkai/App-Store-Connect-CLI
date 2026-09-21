---
name: asc-testflight-orchestration
description: Orchestrate TestFlight distribution, groups, testers, and What to Test notes using asc. Use when rolling out betas.
---

# asc TestFlight orchestration

Use this skill when managing TestFlight testers, groups, and build distribution.

## Export current config
- `asc testflight config export --app "APP_ID" --output "./testflight.yaml"`
- Include builds/testers:
  - `asc testflight config export --app "APP_ID" --output "./testflight.yaml" --include-builds --include-testers`

## Manage groups and testers
- Groups:
  - `asc testflight groups list --app "APP_ID" --paginate`
  - `asc testflight groups create --app "APP_ID" --name "Beta Testers"`
- Testers:
  - `asc testflight testers list --app "APP_ID" --paginate`
  - `asc testflight testers add --app "APP_ID" --email "tester@example.com" --group "Beta Testers"`
  - `asc testflight testers invite --app "APP_ID" --email "tester@example.com"`

## Distribute builds
- `asc builds add-groups --build-id "BUILD_ID" --group "GROUP_ID"`
- Remove from group:
  - `asc builds remove-groups --build-id "BUILD_ID" --group "GROUP_ID" --confirm`

## Inspect a build's groups
- `asc testflight groups list --build-id "BUILD_ID" --output table`
- The build lookup is experimental. It resolves the app, paginates the app's groups, and includes groups with all-build access.

## Upload without distribution
- `asc publish testflight --app "APP_ID" --ipa "./app.ipa" --upload-only --output json`
- Add `--wait` when the next step needs a processed build.
- Upload-only requires a new IPA or local Xcode build. Do not combine it with an existing `--build-id`, groups, tester notifications, test notes, or beta-review submission. `--build-number` is allowed as upload metadata, but it cannot be the only build input.

## What to Test notes
- `asc builds test-notes create --build-id "BUILD_ID" --locale "en-US" --whats-new "Test instructions"`
- `asc builds test-notes update --localization-id "LOCALIZATION_ID" --whats-new "Updated notes"`

## Notes
- Use `--paginate` on large groups/tester lists.
- Prefer IDs for deterministic operations; use the ID resolver skill when needed.

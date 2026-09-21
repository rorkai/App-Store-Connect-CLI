---
name: asc-build-lifecycle
description: Track build processing, find latest builds, and clean up old builds with asc. Use when managing build retention or waiting on processing.
---

# asc build lifecycle

Use this skill to manage build state, processing, and retention.

## Find the right build
- Latest build:
  - `asc builds info --app "APP_ID" --latest --version "1.2.3" --platform IOS`
- Next safe build number:
  - `asc builds next-build-number --app "APP_ID" --version "1.2.3" --platform IOS`
  - The command scans the full processed-build history and in-flight uploads, then returns one greater than the highest positive numeric build number. Blank or non-numeric processed build numbers are skipped with a warning; if no usable number remains, it falls back to `--initial-build-number`.
- Recent builds:
  - `asc builds list --app "APP_ID" --sort -uploadedDate --limit 10`

## Inspect processing state
- `asc builds info --build-id "BUILD_ID"`

## Distribution flows
- Prefer end-to-end:
  - `asc publish testflight --app "APP_ID" --ipa "./app.ipa" --group "GROUP_ID" --wait`
  - `asc publish appstore --app "APP_ID" --ipa "./app.ipa" --version "1.2.3" --wait --submit --confirm`

## Cleanup
- Preview expiration:
  - `asc builds expire-all --app "APP_ID" --older-than 90d --dry-run`
- Apply expiration:
  - `asc builds expire-all --app "APP_ID" --older-than 90d --confirm`
- Single build:
  - `asc builds expire --build-id "BUILD_ID" --confirm`

## Notes
- `asc builds upload` uploads and commits an IPA or PKG. Use `asc publish` when
  the workflow must also distribute to TestFlight or stage an App Store release.
- For long processing times, use `--wait`, `--poll-interval`, and `--timeout` where supported.

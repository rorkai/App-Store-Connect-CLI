# Version-resolution diagnostics

## Problem

Two App Store version failures gave callers nothing to recover with:

- Commands that resolve `--version` to an App Store version (`versions view`,
  `metadata pull/push`, `validate`, `review submit`, `localizations list`,
  `apps search-keywords set`, and the other users of the shared resolver)
  failed with `app store version not found for version "1.0" and platform
  "IOS"` and did not say which versions exist.
- `versions create` passed through Apple's 409 "You cannot create a new
  version of the App in the current state." without naming the version that
  already exists or is still in progress.

Agents followed both errors with `versions list` and help calls, and
sometimes rebuilt metadata from scratch.

## Decision

Both failures stay failures with the same error class, exit code, and first
line. Only on that error path, the CLI reads the app's versions,
`GET /v1/apps/{id}/appStoreVersions?limit=200` and its following pages,
filtered by platform when the command has one, and appends lines to the error.
Every page is read because the endpoint has no sort parameter and Apple does
not document its order, so one page could miss the newest or matching version.

- Not found: the newest 10 versions (version string, platform, state, ID),
  ordered by `createdDate`, with `... and N more` plus the
  `asc versions list --paginate` command when more exist. With a platform, it then suggests renaming the newest editable version
  (`asc versions update --version-id ID --version VERSION`) or, when none is
  editable, `asc versions create`. An app with no versions gets the create
  command.
- Create 409 from `versions create` (unless `--if-exists` resolved it),
  `release stage` or `publish appstore`: the version that already uses the
  string, with the `versions view` command and how to reuse it (`--if-exists`
  for `versions create`; the other two use it when run again, or ask for a
  new version number when it is live or no longer in progress), and the
  platform's versions that are not live yet, with the command that fits the
  newest one's state (rename an editable version, release one pending developer
  release, or inspect one in review). The text reports state; it does not claim
  which version caused Apple's rejection.

The read has a 5-second budget. If any page fails or the budget runs out, the
original error is returned unchanged. Ambiguity
errors, successful lookups, other create failures, and a failed `--if-exists`
read-back make no extra request.

## Output contract

Errors stay on stderr; stdout stays empty in every output mode, so JSON
consumers see no change. `release stage` already prints its step result on
failure, and its `error` carries the same lines. Apple-supplied fields are sanitized and bounded
per row, so a response cannot inject extra lines. `errors.Is` and
`errors.As` still reach the original `asc.ErrNotFound` or `*asc.APIError`.

## Verification

Shared tests replay recorded list bodies and Apple's captured 409 bodies:
ordering, bounding, editable and non-editable hints, all-platform listings,
state-specific create guidance, duplicate naming, sanitization, failed
diagnostic reads, and no reads for other failures. Black-box `cmd` tests
check exit codes, empty stdout, and stderr for `review submit` not-found and
`versions create` 409. `release stage` and `publish appstore` each have a test
that a refused create carries the lines.

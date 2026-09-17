# Idempotent writes: `--if-exists skip|update|fail`

## Problem

Telemetry over 30 days shows HTTP 409 as the single largest App Store Connect
API error class, about 4.7k distinct installs. The top offenders are create-style
commands that agents retry blindly after a partial failure: `pricing
availability create` (877 installs), `review items add` (872), `metadata push`
(596), `localizations update` (562), `review submissions-submit` (490), `review
details-create` (455), `versions create` (437), `subscriptions setup` (424),
`age-rating edit` (409), and `bundle-ids capabilities add` (398). The retry
runs into "already exists" and then fails with a non-zero exit even though the
desired end state is already present.

## Decision

Create-style commands gain one shared flag:

```
--if-exists fail|skip|update
```

- `fail` is the default and preserves today's behavior exactly: the 409 is
  returned as-is with the existing exit code.
- `skip` treats an existing resource as success. The command exits 0, leaves the
  resource unchanged, and reports that it already existed.
- `update` routes the same inputs to the corresponding update/PATCH call on the
  existing resource when one exists. Commands without a meaningful update
  (`bundle-ids capabilities add`, `review items add`) reject `update` as a
  usage error (exit 2) and document `skip` as the idempotent form.

Unknown values are usage errors (exit 2), validated before any HTTP request.

### Detecting "already exists"

A 409 alone is not proof of existence: Apple also uses 409 for state
transitions that are not idempotent-safe (`STATE_ERROR.*`), for relationship
rejections such as "You cannot create a new version of the App in the current
state" (`ENTITY_ERROR.RELATIONSHIP.INVALID` on `POST /v1/appStoreVersions`),
and for validation of unrelated attributes. The rule is therefore two-step and
keyed on the exact Apple error code:

1. The create request fails with HTTP 409 **and** at least one `errors[].code`
   in the response is one of the codes the command recorded as "already exists"
   (exact, case-insensitive match; a longer code such as
   `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.DIFFERENT_ACCOUNT` does not match).
   Every entry is matched, not only `errors[0]`: Apple can report several causes
   for one 409 and the existence cause is not always first. A duplicate
   `versionString` on `POST /v1/appStoreVersions` arrives as
   `ENTITY_ERROR.RELATIONSHIP.INVALID` followed by
   `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE` (verified live, see the table).
   `asc.APIError.AllCodes` carries the full list; `Code`, `Title` and `Detail`
   still come from `errors[0]`, so nothing changes about the message a failing
   command prints.
2. The CLI reads back the resource by its natural key (the same lookup a caller
   would use to find it), and only when that read returns the resource is the
   409 treated as "already exists".

Any other 409, or a matching 409 whose read-back finds nothing, is returned
unchanged. A 409 whose code is not on the list never triggers the read-back.

Error codes keyed on, per command:

| Command | Natural key read-back | Apple 409 code(s) accepted | Evidence |
| --- | --- | --- | --- |
| `versions create` | `GET /v1/apps/{id}/appStoreVersions?filter[versionString]=&filter[platform]=` | `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE` (source pointer `/data/attributes/versionString`, detail "The version number has been previously used.") | **Verified live** against app `6759231657` on 2026-09-15: re-creating the existing version string returns two errors, `errors[0]` = `ENTITY_ERROR.RELATIONSHIP.INVALID` ("You cannot create a new version of the App in the current state.", pointer `/data/relationships/app`) and `errors[1]` = the duplicate code above. A 409 carrying only the relationship rejection (a genuinely new version string the app cannot accept yet) has no duplicate code and keeps failing. |
| `review details-create` | `GET /v1/appStoreVersions/{id}/appStoreReviewDetail` | `STATE_ERROR.ALREADY_EXISTS`, `ENTITY_ERROR.RELATIONSHIP.INVALID`, `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`, `ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS` | **Verified live** against app `6759231657` on 2026-09-15: creating a detail for a version that already has one returns 409 `STATE_ERROR.ALREADY_EXISTS` ("Resource already exists." / "The given app version already has an existing review."). The relationship and duplicate-attribute codes are kept as defensive alternates; every other `STATE_ERROR.*` keeps failing, and the read-back is the decisive check. |
| `localizations create` / `metadata push` | `GET /v1/appStoreVersions/{id}/appStoreVersionLocalizations` filtered by locale | `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE` (detail "Entity with locale: ... already exists. Try updating.") | Detail text verified from developer forum threads 677931 and 776315; code to be confirmed with the PR2 fixture. |
| `localizations update` | `GET /v1/appStoreVersionLocalizations/{id}` or `GET /v1/appInfoLocalizations/{id}` after the PATCH | `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE` | Same recorded duplicate-locale body. `skip` prints that resource and does not apply fields. `update` sends the same field PATCH again. `STATE_ERROR` still fails. |
| `pricing availability create` | `GET /v1/apps/{id}/appAvailabilityV2` | `ENTITY_ERROR.RELATIONSHIP.INVALID`, `ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS` | To be confirmed with the PR3 fixture; the CLI already maps this conflict to "app availability already exists" in `web apps availability create`. |
| `bundle-ids capabilities add` | `GET /v1/bundleIds/{id}/bundleIdCapabilities` filtered by capability type | `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`, `ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS` | To be confirmed with the PR4 fixture. `ENTITY_ERROR.ATTRIBUTE.TYPE` (unsupported capability) is also a 409 and must keep failing. |
| `review items add` | `GET /v1/reviewSubmissions/{id}/items` filtered by the linked resource | `ENTITY_ERROR.RELATIONSHIP.INVALID`, `ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS` | To be confirmed with the PR4 fixture. `STATE_ERROR.*` (submission not editable) must keep failing. |

Other Apple existence codes seen in this repository's fixtures, kept for
reference when a later command needs them: bare `ENTITY_ERROR` with detail
"A device with number ... already exists on this team." (device registration),
and `ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS` on
`POST /iris/v1/inAppPurchaseSubmissions`.

The shared helper `shared.ResolveIfExistsConflict` implements the rule: it
requires `errors.Is(err, asc.ErrConflict)`, an `*asc.APIError` **any** of whose
codes is in the command's list (`shared.IsIfExistsConflict`, which walks
`AllCodes` and falls back to `Code` when a caller built the error by hand), and
a successful read-back. It never swallows a non-409 error or a 409 none of whose
codes is listed.

### Output contract

- Commands whose receipt is an exported camelCase struct in
  `internal/asc/output_*.go` (for example `versions create`) gain two additive
  fields: `alreadyExists` (bool, omitted when false) and `action`
  (`created`, `skipped`, or `updated`). Existing consumers see one new
  `"action":"created"` key on the unchanged success path.
- Commands that print Apple's envelope unmodified (for example `review
  details-create`) keep printing the envelope: on `skip` the existing resource's
  envelope from the read-back, on `update` the PATCH response. The envelope is
  not decorated, per the JSON output contract.
- In both cases `skip` and `update` write one diagnostic line to stderr, for
  example `review details-create: review detail DETAIL_ID already exists for
  version VERSION_ID; left unchanged (--if-exists skip)`, so table output on a
  TTY also shows what happened.

### Series

1. `if-exists-core`: shared flag and helpers, receipt fields, `versions create`
   (`update` routes to `PATCH /v1/appStoreVersions/{id}` with `--copyright` and
   `--release-type`; `--copy-metadata-from` still runs against the existing
   version, and because that copy PATCHes the existing version's localizations
   the receipt reports `updated` even when the version resource itself had
   nothing to PATCH), `review details-create` (`update` routes to
   `PATCH /v1/appStoreReviewDetails/{id}` with the same attributes).
2. `if-exists-localizations`: `localizations create`/`update` and
   `metadata push`.
3. `if-exists-pricing`: `pricing availability create` (`update` routes to the
   availability edit path).
4. `if-exists-capabilities`: `bundle-ids capabilities add` and `review items
   add` (`skip` only).

Not in this series, analyzed for follow-up: `review submissions-submit`
(409 `STATE_ERROR` when the submission is not in a submittable state or has no
items; not an existence conflict), `subscriptions setup` (composite command,
each step needs its own existence rule), `age-rating edit` (409
`STATE_ERROR` when the declaration is locked by an in-review version).

## Compatibility

No default changes. `--if-exists fail` is byte-for-byte today's behavior. The
new receipt fields are additive. Usage errors keep exit code 2 and empty
stdout. No interactive prompts.

## Verification

Each command has a RED `httptest` that replays Apple's real 409 body before the
implementation, then CLI-level coverage for `fail` (unchanged), `skip`
(read-back, exit 0, receipt, stderr line), `update` (PATCH with the same
inputs), a 409 whose read-back finds nothing (still fails), and an invalid
`--if-exists` value (exit 2 before any HTTP request).

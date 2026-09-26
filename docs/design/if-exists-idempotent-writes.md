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
  existing resource when one exists. When the invocation supplied nothing the
  update call can carry (`versions create` without `--copyright` or
  `--release-type`, `localizations create` with only `--locale`), `update`
  resolves like `skip` instead of sending an empty PATCH that a non-editable
  resource could reject. `bundle-ids capabilities add` routes `update` to
  the capability PATCH with `--settings`, the only input it can carry. A
  command without a meaningful update (`review items add`) rejects `update` as
  a usage error (exit 2) and documents `skip` as the idempotent form.

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
| `localizations create` | `GET /v1/appStoreVersions/{id}/appStoreVersionLocalizations` matched on locale, then `GET /v1/appStoreVersionLocalizations/{id}` so Apple's own single-resource envelope is what gets printed | `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE` (pointer `/data/attributes/locale`, detail "Entity with locale: ... already exists. Try updating.") | **Verified live** against disposable app `6759231657` on 2026-09-15: re-creating an existing `en-US` locale returned HTTP 409 with this code and pointer. The exact-locale read-back found the existing resource; `skip` and `update` with no metadata both exited 0 without a PATCH. |
| `pricing availability create` | `GET /v1/apps/{id}/appAvailabilityV2` | `ENTITY_ERROR.RELATIONSHIP.INVALID` (pointer `/data/relationships/app`, detail "An 'appAvailabilities' with a relationship to 'apps' with id '...' already exists."), `ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS` (defensive alternate, not observed) | **Verified live** against app `6759231657` on 2026-09-15 and 2026-09-25: re-creating the existing availability returns 409 with the relationship code and detail above in `errors[0]`; when the request omits catalog territories Apple appends one bootstrap-style "expects an included resource with type 'territories'" error per omitted territory after it. **This code is ambiguous on this endpoint**: Apple also returns it for the public-API bootstrap rejection, which is classified first (by the `territoryAvailabilities.territory` detail in `errors[0]`) and keeps its own remediation. The read-back is decisive either way, since the bootstrap rejection creates nothing. A create with no `territoryAvailabilities` at all is `ENTITY_ERROR.RELATIONSHIP.REQUIRED`, not an existence conflict, and keeps failing. |
| `bundle-ids capabilities add` | `GET /v1/bundleIds/{id}/bundleIdCapabilities`, paginated, matched case-insensitively on `capabilityType` | `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`, `ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS` (both defensive, not observed) | **Checked live** against app `6759231657`'s bundle ID on 2026-09-15 and 2026-09-25: re-adding an enabled API-creatable capability (`IN_APP_PURCHASE`) returns HTTP 201 with the existing resource (id `<bundleId>_<capabilityType>`) and changes nothing, so no existence 409 could be reproduced and `--if-exists` does not engage there. The read-back is the decisive check if Apple ever reports one. `ENTITY_ERROR.ATTRIBUTE.TYPE` (recorded live for `PRIVATE_CLOUD_COMPUTE`, which the API cannot create even though it was already enabled through the portal) is also a 409, is not on the list, and keeps failing: `--if-exists skip` does not rescue it. |
| `review items add` | `GET /v1/reviewSubmissions/{id}/items?include=<relationship>`, paginated, matched on the linked resource ID | `ENTITY_ERROR.RELATIONSHIP.INVALID`, `ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS`, `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE` (defensive, not observed) | **Not reproducible live**: a review submission created for the test cannot be canceled while `READY_FOR_REVIEW` ("Resource is not in cancellable state"), and the disposable app has no reviewable item to attach. Live on 2026-09-25 an unreviewable version returned 409 `STATE_ERROR.ENTITY_STATE_INVALID`, which is not on the list and keeps failing without a read-back. `include=` is required, not `fields[]`: with `fields[]` alone Apple returns items carrying `links` only and no `relationships` key, as `internal/cli/submit/submit_create.go` already documents. A relationship pointer can also be non-nil with `"data":null`, so a real ID must match, and an item whose `state` is `REMOVED` is historical (the resource is detached) so it is skipped rather than taken as proof of presence. `STATE_ERROR.*` (submission not editable or already submitted) keeps failing. |

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
  fields for explicit `skip` and `update` modes: `alreadyExists` (bool, omitted
  when false) and `action` (`created`, `skipped`, or `updated`). Default and
  explicit `fail` mode preserve the historical success output byte-for-byte.
- Commands that print Apple's envelope unmodified (for example `review
  details-create`) keep printing the envelope: on `skip` the existing resource's
  envelope from the read-back, on `update` the PATCH response. The envelope is
  not decorated, per the JSON output contract.
- A command whose success path emits advice about the resource it just created
  (for example `localizations create`'s submit-readiness warning) must suppress
  that advice when `--if-exists` resolved a duplicate: nothing was created, and
  the existing resource may already carry the fields the caller omitted.
- In both cases `skip` and `update` write one diagnostic line to stderr, for
  example `review details-create: review detail DETAIL_ID already exists for
  version VERSION_ID; left unchanged (--if-exists skip)`, so table output on a
  TTY also shows what happened.

### `metadata push`

`metadata push` (and its `metadata apply` alias) is a bulk reconciler rather
than a single create, so `--if-exists` applies per locale and only to the two
create-style writes it issues:

- `POST /v1/appStoreVersionLocalizations` (version scope)
- `POST /v1/appInfoLocalizations` (app-info scope)

Nothing else it issues can produce an existence conflict: the other writes are
`PATCH`es on a resolved localization ID and a `DELETE` behind
`--allow-deletes --confirm`. It also never creates the version, so `versions
create`'s duplicate-`versionString` 409 is out of reach here: the version is
resolved with `GET /v1/apps/{id}/appStoreVersions` and a missing version is an
error before any mutation.

A duplicate-locale 409 is rarer than the raw telemetry count suggests, because
every mutation already runs through `shared.RunReconciledMutation` with a
field-matching read-back. When the locale exists *and* already carries the
planned fields, today's code reconciles the conflict into `action: reconcile`
and exits 0. The 409 survives exactly when the locale is absent from the plan
read and present at apply time with content that differs from the plan, which
is the retry-after-partial-failure shape the telemetry is made of. That is the
case `--if-exists` covers:

- `skip` records the existing localization untouched and contributes to a new
  `skipped` counter instead of `succeeded`:
  `{"scope":"version","locale":"ja","action":"create","status":"skipped","localizationId":"loc-ja","alreadyExists":true,"ifExists":"skip"}`.
- `update` routes the same desired fields to
  `PATCH /v1/appStoreVersionLocalizations/{id}` or
  `PATCH /v1/appInfoLocalizations/{id}` on the localization the read-back
  found, then records `action: update` with `alreadyExists: true`. The body
  carries only the fields the local file set, plus its explicit `null`
  clears as JSON `null`; `locale` is immutable and is never sent. A create
  cannot carry a clear, so the create's read-back never counts as a match when
  the file clears a field: the existing localization is re-read against the
  full desired state, clears included, before deciding whether to PATCH.
  Because the plan lists such a locale as a create rather than as a "field
  cleared locally" update, `--if-exists update` requires `--confirm` up front,
  before any request, whenever a locale planned as a create also clears a
  field. `skip` never applies a clear and needs no confirmation.

App-info localization creates require `name`, but a patch-only file can still
be applied when another writer creates that locale after the initial read:
with `skip` or `update`, apply re-reads the locale before rejecting the missing
name. A found locale follows the selected mode; a still-missing locale fails
before any mutation. Planning and the default `fail` mode keep the create
prerequisite check.

The existence read-back reuses the command's own natural-key lookups
(`readBackVersionLocalization` / `readBackAppInfoLocalization`) with no desired
fields, so it asks only whether the locale is present in
`GET /v1/appStoreVersions/{id}/appStoreVersionLocalizations` or
`GET /v1/appInfos/{id}/appInfoLocalizations` (paginated, limit 200). A
read-back that finds nothing leaves the 409 unchanged, and a 409 whose Apple
code is not `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE` never triggers it, and
because `shared.IsIfExistsConflict` walks every entry in Apple's `errors[]`
array, a duplicate code reported after a relationship rejection is still
matched. Each resolved conflict writes one stderr line, for example
`metadata push: version localization loc-ja for locale ja already exists;
updated in place (--if-exists update)`.

Both conflict bodies were **verified live** against disposable app
`6759231657` on 2026-09-25, and the test fixtures replay them verbatim. Both
creates return HTTP 409 `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE` with pointer
`/data/attributes/locale`; only the detail differs. The version scope says
"Entity with locale: ja already exists. Try updating.", and the app-info scope
says "An 'appInfoLocalizations' with a 'locale' of 'ja' already exists." A
duplicate version create whose body also fails validation carries a second
code, for example `ENTITY_ERROR.ATTRIBUTE.INVALID.TOO_SHORT`, after the
duplicate code.

This section is the authority on the code for `metadata push`; it settles the
"code to be confirmed with the PR2 fixture" note left on the shared
`localizations create` / `update` / `metadata push` table row above, which the
`localizations create` PR rewrites for its own half.

#### Receipt change

`ApplyAction` and `PushPlanResult` are the push command's own exported
camelCase receipt, printed through the renderers registered in `push.go`
rather than through `internal/asc/output_*.go`. The change is additive:

| Field | Type | When present |
| --- | --- | --- |
| `actions[].alreadyExists` | bool | Only when `--if-exists` resolved a create conflict for that locale |
| `actions[].ifExists` | string | Same, carrying the mode that resolved it (`skip` or `update`) |
| `skipped` | int | Only when at least one action was skipped |

`actions[].status` gains one further value, `skipped`, reusing
`asc.IdempotentWriteActionSkipped` so the vocabulary matches the shared
`IdempotentWriteReceipt`. No field is removed or renamed, every new key is
`omitempty`, and a run under the default `--if-exists fail` produces
byte-identical JSON; the table and markdown renderers print a `Skipped:` line
only when the counter is non-zero.

A skipped or updated duplicate suppresses the submit-readiness "was created"
warning for that locale. In the default `fail` mode, a duplicate 409 on the
first create attempt is also classified as pre-existing when its read-back
confirms the planned fields. By contrast, after an ambiguous create attempt is
replayed, a later duplicate 409 and matching read-back remain an ordinary
`action: reconcile`: the earlier attempt may have created the locale, so its
submit-readiness warning is retained.

#### Review-plan binding

`metadata plan` / `metadata approve` / `metadata apply --review-dir` bind an
apply to the exact reviewed options through a plan hash (see
`docs/design/metadata-approval-workflow.md`). The conflict policy is part of
those options: `--if-exists update` can PATCH a localization the reviewer never
saw in the plan, so the normalized mode is recorded in `options.ifExists` and
hashed. `metadata plan` therefore takes `--if-exists` as well, and applying an
approved plan with a different mode fails with the existing
"approved metadata plan drifted" usage error. `fail` renders as the empty
string and is omitted, so plan artifacts written before this flag existed keep
their hash and stay approvable.

### Series

1. `if-exists-core`: shared flag and helpers, receipt fields, `versions create`
   (`update` routes to `PATCH /v1/appStoreVersions/{id}` with `--copyright` and
   `--release-type`; `--copy-metadata-from` still runs against the existing
   version, and because that copy PATCHes the existing version's localizations
   the receipt reports `updated` even when the version resource itself had
   nothing to PATCH), `review details-create` (`update` routes to
   `PATCH /v1/appStoreReviewDetails/{id}` with the same attributes).
2. `if-exists-localizations`: `localizations create` (`update` routes to
   `PATCH /v1/appStoreVersionLocalizations/{id}` with the same field flags).
   `localizations update` is deliberately excluded: it is a pure PATCH that
   already resolves the localization by locale and fails with its own
   non-HTTP "no existing localization found" error when the locale is absent,
   so its remaining 409s are state conflicts with nothing to key on.
3. `if-exists-pricing`: `pricing availability create`. `update` routes to the
   same code path as `pricing availability edit`, through the exported
   `shared.ApplyTerritoryAvailabilityUpdate`. Apple exposes no update operation
   for `availableInNewTerritories`, so on `update` that flag is only verified
   against the existing policy and a mismatch fails; `--territory` and
   `--available` are applied. When every requested territory already matches,
   `update` issues no PATCH and its diagnostic says the record was left
   unchanged. `skip` still pays for the territory-catalog fetch the create
   performs before the POST.
4. `if-exists-capabilities`: `bundle-ids capabilities add` (`skip`, and
   `update` routing `--settings` to `PATCH /v1/bundleIdCapabilities/{id}`;
   with no `--settings` there is nothing to apply, so `update` behaves like
   `skip`) and `review items add` (`skip` only, because a submission item
   carries no inputs to re-apply, so `update` is rejected as a usage error).

   Neither resource has a detail endpoint: the OpenAPI snapshot exposes only
   POST, PATCH and DELETE for `/v1/bundleIdCapabilities/{id}` and
   `/v1/reviewSubmissionItems/{id}`. The collection item is therefore the only
   representation Apple offers, and the printed single-resource envelope is
   built around Apple's own resource object rather than re-read. Where a detail
   endpoint does exist (`localizations create`, `review details-create`) the
   convention re-reads instead of building an envelope.

`metadata push` moved to follow-up: every mutation in `push.go` already runs
through `shared.RunReconciledMutation` with a field-matching read-back, so a
duplicate-locale 409 only survives when the existing remote content differs from
the plan. Making that configurable means threading the mode through
`applyVersionLocalizations` and `applyAppInfoLocalizations` and adding a
`skipped` status to the `ApplyAction` receipt, which is a receipt-schema change
on a bulk command and belongs in its own change.

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

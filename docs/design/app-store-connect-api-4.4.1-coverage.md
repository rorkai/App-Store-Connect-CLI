# App Store Connect API 4.4.1 coverage ledger

## Objective

Deliver production-ready CLI support for every behavior added or changed by App
Store Connect API 4.4.1 without silently changing the ID or lifecycle semantics
of existing stable commands. Schema-only support is acceptable only when the
ledger below records an evidence-backed reason that a first-class command is not
useful.

The work is split into stacked, independently reviewable pull requests based on
the 4.4.1 schema update. Each behavior PR must use RED-GREEN tests, verify a
freshly built binary, and pass the complete repository gate before review.

## Source contract

The coverage ledger is derived from three independent views of the contract:

1. Apple's App Store Connect API 4.4.1 release notes.
2. Apple's official OpenAPI zip, independently downloaded and compared
   byte-for-byte with `docs/openapi/latest.json`.
3. A semantic diff from the original repository 4.4 snapshot commit to 4.4.1,
   plus a second diff from the immediate pre-PR repository snapshot so work
   already reconciled after the 4.4 import is not counted twice.

Reproducible inputs:

| Input | Value |
| --- | --- |
| Official source artifact | `https://developer.apple.com/sample-code/app-store-connect/app-store-connect-openapi-specification.zip` |
| Downloaded zip SHA-256 | `9386762084aa7156a9d5aab20526daf8d4ca423ddaebb0b3fffd2ef6fd836370` |
| Extracted filename | `openapi.oas (2).json` |
| Extracted JSON SHA-256 | `ed0202ef37155b9334772482d2ea0be688c3046b284c895bcbea5455fbe54fd8` |
| Repository 4.4.1 JSON SHA-256 | `ed0202ef37155b9334772482d2ea0be688c3046b284c895bcbea5455fbe54fd8` |
| Original 4.4 baseline commit | `d465bea0c9563e415da8989b284f1810173b073e` |
| Original 4.4 JSON SHA-256 | `eb33a4909309c75c5f4a24e2a41db9bb18df02c4b2113c5b1d6e1eed4ce4c891` |
| Immediate pre-4.4.1 base | `839c4da6db3678ecbab5cf1db6d78b4b8c486957` |

Verified snapshot facts:

| Contract item | 4.4 | 4.4.1 | Delta |
| --- | ---: | ---: | ---: |
| Paths | 929 | 966 | +37 |
| Operations | 1,216 | 1,263 | +47 |
| Component schemas | 1,346 | 1,393 | +47 |
| Removed operations | - | - | 0 |
| Removed schemas | - | - | 0 |
| Modified existing operations from original 4.4 | - | - | 102 |
| Modified existing schemas from original 4.4 | - | - | 61 |
| Unchanged operations transitively affected by modified schemas | - | - | 71 |
| Schema-mediated operations with behavior work remaining at the immediate pre-PR base | - | - | 40 |
| Schema-mediated operations already reconciled after the 4.4 import | - | - | 31 |
| Still-different operations at the immediate pre-PR base | - | - | 50 |
| Still-different schemas at the immediate pre-PR base | - | - | 17 |
| Operation changes already reconciled after the 4.4 import | - | - | 52 |
| Schema changes already reconciled after the 4.4 import | - | - | 44 |

The semantic diff changes only `info.version`, `paths`, and
`components.schemas`; no other component category, security scheme, or
top-level contract changes.

## Current effort and status

The CLI pull requests landed sequentially after exact-head audit. Rows marked
`main`-gated passed their post-merge integration, Govulncheck, and CodeQL
workflows before the next behavior PR advanced. Open work is identified
separately and has no invented landed `main` commit:

| Scope | Pull request | Reference head | Landed `main` commit | Status |
| --- | --- | --- | --- | --- |
| Relationship-aware schema discovery and stale-index enforcement | #1776 | `aaa9b62d` | `893c840e` | Merged and `main`-gated |
| IAP versions, v2 localizations/images, compatibility, and docs | #1777 | `ee40c7b3` | `bd453af4` | Merged and `main`-gated |
| Age-rating social-media fields and adjusted equalizations | #1778 | `39284b0a` | `f6be93f6` | Merged and `main`-gated |
| Subscription versions, v2 localizations/images, compatibility, and docs | #1779 | `c8f3ab52` | `0349e397` | Merged and `main`-gated |
| Subscription-group versions, v2 localizations, compatibility, and docs | #1780 | `48d04e0b` | `8cbcdadc` | Merged and `main`-gated |
| Cross-cutting review-submission version items and migration notes | #1781 | `aba35e0a` | `eea98d3a` | Merged and `main`-gated |
| Age-rating dependency validation | #1782 | `3dfd15a0` | `b92e5605` | Merged and `main`-gated |
| Adjusted-equalization required filters | #1783 | `a06c935c` | `6f11d6aa` | Merged and `main`-gated |
| Positional-argument and usage validation | #1784 | `fc5f0504` | `1cc9face` | Merged and `main`-gated |
| Plural relationship-limit compatibility | #1785 | `da030480` | `b01d4d70` | Merged and `main`-gated |
| Legacy 4.4.1 resource deprecation transition | #1786 | `1ef32153` | `a00985b2` | Merged and `main`-gated |
| Deprecated IAP submit discoverability | #1787 | `04b52a62` | `38fa9b5f` | Merged and `main`-gated |
| Transitive age-rating dependency closure | #1788 | `08003c38` | `9282e82d` | Merged and `main`-gated |
| Final 4.4.1 coverage ledger | #1789 | `51b3a962` | `d6d8d94b` | Merged and `main`-gated; documentation only |
| Subscription localization delete-confirmation coverage | #1790 | `49eda126` | `73466720` | Merged; test only |
| Hardened public 4.4.1 command workflows | #1791 | `e9c2a0dc` | `e902d375` | Merged; documentation only; current #1792 base |
| Seven-property nullable request fidelity | #1792 | `db3657b6` implementation head | - | Open and pre-merge; no landed `main` SHA |
| External ASC workflow skills | rorkai/app-store-connect-cli-skills#50 | `61b9634` | `e5561a9` | Merged; landed tree revalidated |

The hard audit fixed contract gaps beyond the initial six implementation PRs:
endpoint-exact fields, includes, sparse fields, and relationship limits; opaque
continuation URLs; next-aware argument
exclusivity; review-response links and metadata; positional-argument rejection;
age-rating prerequisite declarations; adjusted-equalization required filters;
exact deprecation warnings and migration guidance; and rendered parent-help
discoverability for all 29 deprecated leaves. A post-merge thread audit found
the final transitive age-rating contradiction omitted by #1782; #1788 added
both sparse-update regression cases, landed green, and closed that thread.

The later cross-verification found one remaining typed-request gap: seven
properties marked optional and nullable by OpenAPI could represent omission and
a value, but not explicit JSON `null`. #1792 is the first implementation to
preserve all three states for `socialMedia`, `socialMediaAgeRestricted`, the two
v2 localization `description` properties, v2 group-localization
`customAppName`, and the two v2 image `uploaded` properties. Its exact audited
implementation head is
`db3657b6c6e88ebbf7f0da59137c92df99931e59`. That head also preserves the
legacy omission behavior for a whitespace-only `customAppName` while retaining
explicit JSON `null`. This documentation follow-up is a later commit on the same
open PR, so `db3657b6` is deliberately recorded as the
implementation head rather than misrepresented as a self-referential final PR
head.

The last fully merged behavior integration before #1792 is `main` commit
`9282e82d1feebddaaaa3285c658e6960b8a500f3`; #1789 then landed this coverage
ledger at `d6d8d94b`, test-only #1790 landed at `73466720`, and docs-only #1791
produced the current #1792 base `e902d375`. The 37-path, 47-operation, 47-schema, 102-direct, 71-transitive,
173-contract, 61-modified-schema, and 9-addition/7-deprecation counts are
unchanged. The recursive built-help comparison from `839c4da6` to `9282e82d`
found 48 added and 52 changed leaf paths, 100 affected paths total, with zero
removals; it is historical help-surface evidence, not proof of #1792's nullable
request encoding.

## Live App Store Connect verification

All manual commands used `ASC_BYPASS_KEYCHAIN=1` and the disposable app
`6759231657`. The exact 4.4.1 path delta was exercised against the live service:

| Domain | Operations | Live result |
| --- | ---: | --- |
| IAP versions, localizations, and images | 18 | All added operations received a successful live response |
| Subscription versions, localizations, and images | 18 | All added operations received a successful live response |
| Subscription-group versions and localizations | 10 | All added operations received a successful live response |
| Adjusted equalizations | 1 | Successful with the required upfront price point and plan type |
| **Total** | **47** | **29 GET, 8 POST, 5 PATCH, and 5 DELETE operations on 37 paths** |

The live run covered version creation and readback, localization CRUD, complete
image reserve/upload/commit/read/delete lifecycles, relationship endpoints, and
pagination. A subscription-group version review item was added, updated, and
removed. IAP and subscription version-item attempts reached Apple and returned
the expected readiness rejection because the throwaway products lacked review
prerequisites; no review submission was submitted. All four current
`READY_FOR_REVIEW` submissions contained zero items at closeout.

Live behavior refined four schema-level assumptions:

- Adjusted equalizations succeed only when both an upfront price point and
  `--plan-type MONTHLY` are supplied. #1783 now enforces both before HTTP.
- Setting `socialMediaAgeRestricted=true` required
  `userGeneratedContent=true`, `ageAssurance=true`, and `socialMedia=true`.
  #1782 and #1788 reject all 25 sparse flag combinations that are provably
  contradictory without reading stored state. All four attributes were
  restored to `false` afterward.
- Apple rejects empty and explicit JSON-null descriptions for IAP-version and
  subscription-version localizations even though OpenAPI marks the attribute
  nullable. Non-empty PATCH requests succeed; the CLI retains schema-correct
  encoding while the docs and skills record the live restriction.
- Five retained validator v1 reads returned the same IDs for v2-created group
  localizations, subscription localizations, and subscription images. A runtime
  validator migration was therefore a live-verified no-op, not an assumption.

PR #1792 makes no new live explicit-null claim. It adds typed-client fidelity and
table-driven omit/value/null encoding tests only. The existing CLI commands
continue to send the same concrete values or omissions as before; they do not
gain a new clear/null flag. No explicit-null request for the age-rating, group
localization, or image-upload fields was sent to App Store Connect for #1792,
and the earlier live rejection of null localization descriptions remains the
only live null evidence.

Cleanup was verified by ID. Image resources, secondary localizations, review
items, and temporary group-version resources were deleted where the API permits
it. All four review submissions were empty at closeout.

| Resource | Cleanup result |
| --- | --- |
| IAP parent `6791819283` | Deleted |
| IAP version `07d9113f-2a96-43d0-8399-914deeaa49d4` | Remains orphaned; Apple exposes no version DELETE and parent deletion did not cascade |
| IAP localization `0073ad1d-879d-41f4-bb7d-49c73f0479b5` | Remains because Apple rejects deletion of the final required localization |
| Subscription group `22243347` and subscription `6791819604` | Deleted |
| Subscription version `6f861af5-23c9-4e3c-807a-9879220c051f` | Remains orphaned |
| Subscription localization `9fc52d69-2eed-440c-8053-0646de35daa8` | Remains because it is the final required localization |
| Group `22243440` and all version/localization descendants | Deleted or cascaded; verified gone |
| Validator-test group `22243740` and its group-version/group-localization descendants | Deleted; verified gone |
| Validator-test subscription `6791895240` | Deleted |
| Subscription version `1dcba57e-f03f-49e0-a24e-d048fb6dd479` | Remains orphaned |
| Subscription localization `67326a94-973a-4061-a991-dc7061018232` | Remains because it is the final required localization |

These six retained version/localization resources are confined to the disposable
app. No presigned upload URL, credential, or non-disposable app data is recorded.

After every behavior and workflow-skill change was merged, a final read-only
smoke with the locally built CLI re-read app `6759231657`, its age-rating
declaration, all review submissions, and every submission's items. The app read
succeeded; `userGeneratedContent`, `ageAssurance`, `socialMedia`, and
`socialMediaAgeRestricted` were all `false`; and each of the four submissions
returned no items. A separate independent audit repeated the age-rating,
submission-item, and retained-resource reads with the same results.

## Definition of done

- Every added operation below is implemented and tested, or marked schema-only
  with a concrete rationale.
- Every one of the 102 modified existing operations is classified as a new
  query/response behavior, a deprecation reversal, or a change already covered
  before the schema PR.
- All 71 schema-mediated operation-contract changes that are not direct
  path-item diffs are classified separately. Two change request contracts; all
  71 change response contracts through referenced schemas.
- All 47 added and 61 modified schemas decode and encode through typed models
  where user-facing behavior depends on them, or have an explicit schema-only
  disposition.
- The three new version resource types can be created, listed, viewed, and
  submitted through discoverable CLI commands.
- Version-scoped localization and image workflows support create, read, update,
  delete, pagination, and uploads where the API permits them.
- Existing product-ID and group-ID commands retain their current behavior until
  they have an explicit deprecation warning, migration command, and transition
  tests. A version ID is never silently substituted for a product or group ID.
- `asc schema` exposes relationship fields needed to construct relationship-only
  requests, and CI fails when either generated schema index is stale.
- Command documentation, API notes, migration guidance, and external workflow
  skills reflect the final command surface.
- Focused tests, adjacent tests, built-binary checks, the full repository gate,
  GitHub checks, and appropriate live verification are green on each latest PR
  head.

## Added operation ledger

### In-app purchase versions and version-scoped metadata: 18

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/inAppPurchaseVersions` | Create a version for an IAP relationship | Implemented typed command | #1777 | `ee40c7b3`; HTTP body and built-command tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}` | View a version | Implemented typed command | #1777 | `ee40c7b3`; HTTP query and built-command tests |
| `GET` | `/v2/inAppPurchases/{id}/versions` | List related versions with pagination | Implemented typed command | #1777 | `ee40c7b3`; exact query and pagination tests |
| `GET` | `/v2/inAppPurchases/{id}/relationships/versions` | List version linkages | Implemented typed client/command | #1777 | `ee40c7b3`; linkage response tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/localizations` | List version localizations | Implemented typed command | #1777 | `ee40c7b3`; exact path/query tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/localizations` | List localization linkages | Implemented typed client/command | #1777 | `ee40c7b3`; linkage response tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/image` | Get the singular review image | Implemented typed command | #1777 | `ee40c7b3`; singular response tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/image` | Get singular image linkage | Implemented typed client/command | #1777 | `ee40c7b3`; linkage response tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/images` | List review images | Implemented typed command | #1777 | `ee40c7b3`; list and pagination tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/images` | List image linkages | Implemented typed client/command | #1777 | `ee40c7b3`; linkage response tests |
| `POST` | `/v2/inAppPurchaseLocalizations` | Create a version-scoped localization | Implemented typed command | #1777 | `ee40c7b3`; exact create payload tests |
| `GET` | `/v2/inAppPurchaseLocalizations/{id}` | View a localization | Implemented typed command | #1777 | `ee40c7b3`; detail response tests |
| `PATCH` | `/v2/inAppPurchaseLocalizations/{id}` | Update a localization | Implemented typed command | #1777 | `ee40c7b3`; omitted/value/null payload tests |
| `DELETE` | `/v2/inAppPurchaseLocalizations/{id}` | Delete a localization with confirmation | Implemented typed command | #1777 | `ee40c7b3`; confirmation and HTTP tests |
| `POST` | `/v2/inAppPurchaseImages` | Reserve and upload a version-scoped image | Implemented upload command | #1777 | `ee40c7b3`; reserve/upload lifecycle tests |
| `GET` | `/v2/inAppPurchaseImages/{id}` | View an image and upload state | Implemented typed command | #1777 | `ee40c7b3`; detail response tests |
| `PATCH` | `/v2/inAppPurchaseImages/{id}` | Commit uploaded parts | Implemented upload command | #1777 | `ee40c7b3`; checksum and commit tests |
| `DELETE` | `/v2/inAppPurchaseImages/{id}` | Delete an image with confirmation | Implemented typed command | #1777 | `ee40c7b3`; confirmation and HTTP tests |

The review-submission relationship for `inAppPurchaseVersion` modifies the
existing `/v1/reviewSubmissionItems` operation rather than adding another path.

### Subscription versions and version-scoped metadata: 18

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/subscriptionVersions` | Create a version for a subscription relationship | Implemented typed command | #1779 | `c8f3ab52`; HTTP body and built-command tests |
| `GET` | `/v1/subscriptionVersions/{id}` | View a version | Implemented typed command | #1779 | `c8f3ab52`; HTTP query and built-command tests |
| `GET` | `/v1/subscriptions/{id}/versions` | List related versions with pagination | Implemented typed command | #1779 | `c8f3ab52`; exact query and pagination tests |
| `GET` | `/v1/subscriptions/{id}/relationships/versions` | List version linkages | Implemented typed client/command | #1779 | `c8f3ab52`; linkage response tests |
| `GET` | `/v1/subscriptionVersions/{id}/localizations` | List version localizations | Implemented typed command | #1779 | `c8f3ab52`; exact path/query tests |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/localizations` | List localization linkages | Implemented typed client/command | #1779 | `c8f3ab52`; linkage response tests |
| `GET` | `/v1/subscriptionVersions/{id}/image` | Get the singular promotional image | Implemented typed command | #1779 | `c8f3ab52`; singular response tests |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/image` | Get singular image linkage | Implemented typed client/command | #1779 | `c8f3ab52`; linkage response tests |
| `GET` | `/v1/subscriptionVersions/{id}/images` | List promotional images | Implemented typed command | #1779 | `c8f3ab52`; list and pagination tests |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/images` | List image linkages | Implemented typed client/command | #1779 | `c8f3ab52`; linkage response tests |
| `POST` | `/v2/subscriptionLocalizations` | Create a version-scoped localization | Implemented typed command | #1779 | `c8f3ab52`; exact create payload tests |
| `GET` | `/v2/subscriptionLocalizations/{id}` | View a localization | Implemented typed command | #1779 | `c8f3ab52`; detail response tests |
| `PATCH` | `/v2/subscriptionLocalizations/{id}` | Update a localization | Implemented typed command | #1779 | `c8f3ab52`; omitted/value/null payload tests |
| `DELETE` | `/v2/subscriptionLocalizations/{id}` | Delete a localization with confirmation | Implemented typed command | #1779 | `c8f3ab52`; confirmation and HTTP tests |
| `POST` | `/v2/subscriptionImages` | Reserve and upload a version-scoped image | Implemented upload command | #1779 | `c8f3ab52`; reserve/upload lifecycle tests |
| `GET` | `/v2/subscriptionImages/{id}` | View an image and upload state | Implemented typed command | #1779 | `c8f3ab52`; detail response tests |
| `PATCH` | `/v2/subscriptionImages/{id}` | Commit uploaded parts | Implemented upload command | #1779 | `c8f3ab52`; checksum and commit tests |
| `DELETE` | `/v2/subscriptionImages/{id}` | Delete an image with confirmation | Implemented typed command | #1779 | `c8f3ab52`; confirmation and HTTP tests |

The review-submission relationship for `subscriptionVersion` modifies the
existing `/v1/reviewSubmissionItems` operation.

### Subscription-group versions and localizations: 10

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/subscriptionGroupVersions` | Create a version for a group relationship | Implemented typed command | #1780 | `48d04e0b`; HTTP body and built-command tests |
| `GET` | `/v1/subscriptionGroupVersions/{id}` | View a version | Implemented typed command | #1780 | `48d04e0b`; HTTP query and built-command tests |
| `GET` | `/v1/subscriptionGroups/{id}/versions` | List related versions with pagination | Implemented typed command | #1780 | `48d04e0b`; exact query, owner/next validation, and pagination tests |
| `GET` | `/v1/subscriptionGroups/{id}/relationships/versions` | List version linkages | Implemented typed client/command | #1780 | `48d04e0b`; owner/next validation and linkage response tests |
| `GET` | `/v1/subscriptionGroupVersions/{id}/localizations` | List version localizations | Implemented typed command | #1780 | `48d04e0b`; exact path/query and owner/next validation tests |
| `GET` | `/v1/subscriptionGroupVersions/{id}/relationships/localizations` | List localization linkages | Implemented typed client/command | #1780 | `48d04e0b`; owner/next validation and linkage response tests |
| `POST` | `/v2/subscriptionGroupLocalizations` | Create a version-scoped localization | Implemented typed command | #1780 | `48d04e0b`; exact create payload tests |
| `GET` | `/v2/subscriptionGroupLocalizations/{id}` | View a localization | Implemented typed command | #1780 | `48d04e0b`; detail response tests |
| `PATCH` | `/v2/subscriptionGroupLocalizations/{id}` | Update a localization | Implemented typed command | #1780 | `48d04e0b`; omitted/value/null payload tests |
| `DELETE` | `/v2/subscriptionGroupLocalizations/{id}` | Delete a localization with confirmation | Implemented typed command | #1780 | `48d04e0b`; confirmation and HTTP tests |

The review-submission relationship for `subscriptionGroupVersion` modifies the
existing `/v1/reviewSubmissionItems` operation.

### Version review-submission coverage

All three types use `POST /v1/reviewSubmissionItems` with required
`reviewSubmission.data` plus exactly one version relationship. The discoverable
generic command is `asc review items add --submission "SUBMISSION_ID"
--item-type "TYPE" --item-id "VERSION_ID"`; domain-specific submit shortcuts
may delegate to the same typed client after their ID semantics are explicit.

| Version type | Relationship payload | `--item-type` | Required test evidence | Status |
| --- | --- | --- | --- | --- |
| IAP | `inAppPurchaseVersion.data.type=inAppPurchaseVersions` | `inAppPurchaseVersions` | HTTP body test plus built command test | Implemented in #1777 at `ee40c7b3` and cross-verified in #1781 at `aba35e0a` |
| Subscription | `subscriptionVersion.data.type=subscriptionVersions` | `subscriptionVersions` | HTTP body test plus built command test | Implemented in #1781 at `aba35e0a` |
| Subscription group | `subscriptionGroupVersion.data.type=subscriptionGroupVersions` | `subscriptionGroupVersions` | HTTP body test plus built command test | Implemented in #1781 at `aba35e0a` |

The exact directly modified-operation checklist includes the four
review-submission read operations whose sparse fields and includes gain the
three version relationships. The `POST` request change is schema-mediated and
is tracked separately below because the operation object itself is unchanged.

### Subscription adjusted equalizations: 1

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `GET` | `/v1/subscriptionPricePoints/{id}/adjustedEqualizations` | List adjusted equalized price points using the exact territory, subscription, upfront-price-point, and plan-type filters supported by this operation | Implemented typed command | #1778 | `39284b0a`; exact query/response, strict CSV/enums, territory inclusion, opaque-next, ID validation, aggregation, and conflict tests |

## Modified existing contract ledger

The original 4.4-to-4.4.1 diff modifies 102 existing operations. Fifty remain
different from the immediate pre-PR repository snapshot and expand query or
response contracts. The other 52 were already reconciled in the repository
after the original 4.4 import: 44 reverse OpenAPI `deprecated: true` flags and
eight add media-localization sparse-field parameters. Each item remains in the
checklist so "already covered" is an audited disposition rather than an omitted
change.

| Contract area | Semantic change | Verification owner | Exact evidence |
| --- | --- | --- | --- |
| IAP reads | `fields[inAppPurchases]` gains `versions`; IAP detail and app-IAP collection reads gain `include=versions`, `fields[inAppPurchaseVersions]`, and `limit[versions]` | #1777 | `ee40c7b3`; endpoint-specific option and exact query tests |
| Subscription reads | Subscription detail and group-subscription reads gain version includes, sparse fields, and relationship limits; subscription sparse fields gain `versions` across related endpoints | #1779 | `c8f3ab52`; exact query and compatibility tests |
| Subscription-group reads | Group detail and app-group collection reads gain version includes, sparse fields, and relationship limits; group sparse fields gain `versions` | #1780 | `48d04e0b`; exact query, owner/next validation, and compatibility tests |
| Review submission reads | Review-item sparse fields and includes gain `inAppPurchaseVersion`, `subscriptionVersion`, and `subscriptionGroupVersion` | #1781 | `aba35e0a`; all four changed GET surfaces, automatic item inclusion, and response round-trip tests |
| Pricing reads | Price-point sparse fields gain `adjustedEqualizations`; existing equalization and price-point relationship operations gain `filter[upfrontPricePointId]` and `filter[planType]` where allowed | #1778 | `39284b0a`; endpoint-specific option, exact query, strict CSV/enums, territory inclusion, opaque-next, ID validation, and aggregation tests |
| Age rating reads and update | Age-rating sparse fields and update schema gain `socialMedia` and `socialMediaAgeRestricted` | #1778 and #1792 | `39284b0a` added the fields and CLI behavior; #1792 implementation head `db3657b6` adds exact omit/value/null request encoding |
| App info reads | `AppInfo.attributes.kidsAgeBand` and `fields[appInfos]=kidsAgeBand` appear as deprecated additions | #1778 | `39284b0a`; generic decoding/output characterization, no new write path |
| Included-resource unions | IAP, subscription, group, and review-submission responses gain their corresponding version resource discriminators | #1777, #1779, #1780, #1781 | `ee40c7b3`, `c8f3ab52`, `48d04e0b`, `aba35e0a`; typed response and included-resource tests |

No existing operation changes from nondeprecated to `deprecated: true` in the
OpenAPI JSON. Forty-four operations instead reverse `deprecated: true` from the
original 4.4 snapshot, primarily screenshot and preview resources. Separately,
Apple's prose release notes deprecate the seven version-replaced resource
families below without setting new operation-level flags. Deprecation behavior
therefore cannot be inferred solely from OpenAPI flags.

## Added and modified schema ledger

Schema discovery and drift enforcement are implemented in #1776 at
`aaa9b62d`: create/update request relationships are exposed through `asc schema`,
referenced relationship schemas are resolved recursively, and both generated
indexes fail their tests when stale.

The 47 added schemas break down into:

- 18 IAP schemas: version resources/linkages/responses, localization v2 CRUD
  requests/responses, and image v2 CRUD/upload requests/responses.
- 18 subscription schemas: version resources/linkages/responses, localization
  v2 CRUD requests/responses, and image v2 CRUD/upload requests/responses.
- 11 subscription-group schemas: version resources/linkages/responses and
  localization v2 CRUD requests/responses.

The exact 61-schema modified-contract checklist is split by whether code work
remained at the immediate pre-PR base.

Still different before the 4.4.1 schema PR:

- [x] `AgeRatingDeclaration` - two social-media Boolean attributes (#1778, `39284b0a`)
- [x] `AgeRatingDeclarationUpdateRequest` - two social-media update attributes added in #1778 (`39284b0a`); exact omit/value/null fidelity first implemented in #1792 at `db3657b6`
- [x] `AppInfo` - deprecated `kidsAgeBand` read attribute (#1778, `39284b0a`)
- [x] `InAppPurchaseV2` - versions relationship (#1777, `ee40c7b3`)
- [x] `InAppPurchaseV2Response` - included IAP-version discriminator (#1777, `ee40c7b3`)
- [x] `InAppPurchasesV2Response` - included IAP-version discriminator (#1777, `ee40c7b3`)
- [x] `ReviewSubmissionItem` - three version relationships (#1781, `aba35e0a`)
- [x] `ReviewSubmissionItemCreateRequest` - three version create relationships (#1781, `aba35e0a`)
- [x] `ReviewSubmissionItemResponse` - three included version discriminators (#1781, `aba35e0a`)
- [x] `ReviewSubmissionItemsResponse` - three included version discriminators (#1781, `aba35e0a`)
- [x] `Subscription` - versions relationship (#1779, `c8f3ab52`)
- [x] `SubscriptionGroup` - versions relationship (#1780, `48d04e0b`)
- [x] `SubscriptionGroupResponse` - included group-version discriminator (#1780, `48d04e0b`)
- [x] `SubscriptionGroupsResponse` - included group-version discriminator (#1780, `48d04e0b`)
- [x] `SubscriptionPricePoint` - adjusted-equalizations relationship (#1778, `39284b0a`)
- [x] `SubscriptionResponse` - included subscription-version discriminator (#1779, `c8f3ab52`)
- [x] `SubscriptionsResponse` - included subscription-version discriminator (#1779, `c8f3ab52`)

Already reconciled after the original 4.4 import and retained as audited
schema-only dispositions:

- [x] `AppCustomProductPageLocalization` - media sparse-field propagation already present
- [x] `AppCustomProductPageLocalizationAppPreviewSetsLinkagesResponse` - deprecation reversal already present
- [x] `AppCustomProductPageLocalizationAppScreenshotSetsLinkagesResponse` - deprecation reversal already present
- [x] `AppEventLocalization` - media sparse-field propagation already present
- [x] `AppEventLocalizationAppEventScreenshotsLinkagesResponse` - deprecation reversal already present
- [x] `AppEventLocalizationAppEventVideoClipsLinkagesResponse` - deprecation reversal already present
- [x] `AppEventScreenshot` - deprecation reversal already present
- [x] `AppEventScreenshotCreateRequest` - deprecation reversal already present
- [x] `AppEventScreenshotResponse` - deprecation reversal already present
- [x] `AppEventScreenshotUpdateRequest` - deprecation reversal already present
- [x] `AppEventScreenshotsResponse` - deprecation reversal already present
- [x] `AppEventVideoClip` - deprecation reversal already present
- [x] `AppEventVideoClipCreateRequest` - deprecation reversal already present
- [x] `AppEventVideoClipResponse` - deprecation reversal already present
- [x] `AppEventVideoClipUpdateRequest` - deprecation reversal already present
- [x] `AppEventVideoClipsResponse` - deprecation reversal already present
- [x] `AppPreview` - deprecation reversal already present
- [x] `AppPreviewCreateRequest` - deprecation reversal already present
- [x] `AppPreviewResponse` - deprecation reversal already present
- [x] `AppPreviewSet` - deprecation reversal already present
- [x] `AppPreviewSetAppPreviewsLinkagesRequest` - deprecation reversal already present
- [x] `AppPreviewSetAppPreviewsLinkagesResponse` - deprecation reversal already present
- [x] `AppPreviewSetCreateRequest` - deprecation reversal already present
- [x] `AppPreviewSetResponse` - deprecation reversal already present
- [x] `AppPreviewSetsResponse` - deprecation reversal already present
- [x] `AppPreviewUpdateRequest` - deprecation reversal already present
- [x] `AppPreviewsResponse` - deprecation reversal already present
- [x] `AppScreenshot` - deprecation reversal already present
- [x] `AppScreenshotCreateRequest` - deprecation reversal already present
- [x] `AppScreenshotResponse` - deprecation reversal already present
- [x] `AppScreenshotSet` - deprecation reversal already present
- [x] `AppScreenshotSetAppScreenshotsLinkagesRequest` - deprecation reversal already present
- [x] `AppScreenshotSetAppScreenshotsLinkagesResponse` - deprecation reversal already present
- [x] `AppScreenshotSetCreateRequest` - deprecation reversal already present
- [x] `AppScreenshotSetResponse` - deprecation reversal already present
- [x] `AppScreenshotSetsResponse` - deprecation reversal already present
- [x] `AppScreenshotUpdateRequest` - deprecation reversal already present
- [x] `AppScreenshotsResponse` - deprecation reversal already present
- [x] `AppStoreVersionExperimentTreatmentLocalization` - media sparse-field propagation already present
- [x] `AppStoreVersionExperimentTreatmentLocalizationAppPreviewSetsLinkagesResponse` - deprecation reversal already present
- [x] `AppStoreVersionExperimentTreatmentLocalizationAppScreenshotSetsLinkagesResponse` - deprecation reversal already present
- [x] `AppStoreVersionLocalization` - media sparse-field propagation already present
- [x] `AppStoreVersionLocalizationAppPreviewSetsLinkagesResponse` - deprecation reversal already present
- [x] `AppStoreVersionLocalizationAppScreenshotSetsLinkagesResponse` - deprecation reversal already present

### Exact added schema checklist

IAP ownership (#1777 at `ee40c7b3`; typed models plus request/response and
round-trip tests):

- [x] `InAppPurchaseVersion`
- [x] `InAppPurchaseVersionCreateRequest`
- [x] `InAppPurchaseVersionResponse`
- [x] `InAppPurchaseVersionsResponse`
- [x] `InAppPurchaseV2VersionsLinkagesResponse`
- [x] `InAppPurchaseVersionImageLinkageResponse`
- [x] `InAppPurchaseVersionImagesLinkagesResponse`
- [x] `InAppPurchaseVersionLocalizationsLinkagesResponse`
- [x] `InAppPurchaseLocalizationV2`
- [x] `InAppPurchaseLocalizationV2CreateRequest` - nullable create description completed by #1792 at `db3657b6`
- [x] `InAppPurchaseLocalizationV2UpdateRequest`
- [x] `InAppPurchaseLocalizationV2Response`
- [x] `InAppPurchaseLocalizationsV2Response`
- [x] `InAppPurchaseImageV2`
- [x] `InAppPurchaseImageV2CreateRequest`
- [x] `InAppPurchaseImageV2UpdateRequest` - nullable uploaded state completed by #1792 at `db3657b6`
- [x] `InAppPurchaseImageV2Response`
- [x] `InAppPurchaseImagesV2Response`

Subscription ownership (#1779 at `c8f3ab52`; typed models plus request/response
and round-trip tests):

- [x] `SubscriptionVersion`
- [x] `SubscriptionVersionCreateRequest`
- [x] `SubscriptionVersionResponse`
- [x] `SubscriptionVersionsResponse`
- [x] `SubscriptionVersionsLinkagesResponse`
- [x] `SubscriptionVersionImageLinkageResponse`
- [x] `SubscriptionVersionImagesLinkagesResponse`
- [x] `SubscriptionVersionLocalizationsLinkagesResponse`
- [x] `SubscriptionLocalizationV2`
- [x] `SubscriptionLocalizationV2CreateRequest` - nullable create description completed by #1792 at `db3657b6`
- [x] `SubscriptionLocalizationV2UpdateRequest`
- [x] `SubscriptionLocalizationV2Response`
- [x] `SubscriptionLocalizationsV2Response`
- [x] `SubscriptionImageV2`
- [x] `SubscriptionImageV2CreateRequest`
- [x] `SubscriptionImageV2UpdateRequest` - nullable uploaded state completed by #1792 at `db3657b6`
- [x] `SubscriptionImageV2Response`
- [x] `SubscriptionImagesV2Response`

Subscription-group ownership (#1780 at `48d04e0b`; typed models plus
request/response and round-trip tests):

- [x] `SubscriptionGroupVersion`
- [x] `SubscriptionGroupVersionCreateRequest`
- [x] `SubscriptionGroupVersionResponse`
- [x] `SubscriptionGroupVersionsResponse`
- [x] `SubscriptionGroupVersionsLinkagesResponse`
- [x] `SubscriptionGroupVersionLocalizationsLinkagesResponse`
- [x] `SubscriptionGroupLocalizationV2`
- [x] `SubscriptionGroupLocalizationV2CreateRequest` - nullable custom app name completed by #1792 at `db3657b6`
- [x] `SubscriptionGroupLocalizationV2UpdateRequest`
- [x] `SubscriptionGroupLocalizationV2Response`
- [x] `SubscriptionGroupLocalizationsV2Response`

### Exact modified-operation checklist

This checklist contains exactly the 102 operations whose path-item operation
objects differ between 4.4 and 4.4.1: 50 behavior changes that remained at the
immediate pre-PR base plus 52 changes already reconciled after the 4.4 import.
Schema-mediated request-contract changes are listed separately after it and do
not alter this count.

Age rating and app info (#1778 at `39284b0a`; sparse-field, typed-decoder,
payload, output, and compatibility tests):

- [x] `GET /v1/appInfoLocalizations/{id}`
- [x] `GET /v1/appInfos/{id}`
- [x] `GET /v1/appInfos/{id}/ageRatingDeclaration`
- [x] `GET /v1/appInfos/{id}/appInfoLocalizations`
- [x] `GET /v1/apps`
- [x] `GET /v1/apps/{id}`
- [x] `GET /v1/apps/{id}/appInfos`
- [x] `GET /v1/ciProducts/{id}/app`

IAP and promoted-purchase propagation (#1777 at `ee40c7b3`; endpoint-exact
query and response compatibility tests):

- [x] `GET /v1/apps/{id}/inAppPurchasesV2`
- [x] `GET /v1/inAppPurchaseAppStoreReviewScreenshots/{id}`
- [x] `GET /v1/inAppPurchaseContents/{id}`
- [x] `GET /v1/inAppPurchaseImages/{id}`
- [x] `GET /v1/inAppPurchaseLocalizations/{id}`
- [x] `GET /v1/promotedPurchases/{id}`
- [x] `GET /v2/inAppPurchases/{id}`
- [x] `GET /v2/inAppPurchases/{id}/appStoreReviewScreenshot`
- [x] `GET /v2/inAppPurchases/{id}/content`
- [x] `GET /v2/inAppPurchases/{id}/images`
- [x] `GET /v2/inAppPurchases/{id}/inAppPurchaseLocalizations`
- [x] `GET /v2/inAppPurchases/{id}/promotedPurchase`
- [x] `GET /v1/apps/{id}/promotedPurchases`

Review submissions (#1781 at `aba35e0a`; exact sparse-field/include tests plus
`links`, `included`, and `meta` response round trips):

- [x] `GET /v1/reviewSubmissions`
- [x] `GET /v1/reviewSubmissions/{id}`
- [x] `GET /v1/reviewSubmissions/{id}/items`
- [x] `GET /v1/apps/{id}/reviewSubmissions`

Review-item includes are exhaustive for the endpoint. Sparse fields for related
resources are exact for the 4.4.1 delta, but ten older related-resource sparse
groups remain outside this slice: `appStoreVersions`,
`appCustomProductPageVersions`, `appStoreVersionExperiments`, `appEvents`,
`backgroundAssetVersions`, and five Game Center resource groups. The API has no
review-item detail GET; deprecated detail stubs therefore remain explicit
errors. Item PATCH preserves nullable `resolved` and `removed` without adding a
confirmation prompt, while submission PATCH preserves nullable `platform`,
`submitted`, and `canceled` and still requires confirmation. Create targets are
restricted to the exact version resource types. The pre-existing submission-
create command still requires a concrete platform even though the schema
permits omission or `null`; changing that stable behavior is outside this
compatibility slice.

Subscriptions, groups, and pricing (#1778 at `39284b0a`, #1779 at `c8f3ab52`,
and #1780 at `48d04e0b`; endpoint-exact query, response, compatibility, and
opaque-pagination tests):

- [x] `GET /v1/apps/{id}/subscriptionGroups`
- [x] `GET /v1/subscriptionAppStoreReviewScreenshots/{id}`
- [x] `GET /v1/subscriptionGroupLocalizations/{id}`
- [x] `GET /v1/subscriptionGroups/{id}`
- [x] `GET /v1/subscriptionGroups/{id}/subscriptionGroupLocalizations`
- [x] `GET /v1/subscriptionGroups/{id}/subscriptions`
- [x] `GET /v1/subscriptionImages/{id}`
- [x] `GET /v1/subscriptionLocalizations/{id}`
- [x] `GET /v1/subscriptionOfferCodes/{id}`
- [x] `GET /v1/subscriptionOfferCodes/{id}/prices`
- [x] `GET /v1/subscriptionPricePoints/{id}`
- [x] `GET /v1/subscriptionPricePoints/{id}/equalizations`
- [x] `GET /v1/subscriptionPromotionalOffers/{id}`
- [x] `GET /v1/subscriptionPromotionalOffers/{id}/prices`
- [x] `GET /v1/subscriptions/{id}`
- [x] `GET /v1/subscriptions/{id}/appStoreReviewScreenshot`
- [x] `GET /v1/subscriptions/{id}/images`
- [x] `GET /v1/subscriptions/{id}/introductoryOffers`
- [x] `GET /v1/subscriptions/{id}/offerCodes`
- [x] `GET /v1/subscriptions/{id}/pricePoints`
- [x] `GET /v1/subscriptions/{id}/prices`
- [x] `GET /v1/subscriptions/{id}/promotedPurchase`
- [x] `GET /v1/subscriptions/{id}/promotionalOffers`
- [x] `GET /v1/subscriptions/{id}/subscriptionLocalizations`
- [x] `GET /v1/winBackOffers/{id}/prices`

Already reconciled between the original 4.4 import and the immediate pre-PR
base; checked items require no new behavior PR but remain part of the 102-item
contract audit.

Media sparse-field parameter changes:

- [x] `GET /v1/appCustomProductPageLocalizations/{id}`
- [x] `GET /v1/appCustomProductPageVersions/{id}/appCustomProductPageLocalizations`
- [x] `GET /v1/appEventLocalizations/{id}`
- [x] `GET /v1/appEvents/{id}/localizations`
- [x] `GET /v1/appStoreVersionExperimentTreatmentLocalizations/{id}`
- [x] `GET /v1/appStoreVersionExperimentTreatments/{id}/appStoreVersionExperimentTreatmentLocalizations`
- [x] `GET /v1/appStoreVersionLocalizations/{id}`
- [x] `GET /v1/appStoreVersions/{id}/appStoreVersionLocalizations`

Operation deprecation reversals:

- [x] `DELETE /v1/appEventScreenshots/{id}`
- [x] `DELETE /v1/appEventVideoClips/{id}`
- [x] `DELETE /v1/appPreviewSets/{id}`
- [x] `DELETE /v1/appPreviews/{id}`
- [x] `DELETE /v1/appScreenshotSets/{id}`
- [x] `DELETE /v1/appScreenshots/{id}`
- [x] `GET /v1/appCustomProductPageLocalizations/{id}/appPreviewSets`
- [x] `GET /v1/appCustomProductPageLocalizations/{id}/appScreenshotSets`
- [x] `GET /v1/appCustomProductPageLocalizations/{id}/relationships/appPreviewSets`
- [x] `GET /v1/appCustomProductPageLocalizations/{id}/relationships/appScreenshotSets`
- [x] `GET /v1/appEventLocalizations/{id}/appEventScreenshots`
- [x] `GET /v1/appEventLocalizations/{id}/appEventVideoClips`
- [x] `GET /v1/appEventLocalizations/{id}/relationships/appEventScreenshots`
- [x] `GET /v1/appEventLocalizations/{id}/relationships/appEventVideoClips`
- [x] `GET /v1/appEventScreenshots/{id}`
- [x] `GET /v1/appEventVideoClips/{id}`
- [x] `GET /v1/appPreviewSets/{id}`
- [x] `GET /v1/appPreviewSets/{id}/appPreviews`
- [x] `GET /v1/appPreviewSets/{id}/relationships/appPreviews`
- [x] `GET /v1/appPreviews/{id}`
- [x] `GET /v1/appScreenshotSets/{id}`
- [x] `GET /v1/appScreenshotSets/{id}/appScreenshots`
- [x] `GET /v1/appScreenshotSets/{id}/relationships/appScreenshots`
- [x] `GET /v1/appScreenshots/{id}`
- [x] `GET /v1/appStoreVersionExperimentTreatmentLocalizations/{id}/appPreviewSets`
- [x] `GET /v1/appStoreVersionExperimentTreatmentLocalizations/{id}/appScreenshotSets`
- [x] `GET /v1/appStoreVersionExperimentTreatmentLocalizations/{id}/relationships/appPreviewSets`
- [x] `GET /v1/appStoreVersionExperimentTreatmentLocalizations/{id}/relationships/appScreenshotSets`
- [x] `GET /v1/appStoreVersionLocalizations/{id}/appPreviewSets`
- [x] `GET /v1/appStoreVersionLocalizations/{id}/appScreenshotSets`
- [x] `GET /v1/appStoreVersionLocalizations/{id}/relationships/appPreviewSets`
- [x] `GET /v1/appStoreVersionLocalizations/{id}/relationships/appScreenshotSets`
- [x] `PATCH /v1/appEventScreenshots/{id}`
- [x] `PATCH /v1/appEventVideoClips/{id}`
- [x] `PATCH /v1/appPreviewSets/{id}/relationships/appPreviews`
- [x] `PATCH /v1/appPreviews/{id}`
- [x] `PATCH /v1/appScreenshotSets/{id}/relationships/appScreenshots`
- [x] `PATCH /v1/appScreenshots/{id}`
- [x] `POST /v1/appEventScreenshots`
- [x] `POST /v1/appEventVideoClips`
- [x] `POST /v1/appPreviewSets`
- [x] `POST /v1/appPreviews`
- [x] `POST /v1/appScreenshotSets`
- [x] `POST /v1/appScreenshots`

### Schema-mediated operation-contract checklist

These 71 operations do not appear in the 102-operation path-item diff because
their operation objects are byte-for-byte unchanged. They reference one or
more of the 61 modified schemas, so their effective request or response
contracts still change. Two have modified request contracts and all 71 have
modified response contracts. Together with the 102 directly modified
operations, they produce 173 unique operation-contract audit items.

Behavior work remaining at the immediate pre-PR base (40), now reconciled.
`PATCH /v1/ageRatingDeclarations/{id}` is covered by #1778 at `39284b0a`;
review-submission item request/response changes are covered by #1781 at
`aba35e0a`; IAP, subscription, and group response propagation is covered by
PR `#1777` at `ee40c7b3`, #1779 at `c8f3ab52`, and #1780 at `48d04e0b`. Checked
response-only operations retain their existing command semantics and decode the
expanded typed relationships without introducing a new flag or ID contract.

- [x] `PATCH /v1/ageRatingDeclarations/{id}` - request and response
- [x] `PATCH /v1/appInfoLocalizations/{id}`
- [x] `PATCH /v1/appInfos/{id}`
- [x] `PATCH /v1/apps/{id}`
- [x] `PATCH /v1/inAppPurchaseAppStoreReviewScreenshots/{id}`
- [x] `PATCH /v1/inAppPurchaseImages/{id}`
- [x] `PATCH /v1/inAppPurchaseLocalizations/{id}`
- [x] `PATCH /v1/promotedPurchases/{id}`
- [x] `PATCH /v1/reviewSubmissionItems/{id}`
- [x] `PATCH /v1/reviewSubmissions/{id}`
- [x] `PATCH /v1/subscriptionAppStoreReviewScreenshots/{id}`
- [x] `PATCH /v1/subscriptionGroupLocalizations/{id}`
- [x] `PATCH /v1/subscriptionGroups/{id}`
- [x] `PATCH /v1/subscriptionImages/{id}`
- [x] `PATCH /v1/subscriptionIntroductoryOffers/{id}`
- [x] `PATCH /v1/subscriptionLocalizations/{id}`
- [x] `PATCH /v1/subscriptionOfferCodes/{id}`
- [x] `PATCH /v1/subscriptionPromotionalOffers/{id}`
- [x] `PATCH /v1/subscriptions/{id}`
- [x] `PATCH /v2/inAppPurchases/{id}`
- [x] `POST /v1/appInfoLocalizations`
- [x] `POST /v1/inAppPurchaseAppStoreReviewScreenshots`
- [x] `POST /v1/inAppPurchaseImages`
- [x] `POST /v1/inAppPurchaseLocalizations`
- [x] `POST /v1/inAppPurchaseSubmissions`
- [x] `POST /v1/promotedPurchases`
- [x] `POST /v1/reviewSubmissionItems` - request and response
- [x] `POST /v1/reviewSubmissions`
- [x] `POST /v1/subscriptionAppStoreReviewScreenshots`
- [x] `POST /v1/subscriptionGroupLocalizations`
- [x] `POST /v1/subscriptionGroups`
- [x] `POST /v1/subscriptionImages`
- [x] `POST /v1/subscriptionIntroductoryOffers`
- [x] `POST /v1/subscriptionLocalizations`
- [x] `POST /v1/subscriptionOfferCodes`
- [x] `POST /v1/subscriptionPrices`
- [x] `POST /v1/subscriptionPromotionalOffers`
- [x] `POST /v1/subscriptionSubmissions`
- [x] `POST /v1/subscriptions`
- [x] `POST /v2/inAppPurchases`

Already reconciled through schema deprecation reversals or media model changes
after the original 4.4 import (31):

- [x] `GET /v1/appClipDefaultExperiences/{id}/releaseWithAppStoreVersion`
- [x] `GET /v1/appCustomProductPageVersions/{id}`
- [x] `GET /v1/appCustomProductPages/{id}`
- [x] `GET /v1/appCustomProductPages/{id}/appCustomProductPageVersions`
- [x] `GET /v1/appEvents/{id}`
- [x] `GET /v1/appStoreVersionExperimentTreatments/{id}`
- [x] `GET /v1/appStoreVersionExperiments/{id}/appStoreVersionExperimentTreatments`
- [x] `GET /v1/appStoreVersions/{id}`
- [x] `GET /v1/apps/{id}/appCustomProductPages`
- [x] `GET /v1/apps/{id}/appEvents`
- [x] `GET /v1/apps/{id}/appStoreVersions`
- [x] `GET /v1/builds/{id}/appStoreVersion`
- [x] `GET /v1/gameCenterAppVersions/{id}/appStoreVersion`
- [x] `GET /v2/appStoreVersionExperiments/{id}/appStoreVersionExperimentTreatments`
- [x] `PATCH /v1/appCustomProductPageLocalizations/{id}`
- [x] `PATCH /v1/appCustomProductPageVersions/{id}`
- [x] `PATCH /v1/appCustomProductPages/{id}`
- [x] `PATCH /v1/appEventLocalizations/{id}`
- [x] `PATCH /v1/appEvents/{id}`
- [x] `PATCH /v1/appStoreVersionExperimentTreatments/{id}`
- [x] `PATCH /v1/appStoreVersionLocalizations/{id}`
- [x] `PATCH /v1/appStoreVersions/{id}`
- [x] `POST /v1/appCustomProductPageLocalizations`
- [x] `POST /v1/appCustomProductPageVersions`
- [x] `POST /v1/appCustomProductPages`
- [x] `POST /v1/appEventLocalizations`
- [x] `POST /v1/appEvents`
- [x] `POST /v1/appStoreVersionExperimentTreatmentLocalizations`
- [x] `POST /v1/appStoreVersionExperimentTreatments`
- [x] `POST /v1/appStoreVersionLocalizations`
- [x] `POST /v1/appStoreVersions`

## Release-note capability ledger

| # | Apple addition | Owner | Verification | Status |
| ---: | --- | --- | --- | --- |
| 1 | Discrete IAP versions and their localizations/review images | #1777 | 18-operation ledger, CLI/HTTP/upload tests | Implemented at `ee40c7b3` |
| 2 | Discrete subscription versions and their localizations/promotional images | #1779 | 18-operation ledger, CLI/HTTP/upload tests | Implemented at `c8f3ab52` |
| 3 | Discrete subscription-group versions and their localizations | #1780 | 10-operation ledger and CLI/HTTP tests | Implemented at `48d04e0b` |
| 4 | Submit all three version types through review-submission items | #1777 and #1781 | Three exact relationship payload tests plus built-command tests | Implemented at `ee40c7b3` and `aba35e0a` |
| 5 | Version-scoped v2 IAP localizations and images | #1777 and #1792 | CRUD/upload coverage from #1777; #1792 table-tests create-description and image-upload omission/value/null encoding | Feature implementation at `ee40c7b3`; complete nullable request fidelity at #1792 implementation head `db3657b6` (pre-merge) |
| 6 | Version-scoped v2 subscription localizations and images | #1779 and #1792 | CRUD/upload coverage from #1779; #1792 table-tests create-description and image-upload omission/value/null encoding | Feature implementation at `c8f3ab52`; complete nullable request fidelity at #1792 implementation head `db3657b6` (pre-merge) |
| 7 | Version-scoped v2 subscription-group localizations | #1780 and #1792 | CRUD coverage from #1780; #1792 table-tests `customAppName` omission/value/null encoding, including whitespace omission | Feature implementation at `48d04e0b`; complete nullable request fidelity at #1792 implementation head `db3657b6` (pre-merge) |
| 8 | Adjusted subscription equalizations and new filters | #1778 | Exact query/response, option-scope, strict-CSV/enum, territory-inclusion, opaque-next, ID-validation, and aggregation tests | Implemented at `39284b0a` |
| 9 | `socialMedia` and `socialMediaAgeRestricted` age-rating attributes | #1778 and #1792 | Payload/output/help coverage from #1778; #1792 table-tests omission/value/null encoding for both update fields | Feature implementation at `39284b0a`; complete nullable request fidelity at #1792 implementation head `db3657b6` (pre-merge) |

## Deprecation and migration ledger

Apple deprecates seven resource families in the prose release notes:

| Deprecated family | Replacement API and implemented command | Owner | Compatibility and warning status | Transition evidence |
| --- | --- | --- | --- | --- |
| IAP localizations v1 | `/v2/inAppPurchaseLocalizations`; `asc iap versions localizations ... --version-id` | #1777, #1786 | Four stable leaves preserved with one exact warning and DEPRECATED direct help | CRUD/ID tests, warning/payload/exit compatibility tests, migration docs |
| IAP images v1 | `/v2/inAppPurchaseImages`; `asc iap versions images ... --version-id` | #1777, #1786 | Five stable leaves preserved with one exact warning and DEPRECATED direct help | Upload characterization, reserve/upload/commit tests, migration docs |
| IAP submissions | `/v1/reviewSubmissionItems`; `asc review items add --item-type inAppPurchaseVersions` | #1777, #1781, #1786, #1787 | Stable submit leaf warns, remains directly callable, and is visible in parent help | Exact relationship payload, warning/exit tests, rendered-help regression |
| Subscription localizations v1 | `/v2/subscriptionLocalizations`; `asc subscriptions versions localizations ... --version-id` | #1779, #1786 | Five stable leaves and one experimental `sync` leaf preserved with exact warnings | CRUD/ID, warning/payload/exit, and migration-doc tests |
| Subscription images v1 | `/v2/subscriptionImages`; `asc subscriptions versions images ... --version-id` | #1779, #1786 | Five stable leaves preserved with exact warnings | Upload characterization, reserve/upload/commit, and migration-doc tests |
| Subscription-group localizations v1 | `/v2/subscriptionGroupLocalizations`; `asc subscriptions groups versions localizations ... --version-id` | #1780, #1786 | Five stable leaves and one experimental `sync` leaf preserved with exact warnings | CRUD/ID, warning/payload/exit, and migration-doc tests |
| Subscription and group submissions | `/v1/reviewSubmissionItems`; item types `subscriptionVersions` and `subscriptionGroupVersions` | #1781, #1786 | Two stable submit leaves preserved with exact warnings and DEPRECATED direct help | Relationship payload, warning/exit, and migration-doc tests |

PR `#1786` begins the repository's required deprecation window for 29 public leaves:
27 stable and two experimental `sync` commands. `asc iap setup` and
`asc subscriptions setup` remain stable but emit one combined warning when
legacy localization flags are requested. All 33 exported legacy client methods
carry precise Go `Deprecated:` replacement documentation. The wrapper preserves
flags, endpoint selection, stdout, confirmation requirements, and exit behavior
while writing one migration warning to stderr.

No stable behavior was deleted. Removal requires a later release after the
documented deprecation window; this goal intentionally stops before release.

## Pull-request sequence and status

1. Schema tooling was audited at #1776 head `aaa9b62d` and landed as
   `893c840e`.
2. IAP versions were audited at #1777 head `ee40c7b3` and landed as
   `bd453af4`.
3. Age rating and pricing were audited at #1778 head `39284b0a` and landed as
   `f6be93f6`.
4. Subscription versions were audited at #1779 head `c8f3ab52` and landed as
   `0349e397`.
5. Subscription-group versions were audited at #1780 head `48d04e0b` and
   landed as `8cbcdadc`.
6. Cross-cutting review integration was audited at #1781 head `aba35e0a` and
   landed as `eea98d3a`.
7. Age-rating prerequisite validation was audited at #1782 head `3dfd15a0` and
   landed as `b92e5605`.
8. Adjusted-equalization filter validation was audited at #1783 head
   `a06c935c` and landed as `6f11d6aa`.
9. Positional-argument validation was audited at #1784 head `fc5f0504` and
   landed as `1cc9face`.
10. Relationship-limit compatibility was audited at #1785 head `da030480` and
    landed as `b01d4d70`.
11. The 29-leaf deprecation transition was audited at #1786 head `1ef32153` and
    landed as `a00985b2`.
12. Deprecated IAP submit discoverability was audited at #1787 head `04b52a62`
    and landed as `38fa9b5f`.
13. The final transitive age-rating dependency was audited at #1788 head
    `08003c38` and landed as `9282e82d`; the historical #1782 thread was then
    resolved with exact-main evidence.
14. The final coverage ledger was audited at #1789 head `51b3a962` and landed
    as `d6d8d94b` without changing CLI behavior.
15. Subscription localization delete-confirmation coverage was audited at
    #1790 head `49eda126` and landed as `73466720`; it is test-only.
16. Hardened public 4.4.1 command workflows were audited at #1791 head
    `e9c2a0dc` and landed as `e902d375`; that docs-only commit is the current
    base of #1792.
17. #1792 first completes all seven nullable request properties at exact
    implementation head `db3657b6c6e88ebbf7f0da59137c92df99931e59`.
    The PR remains open and pre-merge, this ledger change is a later docs-only
    commit on that PR, and no landed `main` SHA is claimed.
18. External workflow skills were audited at exact head `61b9634` and landed as
    `e5561a9`. All 11 review threads are resolved, and the landed tree passes all
    23 skill validators plus 245 command-example and 716 flag help checks.
19. Exact CLI `main` `9282e82d` is the last fully merged behavior integration
    before #1792 and has green integration, Govulncheck, and CodeQL workflows.
    Its built help has 48 added and 52 changed leaf paths relative to
    `839c4da6`, with zero removals. Those workflows and help counts do not prove
    #1792's still-pre-merge nullable encoding.

CLI behavior and lifecycle PRs through #1788, the ledger PR, test-only #1790,
docs-only #1791, and the companion skills PR are merged. #1792 remains the
nullable-fidelity closeout gate. No release, tag, or package publication is
part of this goal.

### Built help-surface delta

The recursive comparison executes every reachable leaf `--help` path in fresh
binaries built at `839c4da6` and `9282e82d`. It found 48 additions:

```text
iap versions create
iap versions image
iap versions images create
iap versions images delete
iap versions images list
iap versions images update
iap versions images view
iap versions links image
iap versions links images
iap versions links localizations
iap versions links versions
iap versions list
iap versions localizations create
iap versions localizations delete
iap versions localizations list
iap versions localizations update
iap versions localizations view
iap versions submit
iap versions view
subscriptions groups versions create
subscriptions groups versions links localizations
subscriptions groups versions links versions
subscriptions groups versions list
subscriptions groups versions localizations create
subscriptions groups versions localizations delete
subscriptions groups versions localizations list
subscriptions groups versions localizations update
subscriptions groups versions localizations view
subscriptions groups versions view
subscriptions pricing price-points adjusted-equalizations
subscriptions versions create
subscriptions versions images delete
subscriptions versions images links
subscriptions versions images list
subscriptions versions images primary
subscriptions versions images primary-link
subscriptions versions images update
subscriptions versions images upload
subscriptions versions images view
subscriptions versions links
subscriptions versions list
subscriptions versions localizations create
subscriptions versions localizations delete
subscriptions versions localizations links
subscriptions versions localizations list
subscriptions versions localizations update
subscriptions versions localizations view
subscriptions versions view
```

It found 52 changed leaf paths:

```text
age-rating edit
iap images create
iap images delete
iap images list
iap images update
iap images view
iap list
iap localizations create
iap localizations delete
iap localizations list
iap localizations update
iap setup
iap submit
iap view
review items add
review items list
review items update
review items view
review items-add
review items-get
review items-list
review items-update
review submissions-get
review submissions-list
review submissions-update
schema
subscriptions groups list
subscriptions groups localizations create
subscriptions groups localizations delete
subscriptions groups localizations list
subscriptions groups localizations sync
subscriptions groups localizations update
subscriptions groups localizations view
subscriptions groups view
subscriptions images create
subscriptions images delete
subscriptions images list
subscriptions images update
subscriptions images view
subscriptions list
subscriptions localizations create
subscriptions localizations delete
subscriptions localizations list
subscriptions localizations sync
subscriptions localizations update
subscriptions localizations view
subscriptions pricing price-points equalizations
subscriptions pricing price-points list
subscriptions review submit
subscriptions review submit-group
subscriptions setup
subscriptions view
```

There are zero removed leaf paths. `iap submit` remains callable and now appears
in parent help with its DEPRECATED migration text; it is counted as changed,
not removed.

## Mandatory verification for every behavior PR

- Inspect current built `--help` before choosing the command shape.
- Validate the exact operation's request attributes, relationships, filters,
  includes, sparse fields, limits, and response schemas.
- Establish RED CLI tests and HTTP method/path/query/body tests.
- Cover success, required-flag validation, invalid values, API errors, empty
  responses, pagination, and upload/artifact failures where applicable.
- Assert destructive commands require `--confirm` before authentication or
  network side effects.
- Verify JSON and representative table output by structure, not only strings.
- Build a fresh `/tmp/asc` binary and verify stdout, stderr, help, and exit codes.
- Use `ASC_BYPASS_KEYCHAIN=1` for every manual CLI test.
- Run focused tests after each fix, adjacent packages before commit, and then:

```bash
make format
make check-docs
make lint
ASC_BYPASS_KEYCHAIN=1 make test
```

- Prefer read-only live verification. Use disposable app `6759231657` for
  mutations, record created IDs, clean them up, and report leftovers.
- Re-query the latest PR head, thread-aware reviews, required checks, and
  mergeability before declaring a slice ready.

## Final omission audit

The merged closeout through #1789 independently repeated these checks rather
than trusting the per-PR reports. The later nullable-fidelity finding and its
pre-merge evidence are separated explicitly:

- [x] Re-downloaded Apple's official OpenAPI zip and verified the artifact and
  extracted JSON hashes against the repository snapshot.
- [x] Recomputed the 4.4-to-4.4.1 delta: 37 paths, 47 added operations, 47 added
  schemas, zero removals, 102 directly modified operations, and 61 modified
  schemas.
- [x] Mapped all 47 added operations to exact HTTP method/path tests and
  discoverable typed command/client surfaces.
- [x] Classified all 102 direct plus 71 transitive operation-contract changes:
  173 unique existing-operation contracts with no missing or extra ledger item.
- [x] On #1792 implementation head `db3657b6`, mapped all 47 added and 61
  modified schemas to typed models, documented generic decoding, or an
  already-reconciled schema-only disposition. This includes table-driven
  omission/value/null encoding for the seven nullable properties missed by the
  earlier merged closeout.
- [x] Mapped all nine release-note additions and seven deprecated families to
  commands, compatibility treatment, tests, and migration guidance.
- [x] Exercised all 100 changed or new leaf help paths: 48 additions and 52
  changes, with zero removals. Generated command docs and rendered parent help
  teach the v2 migrations; client registries and upload helpers match the
  contract.
- [x] Sequentially merged all 13 exact audited CLI heads through #1788,
  producing `main` `9282e82d`; its integration, Govulncheck, and CodeQL
  workflows passed.
- [ ] #1792 remains open and pre-merge. Its implementation head is
  `db3657b6c6e88ebbf7f0da59137c92df99931e59`; there is no landed `main` SHA or
  post-merge workflow result to record yet.
- [x] Deprecated all 29 public legacy leaves with one exact warning and direct
  migration help, added conditional setup warnings, and documented all 33
  exported legacy client methods without deleting stable behavior.
- [x] Exercised all 47 added operations against live App Store Connect on app
  `6759231657`, including reserve/upload/commit/delete image lifecycles and
  version/localization mutations.
- [x] Recorded live schema differences: adjusted-equalization prerequisites,
  age-rating dependencies, and Apple's rejection of empty/null v2 localization
  descriptions.
- [x] Verified validator v1 reads see v2-created resources with identical IDs;
  no runtime validator migration was needed.
- [x] Cleaned every disposable resource the API permits and recorded the three
  unavoidable version/localization remnant pairs. All four review submissions
  are empty and age-rating fields were restored.
- [x] Audited and merged companion workflow-skills PR #50 at exact head
  `61b9634`, producing skills `main` `e5561a9`; all 11 threads are resolved,
  all 23 skills validate, and 245 command examples with 716 flags match exact
  CLI help. No runnable deprecated localization or submission teaching remains.
- [x] Ran the final read-only live smoke on disposable app `6759231657` after
  behavior work through #1788 and the workflow-skill changes merged: the app
  read succeeded, all four age fields were restored to `false`, and all four
  review submissions contained zero items. This predates #1792 and is not
  evidence that Apple accepts explicit null for its seven corrected fields.
- [x] Confirmed the last fully merged behavior integration, `main` `9282e82d`,
  has green integration, Govulncheck, and CodeQL workflows; #1789 later landed
  the ledger at `d6d8d94b`, test-only #1790 landed at `73466720`, and docs-only
  #1791 produced #1792's base `e902d375`. Skills `main` `e5561a9` still passes local validation (it has no
  configured `main` status or check runs), and the latest release/tag remains
  `3.0.0` at the pre-integration commit `839c4da6`. No release, tag,
  Homebrew/WinGet update, or package publication was performed.

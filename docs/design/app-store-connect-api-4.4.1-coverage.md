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

The implementation is complete across six reviewable pull requests. The heads
recorded here are the exact commits used to reconcile this ledger:

| Scope | Pull request | Audited head | Status |
| --- | --- | --- | --- |
| Relationship-aware schema discovery and stale-index enforcement | #1776 | `aaa9b62d` | Implemented and repository-gated |
| IAP versions, v2 localizations/images, compatibility, and docs | #1777 | `2b4668b1` | Implemented and repository-gated |
| Age-rating social-media fields and adjusted equalizations | #1778 | `64e2f1da` | Implemented and repository-gated |
| Subscription versions, v2 localizations/images, compatibility, and docs | #1779 | `aa4856bd` | Implemented and repository-gated |
| Subscription-group versions, v2 localizations, compatibility, and docs | #1780 | `83f3103e` | Implemented and repository-gated |
| Cross-cutting review-submission version items and migration notes | #1781 | `77ced0b1` | Implemented and repository-gated |
| External ASC workflow skills | rorkai/app-store-connect-cli-skills#50 | `5e6fad3d` | Targeted runtime examples green; two optional validators `UNVERIFIED` |
| Zero-conflict six-PR integration | Local integration audit | `5f486516` | Exact-head format/docs/lint/test/build green |

The hard audit found and fixed contract gaps beyond the initial implementation:
explicit JSON `null` support for nullable v2 localization updates; endpoint-
exact IAP detail/list options; IAP `versions` sparse-field propagation; opaque
Apple pagination URLs and mutually exclusive query flags for price points;
exact review-item fields/includes; preservation of review response `links`,
`included`, and `meta`; positional-argument rejection; and missing command
documentation. The final review pass also made `--item-fields` automatically
include submission items on list/detail requests, removed duplicate cross-PR
resource-type constants, and removed examples for absent companion commands.
Pricing closeout added next-aware blank subscription/price-point ID validation
before HTTP, asserted both aggregated pagination pages, and made the HTTP test
handler concurrency-safe by avoiding `Fatalf`. It also made
`--territory-fields` automatically request `include=territory` for price-point
lists and equalizations, with exact-query tests that omit the explicit include.
The shared pricing CSV parser now rejects separator-only input and any empty
element for every new pricing flag, with seven CLI cases proving exit 2 before
authentication.
The subscription-group test hook was moved to a dedicated file, eliminating
the only genuine #1779/#1780 merge conflict without changing runtime behavior.
Four group-version list commands now reject an owner ID combined with opaque
`--next` before authentication or HTTP, proven by poison-client regression
tests.

The external ASC workflow skills are synchronized in draft PR #50 at
`5e6fad3d`, whose body references exact CLI heads #1778 `64e2f1da` and #1780
`83f3103e`. Rebuilt-head checks pass all three review-item examples, six group
examples, both pricing/age help paths, opaque-next conflicts, separator-only
CSV rejection, and value/clear conflicts. The draft is mergeable with no
review threads and no proven content drift. Its PyYAML and npm-based optional
validators remain explicitly `UNVERIFIED` because those validators were
unavailable; targeted runtime evidence covers the changed surface.

The definitive integration at `5f486516` starts from `main` `25b33c17` and
combines the six whole PR heads above with zero conflicts and zero manual edits.
It preserves the exact 37-path, 47-operation, 47-schema, 102-direct,
71-transitive, 173-contract, 61-modified-schema, and 9-addition/7-deprecation
counts. All 62 changed or new leaf help paths render, and `make format`, `make
check-docs`, `make lint`, `ASC_BYPASS_KEYCHAIN=1 make test`, and `make build`
pass on that exact clean tree. #1778's one unrelated submit-package timing
failure was classified against the same head; its GitHub package-shard rerun
passed without a code change.

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
| `POST` | `/v1/inAppPurchaseVersions` | Create a version for an IAP relationship | Implemented typed command | #1777 | `2b4668b1`; HTTP body and built-command tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}` | View a version | Implemented typed command | #1777 | `2b4668b1`; HTTP query and built-command tests |
| `GET` | `/v2/inAppPurchases/{id}/versions` | List related versions with pagination | Implemented typed command | #1777 | `2b4668b1`; exact query and pagination tests |
| `GET` | `/v2/inAppPurchases/{id}/relationships/versions` | List version linkages | Implemented typed client/command | #1777 | `2b4668b1`; linkage response tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/localizations` | List version localizations | Implemented typed command | #1777 | `2b4668b1`; exact path/query tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/localizations` | List localization linkages | Implemented typed client/command | #1777 | `2b4668b1`; linkage response tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/image` | Get the singular review image | Implemented typed command | #1777 | `2b4668b1`; singular response tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/image` | Get singular image linkage | Implemented typed client/command | #1777 | `2b4668b1`; linkage response tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/images` | List review images | Implemented typed command | #1777 | `2b4668b1`; list and pagination tests |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/images` | List image linkages | Implemented typed client/command | #1777 | `2b4668b1`; linkage response tests |
| `POST` | `/v2/inAppPurchaseLocalizations` | Create a version-scoped localization | Implemented typed command | #1777 | `2b4668b1`; exact create payload tests |
| `GET` | `/v2/inAppPurchaseLocalizations/{id}` | View a localization | Implemented typed command | #1777 | `2b4668b1`; detail response tests |
| `PATCH` | `/v2/inAppPurchaseLocalizations/{id}` | Update a localization | Implemented typed command | #1777 | `2b4668b1`; omitted/value/null payload tests |
| `DELETE` | `/v2/inAppPurchaseLocalizations/{id}` | Delete a localization with confirmation | Implemented typed command | #1777 | `2b4668b1`; confirmation and HTTP tests |
| `POST` | `/v2/inAppPurchaseImages` | Reserve and upload a version-scoped image | Implemented upload command | #1777 | `2b4668b1`; reserve/upload lifecycle tests |
| `GET` | `/v2/inAppPurchaseImages/{id}` | View an image and upload state | Implemented typed command | #1777 | `2b4668b1`; detail response tests |
| `PATCH` | `/v2/inAppPurchaseImages/{id}` | Commit uploaded parts | Implemented upload command | #1777 | `2b4668b1`; checksum and commit tests |
| `DELETE` | `/v2/inAppPurchaseImages/{id}` | Delete an image with confirmation | Implemented typed command | #1777 | `2b4668b1`; confirmation and HTTP tests |

The review-submission relationship for `inAppPurchaseVersion` modifies the
existing `/v1/reviewSubmissionItems` operation rather than adding another path.

### Subscription versions and version-scoped metadata: 18

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/subscriptionVersions` | Create a version for a subscription relationship | Implemented typed command | #1779 | `aa4856bd`; HTTP body and built-command tests |
| `GET` | `/v1/subscriptionVersions/{id}` | View a version | Implemented typed command | #1779 | `aa4856bd`; HTTP query and built-command tests |
| `GET` | `/v1/subscriptions/{id}/versions` | List related versions with pagination | Implemented typed command | #1779 | `aa4856bd`; exact query and pagination tests |
| `GET` | `/v1/subscriptions/{id}/relationships/versions` | List version linkages | Implemented typed client/command | #1779 | `aa4856bd`; linkage response tests |
| `GET` | `/v1/subscriptionVersions/{id}/localizations` | List version localizations | Implemented typed command | #1779 | `aa4856bd`; exact path/query tests |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/localizations` | List localization linkages | Implemented typed client/command | #1779 | `aa4856bd`; linkage response tests |
| `GET` | `/v1/subscriptionVersions/{id}/image` | Get the singular promotional image | Implemented typed command | #1779 | `aa4856bd`; singular response tests |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/image` | Get singular image linkage | Implemented typed client/command | #1779 | `aa4856bd`; linkage response tests |
| `GET` | `/v1/subscriptionVersions/{id}/images` | List promotional images | Implemented typed command | #1779 | `aa4856bd`; list and pagination tests |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/images` | List image linkages | Implemented typed client/command | #1779 | `aa4856bd`; linkage response tests |
| `POST` | `/v2/subscriptionLocalizations` | Create a version-scoped localization | Implemented typed command | #1779 | `aa4856bd`; exact create payload tests |
| `GET` | `/v2/subscriptionLocalizations/{id}` | View a localization | Implemented typed command | #1779 | `aa4856bd`; detail response tests |
| `PATCH` | `/v2/subscriptionLocalizations/{id}` | Update a localization | Implemented typed command | #1779 | `aa4856bd`; omitted/value/null payload tests |
| `DELETE` | `/v2/subscriptionLocalizations/{id}` | Delete a localization with confirmation | Implemented typed command | #1779 | `aa4856bd`; confirmation and HTTP tests |
| `POST` | `/v2/subscriptionImages` | Reserve and upload a version-scoped image | Implemented upload command | #1779 | `aa4856bd`; reserve/upload lifecycle tests |
| `GET` | `/v2/subscriptionImages/{id}` | View an image and upload state | Implemented typed command | #1779 | `aa4856bd`; detail response tests |
| `PATCH` | `/v2/subscriptionImages/{id}` | Commit uploaded parts | Implemented upload command | #1779 | `aa4856bd`; checksum and commit tests |
| `DELETE` | `/v2/subscriptionImages/{id}` | Delete an image with confirmation | Implemented typed command | #1779 | `aa4856bd`; confirmation and HTTP tests |

The review-submission relationship for `subscriptionVersion` modifies the
existing `/v1/reviewSubmissionItems` operation.

### Subscription-group versions and localizations: 10

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/subscriptionGroupVersions` | Create a version for a group relationship | Implemented typed command | #1780 | `83f3103e`; HTTP body and built-command tests |
| `GET` | `/v1/subscriptionGroupVersions/{id}` | View a version | Implemented typed command | #1780 | `83f3103e`; HTTP query and built-command tests |
| `GET` | `/v1/subscriptionGroups/{id}/versions` | List related versions with pagination | Implemented typed command | #1780 | `83f3103e`; exact query, owner/next validation, and pagination tests |
| `GET` | `/v1/subscriptionGroups/{id}/relationships/versions` | List version linkages | Implemented typed client/command | #1780 | `83f3103e`; owner/next validation and linkage response tests |
| `GET` | `/v1/subscriptionGroupVersions/{id}/localizations` | List version localizations | Implemented typed command | #1780 | `83f3103e`; exact path/query and owner/next validation tests |
| `GET` | `/v1/subscriptionGroupVersions/{id}/relationships/localizations` | List localization linkages | Implemented typed client/command | #1780 | `83f3103e`; owner/next validation and linkage response tests |
| `POST` | `/v2/subscriptionGroupLocalizations` | Create a version-scoped localization | Implemented typed command | #1780 | `83f3103e`; exact create payload tests |
| `GET` | `/v2/subscriptionGroupLocalizations/{id}` | View a localization | Implemented typed command | #1780 | `83f3103e`; detail response tests |
| `PATCH` | `/v2/subscriptionGroupLocalizations/{id}` | Update a localization | Implemented typed command | #1780 | `83f3103e`; omitted/value/null payload tests |
| `DELETE` | `/v2/subscriptionGroupLocalizations/{id}` | Delete a localization with confirmation | Implemented typed command | #1780 | `83f3103e`; confirmation and HTTP tests |

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
| IAP | `inAppPurchaseVersion.data.type=inAppPurchaseVersions` | `inAppPurchaseVersions` | HTTP body test plus built command test | Implemented in #1777 at `2b4668b1` and cross-verified in #1781 at `77ced0b1` |
| Subscription | `subscriptionVersion.data.type=subscriptionVersions` | `subscriptionVersions` | HTTP body test plus built command test | Implemented in #1781 at `77ced0b1` |
| Subscription group | `subscriptionGroupVersion.data.type=subscriptionGroupVersions` | `subscriptionGroupVersions` | HTTP body test plus built command test | Implemented in #1781 at `77ced0b1` |

The exact directly modified-operation checklist includes the four
review-submission read operations whose sparse fields and includes gain the
three version relationships. The `POST` request change is schema-mediated and
is tracked separately below because the operation object itself is unchanged.

### Subscription adjusted equalizations: 1

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `GET` | `/v1/subscriptionPricePoints/{id}/adjustedEqualizations` | List adjusted equalized price points using the exact territory, subscription, upfront-price-point, and plan-type filters supported by this operation | Implemented typed command | #1778 | `64e2f1da`; exact query/response, strict CSV, territory inclusion, opaque-next, ID validation, aggregation, and conflict tests |

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
| IAP reads | `fields[inAppPurchases]` gains `versions`; IAP detail and app-IAP collection reads gain `include=versions`, `fields[inAppPurchaseVersions]`, and `limit[versions]` | #1777 | `2b4668b1`; endpoint-specific option and exact query tests |
| Subscription reads | Subscription detail and group-subscription reads gain version includes, sparse fields, and relationship limits; subscription sparse fields gain `versions` across related endpoints | #1779 | `aa4856bd`; exact query and compatibility tests |
| Subscription-group reads | Group detail and app-group collection reads gain version includes, sparse fields, and relationship limits; group sparse fields gain `versions` | #1780 | `83f3103e`; exact query, owner/next validation, and compatibility tests |
| Review submission reads | Review-item sparse fields and includes gain `inAppPurchaseVersion`, `subscriptionVersion`, and `subscriptionGroupVersion` | #1781 | `77ced0b1`; all four changed GET surfaces, automatic item inclusion, and response round-trip tests |
| Pricing reads | Price-point sparse fields gain `adjustedEqualizations`; existing equalization and price-point relationship operations gain `filter[upfrontPricePointId]` and `filter[planType]` where allowed | #1778 | `64e2f1da`; endpoint-specific option, exact query, strict CSV, territory inclusion, opaque-next, ID validation, and aggregation tests |
| Age rating reads and update | Age-rating sparse fields and update schema gain `socialMedia` and `socialMediaAgeRestricted` | #1778 | `64e2f1da`; payload, output, help, and exit-behavior tests |
| App info reads | `AppInfo.attributes.kidsAgeBand` and `fields[appInfos]=kidsAgeBand` appear as deprecated additions | #1778 | `64e2f1da`; generic decoding/output characterization, no new write path |
| Included-resource unions | IAP, subscription, group, and review-submission responses gain their corresponding version resource discriminators | #1777, #1779, #1780, #1781 | `2b4668b1`, `aa4856bd`, `83f3103e`, `77ced0b1`; typed response and included-resource tests |

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

- [x] `AgeRatingDeclaration` - two social-media Boolean attributes (#1778, `64e2f1da`)
- [x] `AgeRatingDeclarationUpdateRequest` - two nullable social-media update attributes (#1778, `64e2f1da`)
- [x] `AppInfo` - deprecated `kidsAgeBand` read attribute (#1778, `64e2f1da`)
- [x] `InAppPurchaseV2` - versions relationship (#1777, `2b4668b1`)
- [x] `InAppPurchaseV2Response` - included IAP-version discriminator (#1777, `2b4668b1`)
- [x] `InAppPurchasesV2Response` - included IAP-version discriminator (#1777, `2b4668b1`)
- [x] `ReviewSubmissionItem` - three version relationships (#1781, `77ced0b1`)
- [x] `ReviewSubmissionItemCreateRequest` - three version create relationships (#1781, `77ced0b1`)
- [x] `ReviewSubmissionItemResponse` - three included version discriminators (#1781, `77ced0b1`)
- [x] `ReviewSubmissionItemsResponse` - three included version discriminators (#1781, `77ced0b1`)
- [x] `Subscription` - versions relationship (#1779, `aa4856bd`)
- [x] `SubscriptionGroup` - versions relationship (#1780, `83f3103e`)
- [x] `SubscriptionGroupResponse` - included group-version discriminator (#1780, `83f3103e`)
- [x] `SubscriptionGroupsResponse` - included group-version discriminator (#1780, `83f3103e`)
- [x] `SubscriptionPricePoint` - adjusted-equalizations relationship (#1778, `64e2f1da`)
- [x] `SubscriptionResponse` - included subscription-version discriminator (#1779, `aa4856bd`)
- [x] `SubscriptionsResponse` - included subscription-version discriminator (#1779, `aa4856bd`)

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

IAP ownership (#1777 at `2b4668b1`; typed models plus request/response and
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
- [x] `InAppPurchaseLocalizationV2CreateRequest`
- [x] `InAppPurchaseLocalizationV2UpdateRequest`
- [x] `InAppPurchaseLocalizationV2Response`
- [x] `InAppPurchaseLocalizationsV2Response`
- [x] `InAppPurchaseImageV2`
- [x] `InAppPurchaseImageV2CreateRequest`
- [x] `InAppPurchaseImageV2UpdateRequest`
- [x] `InAppPurchaseImageV2Response`
- [x] `InAppPurchaseImagesV2Response`

Subscription ownership (#1779 at `aa4856bd`; typed models plus request/response
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
- [x] `SubscriptionLocalizationV2CreateRequest`
- [x] `SubscriptionLocalizationV2UpdateRequest`
- [x] `SubscriptionLocalizationV2Response`
- [x] `SubscriptionLocalizationsV2Response`
- [x] `SubscriptionImageV2`
- [x] `SubscriptionImageV2CreateRequest`
- [x] `SubscriptionImageV2UpdateRequest`
- [x] `SubscriptionImageV2Response`
- [x] `SubscriptionImagesV2Response`

Subscription-group ownership (#1780 at `83f3103e`; typed models plus
request/response and round-trip tests):

- [x] `SubscriptionGroupVersion`
- [x] `SubscriptionGroupVersionCreateRequest`
- [x] `SubscriptionGroupVersionResponse`
- [x] `SubscriptionGroupVersionsResponse`
- [x] `SubscriptionGroupVersionsLinkagesResponse`
- [x] `SubscriptionGroupVersionLocalizationsLinkagesResponse`
- [x] `SubscriptionGroupLocalizationV2`
- [x] `SubscriptionGroupLocalizationV2CreateRequest`
- [x] `SubscriptionGroupLocalizationV2UpdateRequest`
- [x] `SubscriptionGroupLocalizationV2Response`
- [x] `SubscriptionGroupLocalizationsV2Response`

### Exact modified-operation checklist

This checklist contains exactly the 102 operations whose path-item operation
objects differ between 4.4 and 4.4.1: 50 behavior changes that remained at the
immediate pre-PR base plus 52 changes already reconciled after the 4.4 import.
Schema-mediated request-contract changes are listed separately after it and do
not alter this count.

Age rating and app info (#1778 at `64e2f1da`; sparse-field, typed-decoder,
payload, output, and compatibility tests):

- [x] `GET /v1/appInfoLocalizations/{id}`
- [x] `GET /v1/appInfos/{id}`
- [x] `GET /v1/appInfos/{id}/ageRatingDeclaration`
- [x] `GET /v1/appInfos/{id}/appInfoLocalizations`
- [x] `GET /v1/apps`
- [x] `GET /v1/apps/{id}`
- [x] `GET /v1/apps/{id}/appInfos`
- [x] `GET /v1/ciProducts/{id}/app`

IAP and promoted-purchase propagation (#1777 at `2b4668b1`; endpoint-exact
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

Review submissions (#1781 at `77ced0b1`; exact sparse-field/include tests plus
`links`, `included`, and `meta` response round trips):

- [x] `GET /v1/reviewSubmissions`
- [x] `GET /v1/reviewSubmissions/{id}`
- [x] `GET /v1/reviewSubmissions/{id}/items`
- [x] `GET /v1/apps/{id}/reviewSubmissions`

Subscriptions, groups, and pricing (#1778 at `64e2f1da`, #1779 at `aa4856bd`,
and #1780 at `83f3103e`; endpoint-exact query, response, compatibility, and
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
`PATCH /v1/ageRatingDeclarations/{id}` is covered by #1778 at `64e2f1da`;
review-submission item request/response changes are covered by #1781 at
`77ced0b1`; IAP, subscription, and group response propagation is covered by
#1777 at `2b4668b1`, #1779 at `aa4856bd`, and #1780 at `83f3103e`. Checked
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
| 1 | Discrete IAP versions and their localizations/review images | #1777 | 18-operation ledger, CLI/HTTP/upload tests | Implemented at `2b4668b1` |
| 2 | Discrete subscription versions and their localizations/promotional images | #1779 | 18-operation ledger, CLI/HTTP/upload tests | Implemented at `aa4856bd` |
| 3 | Discrete subscription-group versions and their localizations | #1780 | 10-operation ledger and CLI/HTTP tests | Implemented at `83f3103e` |
| 4 | Submit all three version types through review-submission items | #1777 and #1781 | Three exact relationship payload tests plus built-command tests | Implemented at `2b4668b1` and `77ced0b1` |
| 5 | Version-scoped v2 IAP localizations and images | #1777 | CRUD, explicit-null, and upload lifecycle tests | Implemented at `2b4668b1` |
| 6 | Version-scoped v2 subscription localizations and images | #1779 | CRUD, explicit-null, and upload lifecycle tests | Implemented at `aa4856bd` |
| 7 | Version-scoped v2 subscription-group localizations | #1780 | CRUD tests including `customAppName` | Implemented at `83f3103e` |
| 8 | Adjusted subscription equalizations and new filters | #1778 | Exact query/response, option-scope, strict-CSV, territory-inclusion, opaque-next, ID-validation, and aggregation tests | Implemented at `64e2f1da` |
| 9 | `socialMedia` and `socialMediaAgeRestricted` age-rating attributes | #1778 | Payload, output, help, and exit-behavior tests | Implemented at `64e2f1da` |

## Deprecation and migration ledger

Apple deprecates seven resource families in the prose release notes:

| Deprecated family | Replacement API and implemented command | Owner | Compatibility and warning status | Transition evidence |
| --- | --- | --- | --- | --- |
| IAP localizations v1 | `/v2/inAppPurchaseLocalizations`; `asc iap versions localizations ... --version-id` | #1777 | Existing product-ID command preserved; no warning or removal in this compatibility slice | `2b4668b1`; old-command characterization, v2 CRUD/ID, explicit-null, and docs tests |
| IAP images v1 | `/v2/inAppPurchaseImages`; `asc iap versions images ... --version-id` | #1777 | Existing product-ID upload command preserved; no warning or removal in this compatibility slice | `2b4668b1`; old-upload characterization and reserve/upload/commit tests |
| IAP submissions | `/v1/reviewSubmissionItems`; `asc review items add --item-type inAppPurchaseVersions` | #1777 and #1781 | Existing IAP-ID submit shortcut preserved; version-item path is additive | `2b4668b1`, `77ced0b1`; old-submit characterization, exact relationship payload, built-command, and migration-doc tests |
| Subscription localizations v1 | `/v2/subscriptionLocalizations`; `asc subscriptions versions localizations ... --version-id` | #1779 | Existing subscription-ID command preserved; no warning or removal in this compatibility slice | `aa4856bd`; old-command characterization, v2 CRUD/ID, explicit-null, and docs tests |
| Subscription images v1 | `/v2/subscriptionImages`; `asc subscriptions versions images ... --version-id` | #1779 | Existing subscription-ID upload command preserved; no warning or removal in this compatibility slice | `aa4856bd`; old-upload characterization and reserve/upload/commit tests |
| Subscription-group localizations v1 | `/v2/subscriptionGroupLocalizations`; `asc subscriptions groups versions localizations ... --version-id` | #1780 | Existing group-ID command preserved; no warning or removal in this compatibility slice | `83f3103e`; old-command characterization, v2 CRUD/ID, explicit-null, and docs tests |
| Subscription and group submissions | `/v1/reviewSubmissionItems`; item types `subscriptionVersions` and `subscriptionGroupVersions` | #1781 | Existing subscription/group-ID submit shortcuts preserved; version-item paths are additive | `77ced0b1`; two old-submit characterizations, two exact relationship payload tests, built-command tests, and migration notes |

The replacement APIs are version-scoped. Existing stable commands must remain
available while new version-aware commands are introduced. Any later
deprecation of a stable command requires warning text, transition tests,
migration guidance, and a release-note entry. Removal is outside this goal.

## Pull-request sequence and status

1. Schema tooling is implemented in #1776 at `aaa9b62d`.
2. Age rating and pricing are implemented in #1778 at `64e2f1da`.
3. IAP versions are implemented in #1777 at `2b4668b1`.
4. Subscription versions are implemented in #1779 at `aa4856bd`.
5. Subscription-group versions are implemented in #1780 at `83f3103e`.
6. Cross-cutting review integration is implemented in #1781 at `77ced0b1`.
7. External workflow skills are synchronized in draft PR #50 at `5e6fad3d`;
   targeted runtime checks are green and two unavailable optional validators are
   recorded as `UNVERIFIED`.
8. Combined integration is complete at `5f486516`: all six whole PR heads apply
   with zero conflicts or manual edits, all 62 leaf help paths render, and the
   full repository gate is green.

The schema update is merged. Feature PRs target `main` unless a shared-client or
command dependency requires an explicit stack; stacked PRs are retargeted after
their dependency merges. No PR is merged without explicit maintainer approval.

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

The integration closeout independently repeated these checks rather than
trusting the per-PR reports:

- [x] Re-downloaded Apple's official OpenAPI zip and verified the artifact and
  extracted JSON hashes against the repository snapshot.
- [x] Recomputed the 4.4-to-4.4.1 delta: 37 paths, 47 added operations, 47 added
  schemas, zero removals, 102 directly modified operations, and 61 modified
  schemas.
- [x] Mapped all 47 added operations to exact HTTP method/path tests and
  discoverable typed command/client surfaces.
- [x] Classified all 102 direct plus 71 transitive operation-contract changes:
  173 unique existing-operation contracts with no missing or extra ledger item.
- [x] Mapped all 47 added and 61 modified schemas to typed models, documented
  generic decoding, or an already-reconciled schema-only disposition.
- [x] Mapped all nine release-note additions and seven deprecated families to
  commands, compatibility treatment, tests, and migration guidance.
- [x] Exercised all 62 changed or new leaf help paths and checked generated
  command docs, clients, review-item registries, upload helpers, and external
  workflow skills for stale v1 assumptions.
- [x] Applied all six exact PR heads to `main` `25b33c17` with zero conflicts or
  manual edits and passed format, docs, lint, test, and build on clean
  integration SHA `5f486516`.
- [x] Performed read-only live checks where account state permitted, used
  deterministic fixtures for mutations, and performed no live mutations during
  integration. The two unavailable optional external-skill validators remain
  explicitly `UNVERIFIED`; no user-facing behavior is otherwise unverified.

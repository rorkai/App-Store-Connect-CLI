# App Store Connect API 4.4.1 coverage plan

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
| Still-different operations at the immediate pre-PR base | - | - | 50 |
| Still-different schemas at the immediate pre-PR base | - | - | 17 |
| Operation changes already reconciled after the 4.4 import | - | - | 52 |
| Schema changes already reconciled after the 4.4 import | - | - | 44 |

The semantic diff changes only `info.version`, `paths`, and
`components.schemas`; no other component category, security scheme, or
top-level contract changes.

## Definition of done

- Every added operation below is implemented and tested, or marked schema-only
  with a concrete rationale.
- Every one of the 102 modified existing operations is classified as a new
  query/response behavior, a deprecation reversal, or a change already covered
  before the schema PR.
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
| `POST` | `/v1/inAppPurchaseVersions` | Create a version for an IAP relationship | Planned typed command | IAP version PR | Pending |
| `GET` | `/v1/inAppPurchaseVersions/{id}` | View a version | Planned typed command | IAP version PR | Pending |
| `GET` | `/v2/inAppPurchases/{id}/versions` | List related versions with pagination | Planned typed command | IAP version PR | Pending |
| `GET` | `/v2/inAppPurchases/{id}/relationships/versions` | List version linkages | Planned typed client/command | IAP version PR | Pending |
| `GET` | `/v1/inAppPurchaseVersions/{id}/localizations` | List version localizations | Planned typed command | IAP version PR | Pending |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/localizations` | List localization linkages | Planned typed client/command | IAP version PR | Pending |
| `GET` | `/v1/inAppPurchaseVersions/{id}/image` | Get the singular review image | Planned typed command | IAP version PR | Pending |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/image` | Get singular image linkage | Planned typed client/command | IAP version PR | Pending |
| `GET` | `/v1/inAppPurchaseVersions/{id}/images` | List review images | Planned typed command | IAP version PR | Pending |
| `GET` | `/v1/inAppPurchaseVersions/{id}/relationships/images` | List image linkages | Planned typed client/command | IAP version PR | Pending |
| `POST` | `/v2/inAppPurchaseLocalizations` | Create a version-scoped localization | Planned typed command | IAP version PR | Pending |
| `GET` | `/v2/inAppPurchaseLocalizations/{id}` | View a localization | Planned typed command | IAP version PR | Pending |
| `PATCH` | `/v2/inAppPurchaseLocalizations/{id}` | Update a localization | Planned typed command | IAP version PR | Pending |
| `DELETE` | `/v2/inAppPurchaseLocalizations/{id}` | Delete a localization with confirmation | Planned typed command | IAP version PR | Pending |
| `POST` | `/v2/inAppPurchaseImages` | Reserve and upload a version-scoped image | Planned upload command | IAP version PR | Pending |
| `GET` | `/v2/inAppPurchaseImages/{id}` | View an image and upload state | Planned typed command | IAP version PR | Pending |
| `PATCH` | `/v2/inAppPurchaseImages/{id}` | Commit uploaded parts | Planned upload command | IAP version PR | Pending |
| `DELETE` | `/v2/inAppPurchaseImages/{id}` | Delete an image with confirmation | Planned typed command | IAP version PR | Pending |

The review-submission relationship for `inAppPurchaseVersion` modifies the
existing `/v1/reviewSubmissionItems` operation rather than adding another path.

### Subscription versions and version-scoped metadata: 18

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/subscriptionVersions` | Create a version for a subscription relationship | Planned typed command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptionVersions/{id}` | View a version | Planned typed command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptions/{id}/versions` | List related versions with pagination | Planned typed command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptions/{id}/relationships/versions` | List version linkages | Planned typed client/command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptionVersions/{id}/localizations` | List version localizations | Planned typed command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/localizations` | List localization linkages | Planned typed client/command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptionVersions/{id}/image` | Get the singular promotional image | Planned typed command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/image` | Get singular image linkage | Planned typed client/command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptionVersions/{id}/images` | List promotional images | Planned typed command | Subscription version PR | Pending |
| `GET` | `/v1/subscriptionVersions/{id}/relationships/images` | List image linkages | Planned typed client/command | Subscription version PR | Pending |
| `POST` | `/v2/subscriptionLocalizations` | Create a version-scoped localization | Planned typed command | Subscription version PR | Pending |
| `GET` | `/v2/subscriptionLocalizations/{id}` | View a localization | Planned typed command | Subscription version PR | Pending |
| `PATCH` | `/v2/subscriptionLocalizations/{id}` | Update a localization | Planned typed command | Subscription version PR | Pending |
| `DELETE` | `/v2/subscriptionLocalizations/{id}` | Delete a localization with confirmation | Planned typed command | Subscription version PR | Pending |
| `POST` | `/v2/subscriptionImages` | Reserve and upload a version-scoped image | Planned upload command | Subscription version PR | Pending |
| `GET` | `/v2/subscriptionImages/{id}` | View an image and upload state | Planned typed command | Subscription version PR | Pending |
| `PATCH` | `/v2/subscriptionImages/{id}` | Commit uploaded parts | Planned upload command | Subscription version PR | Pending |
| `DELETE` | `/v2/subscriptionImages/{id}` | Delete an image with confirmation | Planned typed command | Subscription version PR | Pending |

The review-submission relationship for `subscriptionVersion` modifies the
existing `/v1/reviewSubmissionItems` operation.

### Subscription-group versions and localizations: 10

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `POST` | `/v1/subscriptionGroupVersions` | Create a version for a group relationship | Planned typed command | Subscription-group version PR | Pending |
| `GET` | `/v1/subscriptionGroupVersions/{id}` | View a version | Planned typed command | Subscription-group version PR | Pending |
| `GET` | `/v1/subscriptionGroups/{id}/versions` | List related versions with pagination | Planned typed command | Subscription-group version PR | Pending |
| `GET` | `/v1/subscriptionGroups/{id}/relationships/versions` | List version linkages | Planned typed client/command | Subscription-group version PR | Pending |
| `GET` | `/v1/subscriptionGroupVersions/{id}/localizations` | List version localizations | Planned typed command | Subscription-group version PR | Pending |
| `GET` | `/v1/subscriptionGroupVersions/{id}/relationships/localizations` | List localization linkages | Planned typed client/command | Subscription-group version PR | Pending |
| `POST` | `/v2/subscriptionGroupLocalizations` | Create a version-scoped localization | Planned typed command | Subscription-group version PR | Pending |
| `GET` | `/v2/subscriptionGroupLocalizations/{id}` | View a localization | Planned typed command | Subscription-group version PR | Pending |
| `PATCH` | `/v2/subscriptionGroupLocalizations/{id}` | Update a localization | Planned typed command | Subscription-group version PR | Pending |
| `DELETE` | `/v2/subscriptionGroupLocalizations/{id}` | Delete a localization with confirmation | Planned typed command | Subscription-group version PR | Pending |

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
| IAP | `inAppPurchaseVersion.data.type=inAppPurchaseVersions` | `inAppPurchaseVersions` | HTTP body test plus built command test | Planned in IAP version PR |
| Subscription | `subscriptionVersion.data.type=subscriptionVersions` | `subscriptionVersions` | HTTP body test plus built command test | Planned cross-cutting review integration |
| Subscription group | `subscriptionGroupVersion.data.type=subscriptionGroupVersions` | `subscriptionGroupVersions` | HTTP body test plus built command test | Planned cross-cutting review integration |

The exact modified-operation checklist includes the `POST` operation as well as
the four review-submission read operations whose sparse fields and includes
gain the three version relationships.

### Subscription adjusted equalizations: 1

| Method | Path | Required behavior | Disposition | Owner | Evidence |
| --- | --- | --- | --- | --- | --- |
| `GET` | `/v1/subscriptionPricePoints/{id}/adjustedEqualizations` | List adjusted equalized price points using the exact territory, subscription, upfront-price-point, and plan-type filters supported by this operation | Planned typed command | Age/pricing PR | Pending |

## Modified existing contract ledger

The original 4.4-to-4.4.1 diff modifies 102 existing operations. Fifty remain
different from the immediate pre-PR repository snapshot and expand query or
response contracts. The other 52 were already reconciled in the repository
after the original 4.4 import: 44 reverse OpenAPI `deprecated: true` flags and
eight add media-localization sparse-field parameters. Each item remains in the
checklist so "already covered" is an audited disposition rather than an omitted
change.

| Contract area | Semantic change | Verification owner |
| --- | --- | --- |
| IAP reads | `fields[inAppPurchases]` gains `versions`; IAP detail and app-IAP collection reads gain `include=versions`, `fields[inAppPurchaseVersions]`, and `limit[versions]` | IAP version PR |
| Subscription reads | Subscription detail and group-subscription reads gain version includes, sparse fields, and relationship limits; subscription sparse fields gain `versions` across related endpoints | Subscription version PR |
| Subscription-group reads | Group detail and app-group collection reads gain version includes, sparse fields, and relationship limits; group sparse fields gain `versions` | Subscription-group version PR |
| Review submission reads | Review-item sparse fields and includes gain `inAppPurchaseVersion`, `subscriptionVersion`, and `subscriptionGroupVersion` | Review-submission portions of all three version PRs |
| Pricing reads | Price-point sparse fields gain `adjustedEqualizations`; existing equalization and price-point relationship operations gain `filter[upfrontPricePointId]` and `filter[planType]` where allowed | Age/pricing PR |
| Age rating reads and update | Age-rating sparse fields and update schema gain `socialMedia` and `socialMediaAgeRestricted` | Age/pricing PR |
| App info reads | `AppInfo.attributes.kidsAgeBand` and `fields[appInfos]=kidsAgeBand` appear as deprecated additions | Characterize current generic decoding/output; do not introduce a new write path |
| Included-resource unions | IAP, subscription, group, and review-submission responses gain their corresponding version resource discriminators | Owning typed-client tests |

No existing operation changes from nondeprecated to `deprecated: true` in the
OpenAPI JSON. Forty-four operations instead reverse `deprecated: true` from the
original 4.4 snapshot, primarily screenshot and preview resources. Separately,
Apple's prose release notes deprecate the seven version-replaced resource
families below without setting new operation-level flags. Deprecation behavior
therefore cannot be inferred solely from OpenAPI flags.

## Added and modified schema ledger

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

- [ ] `AgeRatingDeclaration` - two social-media Boolean attributes
- [ ] `AgeRatingDeclarationUpdateRequest` - two nullable social-media update attributes
- [ ] `AppInfo` - deprecated `kidsAgeBand` read attribute
- [ ] `InAppPurchaseV2` - versions relationship
- [ ] `InAppPurchaseV2Response` - included IAP-version discriminator
- [ ] `InAppPurchasesV2Response` - included IAP-version discriminator
- [ ] `ReviewSubmissionItem` - three version relationships
- [ ] `ReviewSubmissionItemCreateRequest` - three version create relationships
- [ ] `ReviewSubmissionItemResponse` - three included version discriminators
- [ ] `ReviewSubmissionItemsResponse` - three included version discriminators
- [ ] `Subscription` - versions relationship
- [ ] `SubscriptionGroup` - versions relationship
- [ ] `SubscriptionGroupResponse` - included group-version discriminator
- [ ] `SubscriptionGroupsResponse` - included group-version discriminator
- [ ] `SubscriptionPricePoint` - adjusted-equalizations relationship
- [ ] `SubscriptionResponse` - included subscription-version discriminator
- [ ] `SubscriptionsResponse` - included subscription-version discriminator

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

IAP ownership:

- [ ] `InAppPurchaseVersion`
- [ ] `InAppPurchaseVersionCreateRequest`
- [ ] `InAppPurchaseVersionResponse`
- [ ] `InAppPurchaseVersionsResponse`
- [ ] `InAppPurchaseV2VersionsLinkagesResponse`
- [ ] `InAppPurchaseVersionImageLinkageResponse`
- [ ] `InAppPurchaseVersionImagesLinkagesResponse`
- [ ] `InAppPurchaseVersionLocalizationsLinkagesResponse`
- [ ] `InAppPurchaseLocalizationV2`
- [ ] `InAppPurchaseLocalizationV2CreateRequest`
- [ ] `InAppPurchaseLocalizationV2UpdateRequest`
- [ ] `InAppPurchaseLocalizationV2Response`
- [ ] `InAppPurchaseLocalizationsV2Response`
- [ ] `InAppPurchaseImageV2`
- [ ] `InAppPurchaseImageV2CreateRequest`
- [ ] `InAppPurchaseImageV2UpdateRequest`
- [ ] `InAppPurchaseImageV2Response`
- [ ] `InAppPurchaseImagesV2Response`

Subscription ownership:

- [ ] `SubscriptionVersion`
- [ ] `SubscriptionVersionCreateRequest`
- [ ] `SubscriptionVersionResponse`
- [ ] `SubscriptionVersionsResponse`
- [ ] `SubscriptionVersionsLinkagesResponse`
- [ ] `SubscriptionVersionImageLinkageResponse`
- [ ] `SubscriptionVersionImagesLinkagesResponse`
- [ ] `SubscriptionVersionLocalizationsLinkagesResponse`
- [ ] `SubscriptionLocalizationV2`
- [ ] `SubscriptionLocalizationV2CreateRequest`
- [ ] `SubscriptionLocalizationV2UpdateRequest`
- [ ] `SubscriptionLocalizationV2Response`
- [ ] `SubscriptionLocalizationsV2Response`
- [ ] `SubscriptionImageV2`
- [ ] `SubscriptionImageV2CreateRequest`
- [ ] `SubscriptionImageV2UpdateRequest`
- [ ] `SubscriptionImageV2Response`
- [ ] `SubscriptionImagesV2Response`

Subscription-group ownership:

- [ ] `SubscriptionGroupVersion`
- [ ] `SubscriptionGroupVersionCreateRequest`
- [ ] `SubscriptionGroupVersionResponse`
- [ ] `SubscriptionGroupVersionsResponse`
- [ ] `SubscriptionGroupVersionsLinkagesResponse`
- [ ] `SubscriptionGroupVersionLocalizationsLinkagesResponse`
- [ ] `SubscriptionGroupLocalizationV2`
- [ ] `SubscriptionGroupLocalizationV2CreateRequest`
- [ ] `SubscriptionGroupLocalizationV2UpdateRequest`
- [ ] `SubscriptionGroupLocalizationV2Response`
- [ ] `SubscriptionGroupLocalizationsV2Response`

### Exact modified-operation checklist

Age rating and app info:

- [ ] `GET /v1/appInfoLocalizations/{id}`
- [ ] `GET /v1/appInfos/{id}`
- [ ] `GET /v1/appInfos/{id}/ageRatingDeclaration`
- [ ] `GET /v1/appInfos/{id}/appInfoLocalizations`
- [ ] `GET /v1/apps`
- [ ] `GET /v1/apps/{id}`
- [ ] `GET /v1/apps/{id}/appInfos`
- [ ] `GET /v1/ciProducts/{id}/app`

IAP and promoted-purchase propagation:

- [ ] `GET /v1/apps/{id}/inAppPurchasesV2`
- [ ] `GET /v1/inAppPurchaseAppStoreReviewScreenshots/{id}`
- [ ] `GET /v1/inAppPurchaseContents/{id}`
- [ ] `GET /v1/inAppPurchaseImages/{id}`
- [ ] `GET /v1/inAppPurchaseLocalizations/{id}`
- [ ] `GET /v1/promotedPurchases/{id}`
- [ ] `GET /v2/inAppPurchases/{id}`
- [ ] `GET /v2/inAppPurchases/{id}/appStoreReviewScreenshot`
- [ ] `GET /v2/inAppPurchases/{id}/content`
- [ ] `GET /v2/inAppPurchases/{id}/images`
- [ ] `GET /v2/inAppPurchases/{id}/inAppPurchaseLocalizations`
- [ ] `GET /v2/inAppPurchases/{id}/promotedPurchase`
- [ ] `GET /v1/apps/{id}/promotedPurchases`

Review submissions:

- [ ] `POST /v1/reviewSubmissionItems` - schema-mediated request-contract change for three version relationships
- [ ] `GET /v1/reviewSubmissions`
- [ ] `GET /v1/reviewSubmissions/{id}`
- [ ] `GET /v1/reviewSubmissions/{id}/items`
- [ ] `GET /v1/apps/{id}/reviewSubmissions`

Subscriptions, groups, and pricing:

- [ ] `GET /v1/apps/{id}/subscriptionGroups`
- [ ] `GET /v1/subscriptionAppStoreReviewScreenshots/{id}`
- [ ] `GET /v1/subscriptionGroupLocalizations/{id}`
- [ ] `GET /v1/subscriptionGroups/{id}`
- [ ] `GET /v1/subscriptionGroups/{id}/subscriptionGroupLocalizations`
- [ ] `GET /v1/subscriptionGroups/{id}/subscriptions`
- [ ] `GET /v1/subscriptionImages/{id}`
- [ ] `GET /v1/subscriptionLocalizations/{id}`
- [ ] `GET /v1/subscriptionOfferCodes/{id}`
- [ ] `GET /v1/subscriptionOfferCodes/{id}/prices`
- [ ] `GET /v1/subscriptionPricePoints/{id}`
- [ ] `GET /v1/subscriptionPricePoints/{id}/equalizations`
- [ ] `GET /v1/subscriptionPromotionalOffers/{id}`
- [ ] `GET /v1/subscriptionPromotionalOffers/{id}/prices`
- [ ] `GET /v1/subscriptions/{id}`
- [ ] `GET /v1/subscriptions/{id}/appStoreReviewScreenshot`
- [ ] `GET /v1/subscriptions/{id}/images`
- [ ] `GET /v1/subscriptions/{id}/introductoryOffers`
- [ ] `GET /v1/subscriptions/{id}/offerCodes`
- [ ] `GET /v1/subscriptions/{id}/pricePoints`
- [ ] `GET /v1/subscriptions/{id}/prices`
- [ ] `GET /v1/subscriptions/{id}/promotedPurchase`
- [ ] `GET /v1/subscriptions/{id}/promotionalOffers`
- [ ] `GET /v1/subscriptions/{id}/subscriptionLocalizations`
- [ ] `GET /v1/winBackOffers/{id}/prices`

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

## Release-note capability ledger

| # | Apple addition | Owner | Verification | Status |
| ---: | --- | --- | --- | --- |
| 1 | Discrete IAP versions and their localizations/review images | IAP version PR | 18-operation ledger, CLI/HTTP/upload tests | Planned |
| 2 | Discrete subscription versions and their localizations/promotional images | Subscription version PR | 18-operation ledger, CLI/HTTP/upload tests | Planned |
| 3 | Discrete subscription-group versions and their localizations | Subscription-group version PR | 10-operation ledger and CLI/HTTP tests | Planned |
| 4 | Submit all three version types through review-submission items | IAP plus cross-cutting review integration | Three exact relationship payload tests | Planned |
| 5 | Version-scoped v2 IAP localizations and images | IAP version PR | CRUD and upload lifecycle tests | Planned |
| 6 | Version-scoped v2 subscription localizations and images | Subscription version PR | CRUD and upload lifecycle tests | Planned |
| 7 | Version-scoped v2 subscription-group localizations | Subscription-group version PR | CRUD tests including `customAppName` | Planned |
| 8 | Adjusted subscription equalizations and new filters | Age/pricing PR | Exact query and response tests | Planned |
| 9 | `socialMedia` and `socialMediaAgeRestricted` age-rating attributes | Age/pricing PR | Payload, output, help, and exit-behavior tests | Planned |

## Deprecation and migration ledger

Apple deprecates seven resource families in the prose release notes:

| Deprecated family | Replacement API and planned command | Owner | Compatibility and warning status | Required transition evidence |
| --- | --- | --- | --- | --- |
| IAP localizations v1 | `/v2/inAppPurchaseLocalizations`; `asc iap versions localizations ... --version-id` | IAP version PR | Keep existing product-ID command unchanged; no warning until replacement ships | Old command characterization plus new v2 CRUD/ID tests |
| IAP images v1 | `/v2/inAppPurchaseImages`; `asc iap versions images ... --version-id` | IAP version PR | Keep existing product-ID upload command unchanged; no warning until replacement ships | Old upload characterization plus new reserve/upload/commit tests |
| IAP submissions | `/v1/reviewSubmissionItems`; `asc review items add --item-type inAppPurchaseVersions` | IAP version PR | Keep existing IAP-ID submit shortcut unchanged; no warning until version path ships | Old submit characterization and exact new relationship payload test |
| Subscription localizations v1 | `/v2/subscriptionLocalizations`; `asc subscriptions versions localizations ... --version-id` | Subscription version PR | Keep existing subscription-ID command unchanged; no warning until replacement ships | Old command characterization plus new v2 CRUD/ID tests |
| Subscription images v1 | `/v2/subscriptionImages`; `asc subscriptions versions images ... --version-id` | Subscription version PR | Keep existing subscription-ID upload command unchanged; no warning until replacement ships | Old upload characterization plus new reserve/upload/commit tests |
| Subscription-group localizations v1 | `/v2/subscriptionGroupLocalizations`; `asc subscriptions groups versions localizations ... --version-id` | Subscription-group version PR | Keep existing group-ID command unchanged; no warning until replacement ships | Old command characterization plus new v2 CRUD/ID tests |
| Subscription and group submissions | `/v1/reviewSubmissionItems`; item types `subscriptionVersions` and `subscriptionGroupVersions` | Cross-cutting review integration | Keep existing subscription/group-ID submit shortcuts unchanged; no warning until version paths ship | Two old-submit characterizations and two exact new relationship payload tests |

The replacement APIs are version-scoped. Existing stable commands must remain
available while new version-aware commands are introduced. Any later
deprecation of a stable command requires warning text, transition tests,
migration guidance, and a release-note entry. Removal is outside this goal.

## Pull-request sequence

1. Schema tooling: relationship-aware request discovery and stale-index checks.
2. Age rating and pricing: two Boolean age fields, adjusted equalizations, and
   characterization of the deprecated `AppInfo.kidsAgeBand` addition.
3. IAP versions: version reads/creation, v2 localizations/images, upload flow,
   review-submission integration, and compatibility guidance.
4. Subscription versions: version reads/creation, v2 localizations/images,
   upload flow, review-submission integration, and compatibility guidance.
5. Subscription-group versions: version reads/creation, v2 localizations,
   review-submission integration, and compatibility guidance.
6. Integration closeout: generated documentation, external skills, complete
   ledger reconciliation, live validation, and any narrowly proven fixes.

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

The integration closeout must independently repeat all of these checks rather
than trusting the per-PR reports:

1. Re-download Apple's official OpenAPI zip and verify its hash/version.
2. Recompute added, removed, and modified operations and schemas from 4.4.
3. Map all 47 added operations to code, tests, docs, or an explicit schema-only
   decision.
4. Map all 102 directly modified operations plus the schema-mediated
   `POST /v1/reviewSubmissionItems` contract change to query, payload,
   deprecation, or compatibility coverage.
5. Map all 47 added and 61 modified schemas to typed models, documented generic
   decoding, or an already-reconciled schema-only disposition.
6. Map all nine release-note additions and seven deprecations to commands and
   migration guidance.
7. Search built help, generated command docs, Go clients, review-item type
   registries, upload helpers, and workflow skills for stale v1 assumptions.
8. Run the full gate from a clean integration head and verify every GitHub
   review thread and check against that exact SHA.
9. Record live verification and any behavior that remains `UNVERIFIED`.

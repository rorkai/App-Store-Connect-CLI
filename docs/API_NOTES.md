# API Notes

Operational contracts, endpoint quirks, filter/include rules, and live-verification status for App Store Connect, Apple Ads, and Developer Portal surfaces.

Dated maintainer canary transcripts, authenticated browser/source captures, and disposable test fixture records live in the repository at [API_CANARY_LOG.md](API_CANARY_LOG.md) and are intentionally excluded from the embedded `asc docs show api-notes` guide to keep CLI output concise.

## Web App Privacy Data Usage Updates

- `PATCH /iris/v1/appDataUsages/{usageID}` supports only a same-category, same-purpose identity flip between `DATA_LINKED_TO_YOU` and `DATA_NOT_LINKED_TO_YOU` using the `appDataUsages` JSON:API resource and its `dataProtection` relationship (live-verified).
- Tracking, `DATA_NOT_COLLECTED`, and scope changes must use `DELETE` (`204`) + `POST` (`201`); direct `POST` or `PATCH` to `DATA_NOT_COLLECTED` fails with `409 STATE_ERROR`.
- Direct mutations advance `lastPublished` while `published` remains `true` without calling a publish endpoint. Treat `asc web privacy apply` as a published-state mutation.

## Apple Ads Profile Context Isolation

- Named Apple Ads profiles use only the context stored on that profile and never inherit `ads.org_id` or `ads.ad_account_id` from root config or another profile. Profile-less access-token and environment authentication can still use matching root context.

## Public App Store Ranking

- `asc apps public rank` is an unauthenticated storefront read: iOS inspects up to 200 results from the public iTunes `/search` endpoint, while Apple TV uses the undocumented MZStore search endpoint with `X-Apple-Store-Front: <numeric-storefront-id>,33` (available only for countries with a known numeric storefront ID).
- `found: false` means only that the app was absent from the returned result window.

## Analytics & Sales Reports

- Sales Reports require `YYYY-MM` for monthly reports and `YYYY` for yearly reports on the wire; the CLI accepts `YYYY-MM-DD` and reduces it to the live period identifier before sending.
- Vendor number comes from Sales and Trends → Reports URL (`vendorNumber=...`).
- Sales Reports validates the report type/subtype/frequency/version tuple against Apple's endpoint table. Both `SUBSCRIPTION` `1_3` and `1_4` are accepted (`1_4` is the default and live-verified).
- Use `--paginate` with `asc analytics view --processing-date` (`filter[processingDate]`) to search every report page, or `--next <links.next> --paginate` to resume. Use `--granularity "DAILY,WEEKLY,MONTHLY"` to filter by granularity. Raise `ASC_TIMEOUT` for long runs.

## Finance Reports

Finance reports use Apple fiscal months (`YYYY-MM`), not calendar months.

| API `--report-type` | UI Option | `--region` Code(s) |
|---|---|---|
| `FINANCIAL` | All Countries or Regions (Single File) | `ZZ` (consolidated) |
| `FINANCIAL` | All Countries or Regions (Multiple Files) | `US`, `EU`, `JP`, etc. |
| `FINANCE_DETAIL` | All Countries or Regions (Detailed) | `Z1` (required) |
| Not available | Transaction Tax (Single File) | N/A |

- `FINANCE_DETAIL` requires region code `Z1`.
- Transaction Tax reports are not exposed in `/v1/financeReports` or `/v1/salesReports`; use `asc web finance transaction-tax download` or App Store Connect.
- Region codes: `asc finance regions` or https://developer.apple.com/help/app-store-connect/reference/financial-report-regions-and-currencies/.

## Tax Categories and Transaction Tax Reports

- The public API has no tax-category resource or attribute on `apps`, `appInfos`, or `inAppPurchases`. `asc capabilities --area monetization` reports App Information and IAP tax categories as web-session coverage.
- **App Information (`asc web apps tax-category list|view|set`)**:
  - Catalog read: `GET /iris/v1/taxCategories?filter[productType]=APPLICATION&include=subcategories,conditions&limit[subcategories]=100&limit[conditions]=100`.
  - App read: `GET /iris/v1/appTaxCategories/{appId}?include=category,enabledConditions&limit[enabledConditions]=100`. A missing resource means unconfigured (UI default: App Store Software).
  - `set` requires `--confirm`, validates category and condition IDs against the catalog, sends the complete condition set (`enabledConditions.data=[]` when `--condition` is omitted), and verifies via read-back without retrying ambiguous writes. Explicit creation/readback is live-verified; PATCH, condition changes, and account-specific errors remain unverified.
- **In-App Purchases (`asc web iap tax-category list|view|set|reset`)**:
  - Catalog read uses `filter[productType]=ADDON` and preserves the raw JSON:API envelope.
  - `view` reads `GET /iris/v2/inAppPurchases/{iapId}?include=inAppPurchaseTaxCategoryInfo`. Explicit `inAppPurchaseTaxCategoryInfo.data: null` means inherited from the parent app (raw discovery envelope preserved). When configured, the CLI reads `GET /iris/v1/inAppPurchaseTaxCategoryInfos/{taxInfoId}?include=category,enabledConditions,inAppPurchaseV2&limit[enabledConditions]=100` and verifies the owner IAP ID.
  - `set` (`--confirm`) validates against the `ADDON` catalog, creates when inherited or patches when configured, sends at most one write plus one verification read, and skips matching state as a no-op.
  - `reset` (`--confirm`) deletes only the `inAppPurchaseTaxCategoryInfos` override and verifies an explicit-null relationship (no-op when already inherited). Browser contract is verified; final CLI live execution remains unverified.

## Sandbox Testers

- List, view, update, and clear-history use the public v2 API through `asc sandbox`.
- `asc web sandbox create` requires `--first-name`, `--last-name`, `--email`, `--password` (8+ chars with uppercase, lowercase, and digit), and `--territory`. Territory inputs accept alpha-2, alpha-3, or exact English country names and normalize to 3-letter ASC territory codes (limited to verified ASC alpha-3 surfaces).
- Web create runs two `POST /sandbox/v2/account/validateFields` requests followed by `POST /sandbox/v2/account/create` with `firstName`, `lastName`, `acAccountName`, `acAccountPassword`, and `storeFront` (source-backed; live create acceptance is unverified).
- Web list uses `GET /sandbox/v2/provider/account/list?limit=50` (no pagination contract captured; rows include `id` and `isInFamily`).
- `asc web sandbox delete` (`--confirm`) sends `POST /sandbox/v2/account/delete` with `{"ids":[...]}`, refuses family members and incomplete list snapshots, sends one request, and verifies a fresh list. Delete status/body and post-delete disappearance remain unverified against live data.

## App Store Regulations & Permits declarations

- The public OpenAPI snapshot has no medical-device, personal-service, trader, or DSA declaration endpoints (`TerritoryAvailability.contentStatuses` exposes only read-only trader status consequence codes).
- Declarations use the web-session `ppm/complianceform/v1` service with UI headers (`X-Csrf-Itc: itc`, `Origin`, and `/apps/{id}/distribution/info` `Referer`):
  - `asc web apps declarations list`: `GET /ppm/complianceform/v1/accounts/{accountId}/requirements?contentId={appId}`. Prefers `requirementData` matching `contentId`, falling back to empty `contentId`.
  - `asc web apps medical-device view`: `GET /ppm/complianceform/v1/accounts/{accountId}/requirements/{requirementId}/forms?contentId={appId}`, reading `medicalDeviceData.declaration` (`no`, `yes`, or absent) alongside `$[*]`-rooted `constraints`.
  - `asc web apps medical-device set`: uses `POST .../contents/{appId}/requirements/{requirementId}/forms` when `medicalDeviceData.declaration` is absent and `PUT` when present. `--declared false` derives regions from form constraints, preserves existing regional rows, and skips writing when already `no` + `COLLECTED`. `--declared true [--countries-or-regions EEA,GBR,USA] --confirm` limits regions to `EEA`, `GBR`, `USA` (defaulting to all three), updates `registrationInfo` rows, and verifies readback.
  - `asc web apps medical-device region set --app APP_ID --region REGION --input PATH --confirm`: sends `PUT .../forms` with rootfs-anchored `declaration`, localized `supportInfo`, and `registrationNumber` (for `USA`/`EEA`). Requires app-level declaration `yes` and region availability in `$[*].countriesOrRegions`. Performs contact preflight (legal entity, region coverage, non-empty phone/email/address, no missing constraint legal-entity candidates), sends one PUT, and verifies readback. Negative regional and restore paths are live-verified; arbitrary affirmative legal/contact values remain provider-dependent.
- Personal-service declarations (`personalServiceDeclaration`) are not implemented (no live fixture). EU DSA trader status (`GET /ppm/v1/accounts/{id}/sellerInfo`, `POST /ppm/v1/legalEntities/{id}/sellerInfo`) requires interactive `id.apple.com` identity verification and is out of scope for unattended CLI writes.

## Web-session API keys

- `asc web api-keys list` combines iris v1 team keys (`GET /iris/v1/apiKeys?include=createdBy,revokedBy,provider&sort=-isActive,-revokingDate&limit=2000`) and iris v2 individual keys (`GET /iris/v2/apiKeys?include=visibleApps,createdByActor,revokedByActor&limit[visibleApps]=3&limit=2000`), following `links.next` internally (no `--paginate`). Use `asc web auth capabilities --key-id` to resolve actor-backed roles when an individual key's `roles` array is empty.
- `asc web api-keys view --key-id` reads `GET /iris/v1/apiKeys/{id}?include=provider` (team keys only). Creation dates are absent from the payload and private key material is never printed.
- `asc web api-keys create-individual --user-id USER_UUID --output-dir DIR --confirm` checks `GET /iris/v1/apiAccesses` (`APPROVED`) plus fail-closed `GET /iris/v1/users/{userUUID}` and session-email preflight, stages a local `0600` PKCS#8 P-256 private key under `--output-dir`, sends `POST /iris/v2/apiKeys` (`{data:{type:"apiKeys"}}`), resolves the single new active key from the actor-filtered v2 list (`filter[createdByActor]=USER:{actorID}`), atomically renames the key to `ApiKey_<KEY_ID>.p8` without replacing existing files, registers the SPKI public key via `PATCH /iris/v2/apiKeys/{KEY_ID}`, and verifies active readback. Uncertain steps retain the local artifact and never retry automatically. Source-backed; live individual key creation remains unverified.
- `asc web api-keys revoke --key-id KEY_ID --type team|individual --confirm` sends `PATCH /iris/{v1|v2}/apiKeys/{id}` with `{"data":{"id":"<id>","type":"apiKeys","attributes":{"isActive":false}}}` after pre-listing the requested family, and verifies inactive post-state without retrying 5xx/transport failures. Team create/revoke is live-verified; individual revoke remains unverified.

## Web app availability (iris)

- `GET /iris/v1/apps/{id}/appAvailabilityV2` returns `availableInNewTerritories` and links-only `relationships.territoryAvailabilities`; `?include=availableTerritories&limit[availableTerritories]=200` fails with 400 `PARAMETER_ERROR.INVALID`. When unconfigured, it can return `data: null` (treated as absent by `asc web apps availability create`).
- Territory availability reads use `GET /iris/v2/appAvailabilities/{id}/territoryAvailabilities?include=territory&limit=200` (follow `links.next`; `filter[available]=true` returns 400 `PARAMETER_ERROR.ILLEGAL`, so filter client-side). `asc web apps delete` uses this collection for its removed-from-sale preflight.
- On the public API, `GET /v1/apps/{id}/appAvailabilityV2` and `GET /v2/inAppPurchases/{id}/inAppPurchaseAvailability` return 404 `NOT_FOUND` naming `type 'appAvailabilities'` or `'inAppPurchaseAvailabilities'` when unconfigured (vs `'apps'` / `'inAppPurchases'` when the parent is missing); `asc pricing availability view` treats the unconfigured shape as an expected negative (exit 1, `state_not_ready`).
- When an app has never had a price schedule, `GET /v1/apps/{id}/appPriceSchedule` returns a synthetic schedule resource, and `GET /v1/appPriceSchedules/{id}/manualPrices` returns 404 `There is no resource of type 'null' with id '{appId}'`. `asc pricing current` treats that (and a direct schedule 404 or missing current base price) as an expected negative (`configured: false` or `reason: no_current_base_price`).
- `asc web removed-apps restore` patches `/iris/v1/apps/{id}` with `removed:false`, verifies restoration, and only then POSTs `/iris/v1/userAppPermissions` (`GRANT` or `REVOKE` for `ALL_SILOABLE_USERS`).

## App price schedules (`POST /v1/appPriceSchedules`)

- `POST /v1/appPriceSchedules` replaces the whole schedule. Known HTTP 409 conflict shapes (fixtures in `internal/cli/cmdtest/testdata/pricing_schedule_create_409/`):
  - Past `startDate`: `ENTITY_ERROR.INVALID_START_DATE` (`Interval can not have a start date in the past`).
  - Single future `startDate` without covering today: `ENTITY_ERROR.INVALID_START_DATE` (`Entire timeline must be covered for USA...`).
  - Manual price territory mismatching `baseTerritory`: `ENTITY_ERROR.BASE_TERRITORY_INTERVAL_REQUIRED`.
  - Unknown or cross-app price point ID: `ENTITY_ERROR.NOT_FOUND`.
- App Store Connect evaluates "today" and schedule interval boundaries at **US Pacific midnight** (`America/Los_Angeles`) with exclusive end dates, and stores an accepted today `startDate` as `null`. `asc pricing`, `asc app-setup pricing`, `asc iap pricing`, `asc subscriptions pricing`, and `asc validate` all evaluate current/resolved prices and default start dates using the US Pacific date.

## Developer Portal iCloud containers

- `asc web icloud-containers list` sends `POST /services-account/v1/cloudContainers?filter[AND][hidden]=false` (or `true` with `--hidden`) with `X-HTTP-Method-Override: GET`, `teamId`, and `urlEncodedQueryParams=limit=1000&offset=0&sort=name` (bounded first page, no `--paginate`; warns on stderr if continuation/total indicates truncation).
- `asc web icloud-containers create --identifier iCloud.ID --name NAME --confirm` requires the exact `iCloud.` prefix, preflights both visible and hidden collections (failing closed on truncation or existing matches), sends `POST /services-account/v1/cloudContainers` with `{"data":{"type":"cloudContainers","attributes":{"identifier":...,"name":...,"teamId":...}}}` (no root `teamId`), and verifies the new container across both collections without ever retrying (iCloud containers cannot be deleted; live-verified).

## Web-session Resolution Center

- Resolution Center has no public OpenAPI surface; all calls use `/iris/v1` with Apple ID web-session auth and follow `links.next` internally (no `--paginate`).
- Submission scope (`GET /iris/v1/resolutionCenterThreads?filter[reviewSubmission]={id}&include=reviewSubmission`, used by `asc web review show`) and app scope (`GET /iris/v1/apps/{appId}/resolutionCenterThreads?include=appStoreVersions,app,appMessageThreadDetail,build,betaBackgroundAssetReviewSubmission&limit[appStoreVersions]=2000&filter[threadType]=REJECTION_BINARY,REJECTION_METADATA,REJECTION_REVIEW_SUBMISSION,APP_MESSAGE_ARC,APP_MESSAGE_ARB,APP_MESSAGE_COMM,APP_MESSAGE_INFORMATIONAL`, used by `asc web review threads --app`) are distinct; send the exact known-good query shapes because narrowed/modified includes return 400.
- Unsent draft replies live at `GET /iris/v1/resolutionCenterThreads/{threadId}/resolutionCenterDraftMessage?include=resolutionCenterMessageAttachments,fromActor&limit[resolutionCenterMessageAttachments]=1000`. Explicit `data: null` or HTTP 404 means no draft; a response missing `data` is malformed and fails closed. `asc web review threads --drafts` omits signed attachment download URLs.
- Mutations (`POST /iris/v1/resolutionCenterDraftMessages`, `PATCH`/`DELETE` on the draft resource, and `POST /iris/v1/resolutionCenterMessages` with `createFromDraftMessage` for `asc web review reply` and `asc web review drafts create|update|delete`) require `--confirm`, never upload attachments, never retry ambiguous writes, and remain unverified against a live fixture.

## Web-session app create conflicts (`POST /iris/v1/apps`)

- Re-creating an existing app returns HTTP 409 with three `errors[]` entries whose codes are, in order, `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`, `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`, and `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.SAME_ACCOUNT` (captured live on 2026-09-29; `detail`, `title`, and `source` were not captured), so `APIError.AllCodes()` must inspect every entry in `errors[]` rather than only the first. `asc web apps create --if-exists skip` keys on either code and resolves the existing app via the public API (`GET /v1/apps?filter[bundleId]=`, then `filter[sku]=`).

## Web-session app distribution method

- `distributionType` (`APP_STORE` or `CUSTOM`) and `educationDiscountType` (`DISCOUNTED`, `NOT_DISCOUNTED`, `NOT_APPLICABLE`) exist only on `/iris/v1/apps/{id}` (no sparse fieldset). Missing `distributionType` is rendered as `unknown` in tables and omitted in JSON.
- `asc web apps distribution set --app APP_ID --method public|private --confirm` preflights `GET /iris/v1/apps/{id}`, rejects `DIRECT_URL` or unknown methods, sends one `PATCH /iris/v1/apps/{id}` with `educationDiscountType` and conditional `distributionType`, and verifies via read-back without retries (uncertain outcomes return `status: "uncertain"`). Source-backed; live mutation remains unverified. Unlisted App Store distribution has no captured endpoint.

## Web-session custom-distribution Apple Account recipients

- Private JSON:API under `/iris/v1` (`customAppUsers` only; `customAppOrganizations` is unsupported):
  - `asc web apps distribution users list --app APP_ID [--paginate]`: reads `GET /iris/v1/apps/{appId}/customAppUsers?limit=100` and validates pagination invariants strictly.
  - `create --app APP_ID --recipient-apple-id APPLE_ACCOUNT --confirm` (`POST /iris/v1/customAppUsers` -> `201`) and `delete --app APP_ID --id RECIPIENT_ID --confirm` (`DELETE /iris/v1/customAppUsers/{recipientID}` -> `204`) require the app to be in `distributionType: CUSTOM` (never switching distribution implicitly), send at most one write without retry, and verify against a fresh complete collection read. Browser contract is live-verified; final CLI live execution remains unverified.

## App transfer status (web session)

- `asc web apps transfer status --app APP_ID` reads `GET /iris/v1/apps/{appId}?include=appTransferRequest` (live-verified for `data: null`). Do not call `/iris/v1/apps/{appId}/appTransferRequest` directly (returns HTTP 409 `ENTITY_ERROR.RELATIONSHIP.INVALID` when no transfer exists). Transfer initiation/acceptance actions have no captured API contract.

## Last-compatible version settings (`downloadable`)

- Last-compatible version settings use the nullable boolean `downloadable` attribute on public `appStoreVersions` (`asc versions list/view --output json` and `asc versions update --downloadable true|false`; `--downloadable false` requires `--confirm`). Absent `downloadable` is never defaulted to `true`.
- Live-version lookups query `filter[appStoreState]` and `filter[appVersionState]` in separate requests (Apple combines them with AND) and prefer `appVersionState` (`READY_FOR_DISTRIBUTION`) over `appStoreState` (`READY_FOR_SALE`), while checking `appStoreState` for `PREORDER_READY_FOR_SALE`, `DEVELOPER_REMOVED_FROM_SALE`, and `REMOVED_FROM_SALE`.

## App Store version creation conflicts (`POST /v1/appStoreVersions`)

- Apple returns HTTP 409 `ENTITY_ERROR.RELATIONSHIP.INVALID` ("You cannot create a new version of the App in the current state.") both when an unreleased version already exists for the platform and (with `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`) when the version string is already used.
- On an unhandled 409, `asc versions create` makes a best-effort 5s paginated read of `GET /v1/apps/{id}/appStoreVersions?filter[platform]=...&limit=200` (which has no `sort` parameter, so pages are sorted client-side by `createdDate`) to name existing/unreleased versions in the diagnostic.

## Web-session app status history

- `asc web apps history --app APP_ID [--version-id VERSION_ID]` lists versions via `GET /iris/v1/apps/{id}/appStoreVersions` and serially reads `GET /iris/v1/appStoreVersions/{appStoreVersionId}/appStoreVersionStateChanges` (`appStoreState`, `date`, `initiator`), following `links.next` internally. Requires the `AppStatusHistory` role capability; use `--version-id` or raise `ASC_TIMEOUT` for apps with long histories.

## Web-session review submissions (iris)

- `GET /iris/v1/reviewSubmissions/{id}/items` rejects `include=appStoreVersionExperimentV2` with HTTP 400 `PARAMETER_ERROR.INVALID` (live-verified); `asc web review show` omits it and keeps iris-accepted includes (`inAppPurchaseVersion`, `subscriptionVersion`, `subscriptionGroupVersion`).

## TestFlight Distribution

- `externalBuildState` cannot be patched on build beta details (`--external-testing` was removed in 5.0.0); manage external distribution via `asc builds add-groups ... --submit --confirm` and `asc builds remove-groups ... --confirm`.
- `POST /v1/builds/{id}/relationships/betaGroups` can briefly return a build-specific 404 right after upload; `asc publish testflight` confirms `GET /v1/builds/{id}` (stopping on `FAILED`/`INVALID`) and retries only that propagation 404 with bounded backoff.
- `asc builds add-groups` preflights build state (and external build beta detail when assigning external groups), failing closed on blockers. On HTTP 422, it performs best-effort diagnostic reads of the build and beta detail. `--dry-run` performs the read-only resolution/preflight without mutating or submitting (`--submit` conflicts with `--dry-run`).

## TestFlight text validation (What to Test)

- `betaAppLocalizations.description` and `betaBuildLocalizations.whatsNew` reject `<`, uncomposed generic combining marks, supplementary-plane characters (`U+10000`–`U+10FFFF`, including all emoji), Variation Selectors (`U+FE00`–`U+FE0F`), enclosing/keycap marks (`U+20D0`–`U+20FF`), and symbol/dingbat/box/braille/private-use ranges (`U+2400`–`U+245F`, `U+2500`–`U+259F`, `U+2600`–`U+27EF`, `U+2800`–`U+28FF`, `U+2980`–`U+2BFF`, `U+E000`–`U+F8FF`, `U+FFFD`) with 409 invalid-character errors (`Text for whatsNew contains invalid characters` / `Text contains invalid characters/formats`). Representative live probes across these blocks (including `U+1F600`, `U+10330`, `U+20000`, `U+FE00`/`U+FE0E`/`U+FE0F`, `U+20DD`/`U+20E3`, `U+2400`/`U+2440`, `U+2500`/`U+257F`/`U+2588`/`U+2591`, `U+2600`–`U+26FF`, `U+2701`–`U+27BF`, `U+27C2`, `U+2801`, `U+2980`, `U+2A00`, `U+2B00`–`U+2BFF`, `U+E000`/`U+F8FF`, and `U+FFFD`) were rejected, while sampled code points in neighboring blocks (`Arrows`, `Letterlike Symbols`, `Miscellaneous Technical`, `Enclosed Alphanumerics`, `Geometric Shapes`) and script-specific marks (Devanagari, Thai, Hebrew, Arabic, Cyrillic) were accepted (full probe log in [API_CANARY_LOG.md](API_CANARY_LOG.md)).
- `asc builds test-notes create|update`, `asc builds upload --test-notes`, and `asc publish testflight --test-notes` validate before any HTTP call: they NFC-normalize, strip `<` and uncomposed generic combining marks (`U+0300`–`036F`, `U+1AB0`–`1AFF`, `U+1DC0`–`1DFF`, `U+20D0`–`20FF`, `U+FE20`–`FE2F`) with a stderr notice, and exit 2 on rejected symbol/emoji code points without echoing the text.
- Before writing What to Test, the CLI ensures `betaAppLocalizations` exists for the locale (`filter[app]` + case-insensitive `filter[locale]`), creating it if absent and re-checking on `POST /v1/betaAppLocalizations` HTTP 409 (which can mean either duplicate or invalid locale).

## App Store review details (`appStoreReviewDetails`)

- `notes`: rejected over 4000 characters; `asc review details-create` and `details-update` count Unicode characters of trimmed text with CRLF as 1 (`validation.ReviewNotesLength`) and exit 2 before sending when over 4000.
- Contact fields: `details-create` with default `--if-exists fail` requires non-blank `--contact-first-name` and `--contact-phone` before `POST /v1/appStoreReviewDetails` (`--if-exists skip|update` and `details-update` are exempt).
- `contactPhone`: `+1` numbers written with digits and standard separators must have exactly 10 digits after `+1`; other country codes and numbers without `+` are left to App Store Connect.

## Game Center

- Endpoints resolve Game Center detail via `/v1/apps/{id}/gameCenterDetail` (404 when disabled; `asc game-center details list` has no pagination flags).
- Achievements, leaderboards, and leaderboard sets require creating a release resource to go live; image uploads use reserve → upload → commit.
- `challengesMinimumPlatformVersions` on `gameCenterDetails` is a replace-only `PATCH` relationship (GET is rejected) that requires live `appStoreVersions` linkages (rejects `gameCenterAppVersions` and non-live versions with `ENTITY_ERROR.RELATIONSHIP.INVALID.MIN_CHALLENGES_VERSION_MUST_BE_LIVE`).
- Leaderboard-set member localizations have no direct GET; `view --id` resolves the parent leaderboard and set, then scans the filtered collection. Group challenge relationships are read-only; use `asc game-center challenges create --group-id`.

## Apple Ads Platform API v1

- Platform API v1 (`https://api.ads.apple.com/v1/`) is the only `asc ads` transport. Apple retired Campaign Management API v5 on January 26, 2027, and 6.0.0 removed `asc ads v5`; any `asc ads v5 ...` invocation exits 2 with a removal error that names the v1 replacement from the ledger in `internal/cli/ads/v5_removal.go` (see `docs/release-notes-apple-ads-v5-removal.md`).
- Account-scoped v1 calls send `X-AP-Context: adAccountId=<AD_ACCOUNT_ID>;` resolved from `--ad-account`, `ASC_ADS_AD_ACCOUNT_ID`, profile `ad_account_id`, or root `ads.ad_account_id` when no profile is selected (`--org` / `ASC_ADS_ORG_ID` are never fallbacks).
- `POST /v1/ad-accounts` omits `X-AP-Context`, whereas `GET`/`PUT /v1/ad-accounts/{id}` require `X-AP-Context: adAccountId=<id>;` matching `{id}`.
- `asc ads auth login --network`, `asc ads auth status --validate`, and `asc ads auth discover` call `GET /v1/me` (and `GET /v1/acls` for `discover`) without `X-AP-Context` (`ASC_ADS_ACCESS_TOKEN` skips OAuth token exchange).
- V1 report pagination belongs in the JSON request body (no `--paginate` flag). Negative keywords use a single v1 resource without bulk-delete; product-page countries/devices and custom impression-share report list/view have no one-command v1 replacement. The removed `asc ads v5 reports preset` maps to the `asc ads reports apps` command for its former `--level`; the two ad-group-specific keyword levels map to v1's consolidated `keywords` or `search-terms` report.
- `--org`, `ASC_ADS_ORG_ID`, and profile `org_id` only feed the selected organization that `asc ads auth status` and `asc ads auth discover` report; no request sends an `orgId` context.

## Authentication & Rate Limiting

- App Store Connect JWTs are valid for 10 minutes. `--api-debug` / `ASC_DEBUG=api` logs `X-Rate-Limit` to stderr.
- Retries (`ASC_MAX_RETRIES`, `ASC_BASE_DELAY`, `ASC_MAX_DELAY`, `ASC_RETRY_LOG`; `ASC_MAX_RETRIES=0` disables):
  - GET/HEAD retry transient 408/429/5xx and transport errors.
  - Ordinary POST/PATCH/PUT/DELETE replay only on 429 (ambiguous 408/5xx and transport errors are not replayed); explicitly idempotent mutations retry transient failures.
  - Presigned `uploadOperations` PUT parts retry per part on 408/429/500/502/503/504 and transient transport errors within `ASC_UPLOAD_TIMEOUT`.
  - Unauthenticated public storefront GET commands retry 429/5xx and honor seconds or HTTP-date `Retry-After` up to `ASC_MAX_DELAY`.
- Concurrency: clients cap ordinary mutating requests at 8, with an opt-in bulk ceiling of 16 (`asc.WithBulkMutatingRequestLimit`) used by per-territory availability PATCH fan-out (`PATCH /v1/territoryAvailabilities/{id}`; bulk `POST /v2/appAvailabilities` on already-configured apps is unverified).

## Profiles and certificates list filters

- `GET /v1/profiles` and `GET /v1/certificates` reject `filter[platform]`; only Bundle ID list accepts `filter[platform]` (including `UNIVERSAL`).
- Provisioning profile names generated by `signing fetch` / `signing sync` and explicit `profiles create --name` values are capped at 64 Unicode code points.

## Builds

- `GET /v1/apps/{id}/builds` has no defined order and rejects `sort` with 400 `PARAMETER_ERROR.ILLEGAL`; always query latest builds via `GET /v1/builds?filter[app]={id}&sort=-uploadedDate&limit=1`. Relationship endpoints and top-level collections frequently accept different `sort`/`filter` parameters.
- Build resources and `GET /v1/buildBundles/{id}/buildBundleFileSizes` do not identify `UIDeviceFamily` (iPhone-only builds still list iPad `deviceModel` rows); `asc validate` reads `UIDeviceFamily` from `--ipa` when checking required iPad screenshots.
- Build Upload API reads (`/v1/buildUploads*`, `/v1/buildUploadFiles*`, `/v1/apps/{id}/buildUploads`, `/v1/apps/{id}/relationships/buildUploads`) intermittently return a false 404 `There is no resource of type 'apps' with id '…'`. Top-level reads retry that `type 'apps'` 404 directly; app-scoped reads first verify `GET /v1/apps/{id}` once and retry only when the app exists.

## Xcode Cloud workflows

- **Public `/v1/ciWorkflows` quirks**:
  - `GET /v1/ciWorkflows/{id}` omits `data` linkages on relationships unless `?include=product,repository,xcodeVersion,macOsVersion` is requested (all four are required by `POST /v1/ciWorkflows`), and emits `null` for optional non-nullable create properties (`destination`, `testConfiguration`, `filesAndFoldersRule`) that `workflows duplicate` must strip.
  - TestFlight post-actions (`post_actions[].deployment_config.testflight_deployment_ids`, whose `beta_group_ids` match public `betaGroups` IDs; `beta_tester_ids` equivalence is unverified) and workflow-scoped env vars/secrets exist only on private `/ci/api/` payloads (`asc web xcode-cloud env-vars`).
- **Private `/ci/api` web-session surfaces**:
  - `asc web xcode-cloud scm providers list` (`GET /ci/api/teams/{teamID}/scm-providers-v2`) and `connection-status` (`GET .../scm-providers/{providerID}/connection-v2`): read-only, no pagination; provider contract is browser-verified, CLI live execution unverified.
  - `asc web xcode-cloud settings next-build-number show|set`: `GET` and bodyless `PUT .../products/{productID}/next-build-number?next_build_number=N` (`--confirm`; requires `N` > current, verifies read-back, and redacts any returned TestFlight URL).
  - `asc web xcode-cloud settings version-aliases list|view|create|update|delete`: uses `/ci/api/teams/{publicProviderID}/products/{productID}/configuration-options/version-aliases-v3` (`?limit=100` on list, no `--paginate`; detail `GET`/`PUT`/`DELETE` on `.../{versionAliasID}`). Saves `{name,type,build,locked}` with trimmed `name` (1–40 UTF-16 code units) and `type` in `macos_version|xcode_version`. Mutations require `--confirm`, never retry 408/5xx, and reconcile via read-back (or 404 on delete). Source-captured; live alias mutations remain unverified.

## Devices

- Devices have no DELETE endpoint; enable or disable via `PATCH`. Registration requires a UDID (iOS) or Hardware UUID (macOS). Resetting the device list is portal-only once per membership year, and disabling does not free slots.

## Subscription Offer Codes

- `POST /v1/subscriptionOfferCodes` requires the `prices` relationship for all modes: `FREE_TRIAL` must omit `subscriptionPricePoint` (`--prices "DEU,FRA"`, otherwise 409 `ENTITY_ERROR.RELATIONSHIP.INVALID`), while paid modes require `--prices "DEU:PRICE_POINT_ID"`.

## Monthly Subscriptions with a 12-Month Commitment

- Exposed via `subscriptionPlanAvailabilities` (`/v1/subscriptions/{id}/planAvailabilities`) with `planType=MONTHLY` (upfront availability stays on `subscriptionAvailability`).
- `/v1/subscriptionPricePoints/{id}/adjustedEqualizations` accepts only `filter[planType]=MONTHLY` (rejects `UPFRONT`).
- Unavailable in `USA` and `SGP`; the CLI strips both territories before writing monthly-commitment plan availability.

## Subscription Plan Availability

- `GET /v1/subscriptions/{id}/planAvailabilities?include=availableTerritories` caps `limit[availableTerritories]` at 50; complete territory lists require `GET /v1/subscriptionPlanAvailabilities/{id}/relationships/availableTerritories` (`limit=200`, cursor-paginated).
- `PATCH /v1/subscriptionPlanAvailabilities/{id}` replaces `availableTerritories` wholesale and allows only `availableInNewTerritories` as a mutable attribute (`planType` is create-only, and `availableInNewTerritories` is unsupported for `MONTHLY`). `set` verifies territories via the paginated relationship endpoint and `availableInNewTerritories` via a fresh resource GET.
- Removing an approved subscription from sale (emptying `availableTerritories`) requires the Account Holder web session (`asc web subscriptions availability remove-from-sale` on `/iris/v1`).

## Developer Portal session (web session)

- Bundle IDs, Services IDs, Website Push IDs, App Groups, iCloud containers, and agreements bootstrap CSRF and team context via `POST /services-account/QH65B2/account/listTeams.action`. Same-origin HTTPS redirects are enforced, and cookies/CSRF tokens are never logged.
- `--developer-team` (ID or exact name) is accepted only on Developer Portal commands (`web bundle-ids`, `web service-ids`, `web website-push-ids`, `web app-groups`, `web icloud-containers`, `web agreements`). An explicit selector overrides the cached team; without one, the cached team is reused if available, otherwise the selected App Store Connect provider is matched to a team. A single remaining team is used; multiple unmatched teams require `--developer-team`. There is no team-selection environment-variable fallback.
- `asc web bundle-ids capabilities disable --bundle-id ID --capability PRIVATE_CLOUD_COMPUTE --confirm` requires confirmation and currently supports only `PRIVATE_CLOUD_COMPUTE`. It resolves the Bundle ID through `POST /services-account/v1/bundleIds` (`X-HTTP-Method-Override: GET`), reads `fields[bundleIds]=bundleIdCapabilities` + `include=bundleIdCapabilities,bundleIdCapabilities.capability` (`limit[bundleIdCapabilities]=50`), fails closed on pagination/linkage ambiguity, sends a single `PATCH /services-account/v1/bundleIds/{id}` with `enabled:false` on the target capability, and verifies disabled/removed state via read-back without retry (private proxy acceptance of the field/limit query remains live-unverified).

## Developer Portal Bundle ID reads (web session)

- `asc web bundle-ids list` sends `POST /services-account/v1/bundleIds` (`X-HTTP-Method-Override: GET`) with `teamId` and `urlEncodedQueryParams` (`limit=1000`, `sort=name`, `filter[platform]=IOS,MACOS`). Preserves the JSON:API envelope and warns on stderr if more pages are indicated (no `--paginate`).
- `asc web bundle-ids view --bundle-id ID` sends `POST /services-account/v1/bundleIds/{id}` (`X-HTTP-Method-Override: GET`, body `{teamId}`) with `fields[bundleIds]=name,identifier,platform,seedId,wildcard,~permissions.delete,~permissions.edit` and the full capability/app-group/merchant/cloud-container/certificate/consent include graph.

## Developer Portal Services IDs (web session)

- Public OpenAPI `BundleIdPlatform` omits `SERVICES`; Services IDs use the private `/services-account/v1/bundleIds` proxy (`X-HTTP-Method-Override` for `GET`/`DELETE`, direct `POST`/`PATCH` for writes) and verify `attributes.platform=SERVICES`:
  - `list`: `POST /services-account/v1/bundleIds` (`X-HTTP-Method-Override: GET`, `limit=1000`, `sort=name`, `filter[platform]=SERVICES`, body `{urlEncodedQueryParams, teamId}`).
  - `view --service-id ID`: `POST /services-account/v1/bundleIds/{id}` (`X-HTTP-Method-Override: GET`, body `{teamId}`) with `include=bundleIdCapabilities,bundleIdCapabilities.capability,bundleIdCapabilities.appConsentBundleId`.
  - `create --identifier ID --name NAME --confirm`: `POST /services-account/v1/bundleIds` with `data.type=bundleIds`, `attributes.{identifier,name,platform:"SERVICES",seedId,teamId}`, and empty `bundleIdCapabilities.data` (**must omit root-level `teamId`**, which Apple rejects on non-override `POST`). Verifies via detail read-back.
  - `rename --service-id ID --name NAME --confirm` and `domains set`: preflight and post-write reads enforce `bundleIdCapabilities` completeness (no `next` link, and either exact `paging.total` or the captured `paging.total=0, limit=2147483647` placeholder with resolved references; known navigation-only links like `appGroups`/`bundleId` without `data` are omitted from `PATCH`). `domains set` updates `APPLE_ID_AUTH` (`APPLE_ID_AUTH_WEB_DOMAIN`, `APPLE_ID_AUTH_WEB_RETURN_URL`) on an already enabled Services ID and is live-verified.
  - `delete --service-id ID --confirm`: sends `POST /services-account/v1/bundleIds/{id}` with `X-HTTP-Method-Override: DELETE` and **root-level `teamId`** (omitting it returns HTTP 403 `Please select a team.`), expects 204, and verifies a follow-up detail 404. No Services ID mutation retries automatically.

## Developer Portal Website Push IDs (web session)

- `asc web website-push-ids list` calls legacy `POST /services-account/QH65B2/account/ios/identifiers/listWebsitePushIds.action` (`onlyCountLists=true`, `pageSize=1000`, `pageNumber=1`, `sort=name=asc`, `teamId`, no `sidx`; no `--paginate`) and preserves the root `websitePushIdList` response.
- `view`, `create`, and `delete` use modern `/services-account/v1/websitepushIds` (live-verified):
  - `view`: `POST /services-account/v1/websitepushIds/{id}?include=websitepushIdCapabilities` (`X-HTTP-Method-Override: GET`, body `{teamId}`).
  - `create` (`--confirm`): requires legacy list preflight to be non-full (`pageNumber=1`, rows < `pageSize`) with no matching identifier and an empty capability catalog, sends `POST /services-account/v1/websitepushIds` (`data.type=websitepushIds`, `name`, `identifier`, `teamId`, empty `websitepushIdCapabilities`), and settles via detail/legacy-list read-back without retry.
  - `delete` (`--confirm`): preflights modern detail (`canDelete=true` and proven empty capability relationship), sends `POST /services-account/v1/websitepushIds/{id}` (`X-HTTP-Method-Override: DELETE`, body `{teamId}`), expects 204, and verifies absence in both detail and legacy list without retry. Rename and non-empty capability configuration are unsupported.

## Developer Portal Agreements (web session)

- `asc web agreements status|accept|download` uses `POST /services-account/QH65B2/account/getAgreementHistory` and `POST /services-account/QH65B2/account/acceptAgreements` (HTTP 200 with `resultCode=0` on success). `accept` re-reads `getAgreementHistory` to verify `dateAccepted >= dateEffective`. `download` resolves root-relative `agreementDownloadUrl` paths against the Developer Portal origin (same-origin HTTPS only, non-empty/non-HTML) and redacts the URL from download receipts and errors.

## Transaction Tax reports (web session)

- `asc web finance transaction-tax download` reads `/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/{providerId}/sapVendorNumbers/{sapVendorNumber}?year={YYYY}&month={M}` (requiring `hasVendorTaxReport=true`), derives `regionCurrencyIds` from `reportSummaries[].proceedsByRegion[]`, triggers generation via `GET .../reports?...&isVendorTaxReportReq=true`, polls `/reports/{uuid}/status` for `readyForDownload`, and streams the same-origin HTTPS ZIP under a bounded workflow context into a new `0600` file without retries or leaking sensitive identifiers/URLs.

## Pass Type IDs

- `/v1/passTypeIds/{id}/certificates` rejects `include=passTypeId` and `fields[passTypeIds]` on the live API despite the OpenAPI spec; `asc pass-type-ids certificates list` omits both.

## Sparse Fieldsets Combined with Includes

- `GET /v1/profiles` with `fields[profiles]=name&include=devices` populates `included` even though `devices` is omitted from the primary fieldset; `fields[devices]=...` still sparse-filters included devices. Apple omits per-profile `relationships` unless `devices` is also in `fields[profiles]` (live-verified).
- `GET /v1/certificates` accepts `fields[certificates]=displayName&include=passTypeId`. The probed team had no `PASS_TYPE_ID` certificates, so `included` was absent with and without the sparse fieldset; population of related pass types remains unverified. Non-pass certificates expose `relationships.passTypeId.data=null` only when `passTypeId` is in the fieldset or no certificate fieldset is sent (live-verified; see [the canary record](API_CANARY_LOG.md#sparse-fieldsets-combined-with-includes)).

## App Store Connect API 4.4.1

- Apple added discrete versions for in-app purchases, subscriptions, and subscription groups. Their v2 localizations and images are version-scoped; pass a version ID rather than the legacy product, subscription, or group ID.
- Review submissions accept `inAppPurchaseVersions`, `subscriptionVersions`, and `subscriptionGroupVersions` through `reviewSubmissionItems`. The CLI preserves both relationship data and `included` resources in JSON output.
- API 4.4.1 has no item-detail GET operation (`asc review items view` and `asc review items-get` were removed in 4.0.0); list a parent submission's items with `asc review items list --submission "SUBMISSION_ID"`.
- API 4.4.1 has no marketplace-webhook instance GET operation; `asc marketplace webhooks view` selects the exact ID across all pages of the collection GET.
- Review-item updates accept only nullable `resolved` and `removed` attributes (`--state` was removed in 4.0.0; use `--resolved`, `--removed`, or their matching `--clear-*` flags, with `--confirm` for `removed=true`).
- Review-submission updates expose nullable `platform`, `submitted`, and `canceled` values plus matching `--clear-*` flags (`--confirm` required for `submitted=true` or `canceled=true`; false, null, and platform-only updates do not).
- Canceling a review submission in a non-cancellable state returns HTTP 409 `Resource state is invalid.: Resource is not in cancellable state`. On that error path, `asc review submissions-cancel`, `asc review submissions-update --canceled=true`, and `asc submit cancel` read `GET /v1/reviewSubmissions/{id}` once to explain its state and next step while preserving the conflict exit code.
- The create schema names its second experiment relationship `appStoreVersionExperimentV2`, while its linked resource type remains `appStoreVersionExperiments`; the CLI selector is `appStoreVersionExperimentsV2` (the 3.x `--item-type appStoreVersionExperimentV2` alias was removed in 4.0.0, and experiment treatments are not valid review-item create relationships). Review items require `appCustomProductPageVersions` (`appCustomProductPages` is rejected).
- The product-scoped v1 localization, image, submission, and `sync` command families (`asc iap localizations|images|submit`, `asc subscriptions localizations|images|submit`, `asc subscriptions groups localizations|submit`, `asc subscriptions review submit|submit-group`) were removed in 5.0.0. `asc iap setup` and `asc subscriptions setup` remain supported and warn only when localization flags request legacy v1 localization steps.
- Migration mapping:
  - IAP localizations/images → create or resolve an IAP version, then use `asc iap versions localizations ...` / `asc iap versions images ...`.
  - Subscription localizations/images → create or resolve a subscription version, then use `asc subscriptions versions localizations ...` / `asc subscriptions versions images ...`.
  - Subscription group localizations → create or resolve a group version, then use `asc subscriptions groups versions localizations ...`.
  - IAP / subscription / subscription-group submissions → `asc review items add --submission "SUBMISSION_ID" --item-type <inAppPurchaseVersions|subscriptionVersions|subscriptionGroupVersions> --item-id "<VERSION_ID>"`.
- There is no one-command version-scoped replacement for the removed legacy localization `sync` leaves; reconcile entries through the matching version-localization `list`/`create`/`update`/`delete` commands.
- A legacy IAP image file re-upload has no one-to-one v2 update: create the replacement version image, then delete the old version image if needed. Subscription v2 image updates do not expose the legacy checksum flag; use the version image upload workflow for a new file.
- Exported `internal/asc.Client` methods targeting these legacy resources remain callable during the transition window, with `Deprecated:` comments on legacy submission and IAP localization/image entrypoints naming their version-scoped or review-item replacements.
- Nullable v2 localization updates distinguish omitted, value, and JSON `null`; use the corresponding `--clear-*` flag for explicit clears.

## Developer Portal App Groups (web session)

- `asc web app-groups` combines legacy form endpoints under `/services-account/QH65B2/account/ios/identifiers/` (`listApplicationGroups.action`, `addApplicationGroup.action`, `deleteApplicationGroup.action`; `application/x-www-form-urlencoded` with `teamId` and scoped `csrf`/`csrf_ts` headers, returning `resultCode=0`) with `/services-account/v1/bundleIds`:
  - `delete` (`deleteApplicationGroup.action`, inferred and live-unverified) first scans all team Bundle IDs via proxied `POST /services-account/v1/bundleIds` (`X-HTTP-Method-Override: GET`, `fields[bundleIds]=bundleIdCapabilities`, `limit=200`, `limit[bundleIdCapabilities]=50`, following `links.next`) and refuses if any `APP_GROUPS` capability (even disabled) references the group. After delete, it verifies absence across the strictly paginated App Group list.
  - `assign`, `unassign`, and `set` read the Bundle ID with `fields[bundleIds]=bundleIdCapabilities` and `limit[bundleIdCapabilities]=50`, fail closed on incomplete/ambiguous capability or nested `appGroups` pagination, rewrite only `APP_GROUPS`, and `PATCH /services-account/v1/bundleIds/{id}` (live proxy acceptance remains unverified). `assign` rejects a `group.*` identifier with exit 2 when the team listing proves an opaque resource ID is required. Ambiguous writes settle via read-back without retry and return `DeveloperAppGroupUnverifiedError` when unverified.

## Offline signature verification: Apple leaf certificate types

The offline verifier uses Go's certificate-chain policy, not macOS's platform trust policy. SHA-1-signed leaf or intermediate certificates report `untrusted-chain`, even when `codesign --verify --strict` accepts the binary. The receipt still reports successful CMS and code-page integrity checks when those checks pass. Recognizing an Apple leaf type does not override certificate-chain policy, expiry, or trust anchors.

`asc ipa-info --verify-signature` accepts only code-signing leaves and `asc pkg-info --verify-signature` accepts only installer leaves (`internal/artifacts/verify_leaf.go`). Leaves are matched most-specific-first by Apple OID markers (`1.2.840.113635.100.…`, treated as handled critical extensions) and required EKU; unrecognized or dual-use leaves report `untrusted-chain`:

| Type | Apple markers (`1.2.840.113635.100.…`) | EKU | Artifact | Observed on |
|---|---|---|---|---|
| Apple Development | `6.1.2` + `6.1.12` | codeSigning | IPA / Mach-O | keychain; profiles; Apple docs |
| Apple Distribution | `6.1.4` + `6.1.7` | codeSigning | IPA / Mach-O | keychain; profiles; Apple docs |
| iPhone Developer (legacy) | `6.1.2` | codeSigning | IPA / Mach-O | keychain; profiles |
| iPhone Distribution (legacy) | `6.1.4` | codeSigning | IPA / Mach-O | keychain; profiles; App Store export IPA executable |
| Mac App Distribution | `6.1.7` | codeSigning | IPA / Mach-O | Mac half of Apple Distribution |
| Mac Development | `6.1.12` | codeSigning | IPA / Mach-O | Mac half of Apple Development; Apple docs |
| Developer ID Application | `6.1.13` | codeSigning | IPA / Mach-O | keychain; third-party `/Applications` apps; Apple docs |
| Mac App Store application signing | `6.1.9` | codeSigning | IPA / Mach-O | Mac App Store app in `/Applications`; Apple docs |
| Apple platform signing | `6.22` | codeSigning | IPA / Mach-O | `/bin/ls`, Safari |
| Developer ID Installer | `6.1.14` | `1.2.840.113635.100.4.13` | pkg | notarized third-party pkg |
| Mac Installer Distribution | `6.1.8` | `1.2.840.113635.100.4.9` | pkg | Xcode-exported Mac App Store pkgs |
| Apple package signing (Software Update) | `6.1.29.2` | `1.2.840.113635.100.4.1` | pkg | Xcode's bundled `CoreTypes`, `MobileDevice`, `MobileDeviceDevelopment`, and `XcodeSystemResources` pkgs |

- iOS App Store re-signing and TestFlight (`6.1.25.x`) leaves lack local certificate fixtures and report `untrusted-chain`. CMS-only `x-signature` Mac App Store pkgs report `unsupported`.

## Offline signature verification: xar member payload checksums

- `asc pkg-info --verify-signature` (`internal/artifacts/verify_xar.go`) validates all `<file><data>` heap ranges in the xar table of contents (including component packages) before hashing and verifies each distinct range's `archived-checksum` (`sha1`, `sha256`, `sha384`, `sha512`, `md5`; other algorithms report `unsupported` unless another member is `invalid`).
- Missing checksums on heap data, malformed hex, out-of-heap ranges, or partially overlapping ranges report `invalid`. Extended-attribute (`<ea>`) checksums are intentionally not enforced because `pkgutil --check-signature` accepts modified `<ea>` heap bytes.

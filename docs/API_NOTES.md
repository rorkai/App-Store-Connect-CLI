# API Notes

Quirks and tips for specific App Store Connect API endpoints.

## Web App Privacy Data Usage Updates

- Live canary on 2026-09-04 using a disposable app and a redacted web session verified the private `PATCH /iris/v1/appDataUsages/{usageID}` contract for a same-category, same-purpose `DATA_LINKED_TO_YOU` to `DATA_NOT_LINKED_TO_YOU` identity flip. The request uses the existing `appDataUsages` JSON:API resource and its `dataProtection` relationship; Apple returned `200`, and a fresh GET showed the same usage ID with the new protection.
- A second live canary on 2026-09-05 using the same disposable app verified the reverse `DATA_NOT_LINKED_TO_YOU` to `DATA_LINKED_TO_YOU` transition. The temporary `EMAIL_ADDRESS`/`APP_FUNCTIONALITY` tuple returned `200`, and a fresh GET retained the same usage ID with the new protection. The exact pre-canary `DATA_NOT_COLLECTED` baseline was restored and matched on a fresh GET.
- The canary seeded the tuple with `POST` (`201`), confirmed it by GET, then restored the semantic baseline by deleting it (`204`) and creating the `DATA_NOT_COLLECTED` declaration (`201`). Direct restore attempts via `POST` or `PATCH` to `DATA_NOT_COLLECTED` returned `409 STATE_ERROR`.
- The direct mutations advanced the remote `lastPublished` value while `published` remained `true`, despite no publish endpoint being called. Treat this path as a published-state mutation; do not describe the canary or `asc web privacy apply` as unpublished-only.
- For these verified transitions, the planner must pair only a same-category, same-purpose identity flip in either direction into a PATCH update. Tracking, `DATA_NOT_COLLECTED`, and scope changes remain delete/create operations.

## Apple Ads Profile Context Isolation

- Apple Ads named profiles use only the context stored on that profile: they do not inherit `ads.org_id` or `ads.ad_account_id` from root config or another profile. This prevents a selected profile from silently sending a request to the wrong organization or ad account. Profile-less access-token and environment authentication can still use matching root context.

## Public App Store Ranking

- `asc apps public rank` is an unauthenticated storefront command, not an App Store Connect OpenAPI operation or an Apple Ads metric.
- iOS ranking inspects up to 200 results from the public iTunes `/search` endpoint. Apple TV ranking uses the undocumented MZStore search endpoint with `X-Apple-Store-Front: <numeric-storefront-id>,33`, so it is available only for countries with a known numeric storefront ID.
- `found: false` means only that the app was absent from Apple's returned result window. Storefront order, window size, and the Apple TV response schema can change independently of the CLI.

## Analytics & Sales Reports

- Although Apple's current Sales Reports documentation describes `YYYY-MM-DD` for non-daily dates, the live endpoint requires `YYYY-MM` for monthly reports and `YYYY` for yearly reports. The CLI accepts either form and reduces full monthly or yearly dates to those live period identifiers before the request.
- Vendor number comes from Sales and Trends → Reports URL (`vendorNumber=...`)
- Sales Reports validates the complete report type/subtype/frequency/version tuple against Apple's endpoint table. Although the current table lists `SUBSCRIPTION` `1_3`, live verification in PR #1842 proved `1_4` succeeds and is required by some accounts, so both are accepted and `1_4` remains the default.
- Use `--paginate` with `asc analytics view --processing-date` to search every report page; the CLI forwards the value as `filter[processingDate]` when fetching instances. To resume from a saved report-page `links.next` URL, pass it with `--next <links.next> --paginate`.
- Use `--granularity "DAILY,WEEKLY,MONTHLY"` with `asc analytics view` to filter instances by one or more documented granularities
- Long analytics runs may require raising `ASC_TIMEOUT`

## Finance Reports

Finance reports use Apple fiscal months (`YYYY-MM`), not calendar months.

**API Report Types (mapping to App Store Connect UI):**

| API `--report-type` | UI Option                               | `--region` Code(s)      |
|---------------------|-----------------------------------------|-------------------------|
| `FINANCIAL`         | All Countries or Regions (Single File)  | `ZZ` (consolidated)     |
| `FINANCIAL`         | All Countries or Regions (Multiple Files) | `US`, `EU`, `JP`, etc. |
| `FINANCE_DETAIL`    | All Countries or Regions (Detailed)     | `Z1` (required)         |
| Not available       | Transaction Tax (Single File)           | N/A                     |

**Important:**
- `FINANCE_DETAIL` reports require region code `Z1` (the only valid region for detailed reports)
- Transaction Tax reports are not available through the public App Store Connect API; use the `asc web finance transaction-tax download` web-session workflow or App Store Connect.
- Region codes reference: https://developer.apple.com/help/app-store-connect/reference/financial-report-regions-and-currencies/
- Use `asc finance regions` to see all available region codes

## Tax Categories and Transaction Tax Reports

Verified against the App Store Connect OpenAPI snapshot in `docs/openapi/` and
the App Store Connect web-client source captured for issue #2299:

- The public API still has no tax-category resource or tax-category attribute
  on `apps`, `appInfos`, or `inAppPurchases`.
- App Information tax categories are available through the
  web-session commands `asc web apps tax-category list`,
  `asc web apps tax-category view --app APP_ID`, and
  `asc web apps tax-category set --app APP_ID --category CATEGORY_ID
  [--condition CONDITION_ID ...] --confirm`. The catalog read is
  `GET /iris/v1/taxCategories?filter[productType]=APPLICATION&include=subcategories,conditions&limit[subcategories]=100&limit[conditions]=100`.
  The app read is `GET /iris/v1/appTaxCategories/{appId}?include=category,enabledConditions&limit[enabledConditions]=100`.
- A missing app tax resource is an unconfigured selection; the captured App
  Store Connect UI default is App Store Software. `set` validates category and
  condition IDs against the catalog, sends a complete desired condition set,
  and re-reads the result. Omitting `--condition` sends
  `enabledConditions.data=[]` to clear stale conditions. The command requires
  `--confirm` and does not automatically retry an ambiguous write.
- The request and response shapes are source-backed. A disposable-app canary
  verified explicit application tax configuration and readback. PATCH,
  condition changes, and account-specific errors remain unverified; selecting
  the legally correct classification remains the operator's responsibility.
- In-App Purchase tax categories are available through the web-session commands
  `asc web iap tax-category list`,
  `asc web iap tax-category view --iap IAP_ID`,
  `asc web iap tax-category set --iap IAP_ID --category CATEGORY_ID
  [--condition CONDITION_ID ...] --confirm`, and
  `asc web iap tax-category reset --iap IAP_ID --confirm`. The IAP catalog is
  distinct from the application catalog: it uses
  `GET /iris/v1/taxCategories?filter[productType]=ADDON&include=subcategories,conditions&limit[subcategories]=100&limit[conditions]=100`,
  while App Information uses `filter[productType]=APPLICATION`. `list` keeps
  the raw JSON:API catalog envelope, including `data`, `included`, `links`,
  `meta`, and unknown top-level members, for JSON output.
- `view` first reads
  `GET /iris/v2/inAppPurchases/{iapId}?include=inAppPurchaseTaxCategoryInfo`.
  An explicit `inAppPurchaseTaxCategoryInfo.data: null` means that the IAP
  inherits the parent app's selection; the CLI does not guess the inherited
  category. Inherited JSON output preserves this raw `inAppPurchases` discovery
  envelope. For a configured relationship, the CLI follows the returned
  opaque tax-info ID and reads
  `GET /iris/v1/inAppPurchaseTaxCategoryInfos/{taxInfoId}?include=category,enabledConditions,inAppPurchaseV2&limit[enabledConditions]=100`;
  configured JSON output preserves that raw `inAppPurchaseTaxCategoryInfos`
  detail envelope. The selected IAP is checked against the tax-info owner
  before the result is accepted.
- `set` validates the category and every condition against the ADDON catalog,
  then sends the complete desired category and condition set. Omitting
  `--condition` sends `enabledConditions.data=[]` and clears stale conditions.
  It creates the tax-info resource when the IAP is inherited or patches the
  discovered opaque tax-info ID when configured, performs at most one write and
  one post-read verification, and does not automatically retry an ambiguous
  outcome. If the requested state already matches, it reports a verified
  no-op.
- `reset` deletes only the discovered
  `inAppPurchaseTaxCategoryInfos` override and verifies a fresh explicit-null
  relationship; it never deletes the IAP. When the IAP is already inherited,
  it skips DELETE and reports a verified no-op. These private endpoints and
  their request shapes are absent from the public OpenAPI snapshot.
- A browser canary on 2026-09-06 against disposable app `6759231657` and IAP
  `6760268101` verified ADDON catalog discovery, create, update, explicit
  condition clearing, and delete/readback; the disposable IAP was restored and
  no resources were left behind. This is browser evidence only: final CLI live
  execution remains unverified.
- `GET /v1/financeReports` accepts only `FINANCIAL` and `FINANCE_DETAIL` in
  `filter[reportType]`, and `GET /v1/salesReports` has no tax report type, so
  Transaction Tax reports cannot be generated or downloaded through the public
  API. The captured finance workflow is available through the
  `asc web finance transaction-tax download` command; provider and period
  eligibility remain account-specific.
- `asc capabilities --area monetization` reports App Information and
  In-App Purchase tax-category paths as web-session coverage; Transaction Tax
  reports remain a separate web-session capability.

## Sandbox Testers

- `asc web sandbox create` requires `--first-name`, `--last-name`, `--email`, `--password`, and `--territory`
- Password must include uppercase, lowercase, and a number (8+ chars)
- Historical public v1 create also required password confirmation, a secret question/answer, and a birth date; that removed v1 contract does not establish that those fields are accepted by the current private web flow
- Sandbox territory inputs accept alpha-2, alpha-3, and exact English country names, but the CLI sends canonical 3-letter App Store territory codes (for example, `US`, `USA`, and `United States` all resolve to `USA`)
- This normalization is limited to verified ASC alpha-3 territory surfaces, including customer-review filters; public storefront and finance region flags keep their existing namespaces
- List, view, update, and clear-history use the v2 API through `asc sandbox`
- The current App Store Connect web bundle's New Tester form exposes first name, last name, email, password, confirm password, and country. Its two `POST /sandbox/v2/account/validateFields` requests project the first three fields, then add `acAccountPassword`; `POST /sandbox/v2/account/create` sends exactly `firstName`, `lastName`, `acAccountName`, `acAccountPassword`, and `storeFront`. `confirmPassword` is UI-only, and the bundle contains no `secretQuestion`, `secretAnswer`, or `birthDate` fields. This is a source-backed request shape; it does not prove a successful live create or acceptance of extra portal fields. See issue #2294.
- The same bundle lists testers with `GET /sandbox/v2/provider/account/list?limit=50`; the only live list response captured was the empty `{"totalAccounts":0,"totalInactiveAccounts":0,"accounts":[]}` response. No continuation or pagination contract was captured. Account rows include `id` and `isInFamily`; the UI excludes family members from deletion.
- The captured delete request is `POST /sandbox/v2/account/delete` with a JSON body of `{"ids":[...]}`. `asc web sandbox delete` therefore requires `--confirm`, refuses family members and incomplete list snapshots, sends one delete request, and verifies a fresh list. Delete status/body, post-delete disappearance, and account-wide mutation acceptance remain unverified because no sandbox tester rows were available; do not infer them from the legacy public v1 surface.

## App Store Regulations & Permits declarations

- The public App Store Connect API has no declaration surface. `appInfos` in `docs/openapi/latest.json` exposes no `isRegulatedMedicalDevice`, `isPersonalService`, trader, or DSA attribute, and a case-insensitive scan of the whole snapshot finds no `medical`, `personalService`, `trader`, or `digitalServicesAct` field anywhere. The only trader-adjacent values are read-only `TerritoryAvailability.contentStatuses` reasons (`TRADER_STATUS_NOT_PROVIDED`, `TRADER_STATUS_VERIFICATION_FAILED`, `TRADER_STATUS_VERIFICATION_STATUS_MISSING`), which report a consequence rather than let anything be declared.
- Declarations therefore live on the web-session `ppm/complianceform/v1` service, which is neither JSON:API nor `/ci/api` plain JSON; requests need the App Store Connect UI headers (`X-Csrf-Itc: itc`, `Origin`, and a `/apps/{id}/distribution/info` `Referer`).
- `GET /ppm/complianceform/v1/accounts/{accountId}/requirements?contentId={appId}` lists every declaration Apple tracks for the app. Each row carries `id`, `name`, `ref`, `status`, `dateSigned`, `formId`, and `isRequired`. `requirementData` is keyed by `contentId`; prefer the entry whose `contentId` matches the app and fall back to the entry with an empty `contentId`. `asc web apps declarations list` reads exactly this.
- `GET /ppm/complianceform/v1/accounts/{accountId}/requirements/{requirementId}/forms?contentId={appId}` returns the stored answer alongside `constraints`, an object of JSONPath keys to `{attributeName, options[{value, listValues}]}` validation metadata. The constraint keys are rooted at `$[*]`, so the stored answer is returned as an array; readers accept the answer at the top level, under a `data` object, or as the first element of a `data` array. `asc web apps medical-device view` reads `medicalDeviceData.declaration` (`no`, `yes`, or absent while outstanding) from it.
- `POST /ppm/complianceform/v1/accounts/{accountId}/contents/{appId}/requirements/{requirementId}/forms` saves an answer when the stored `medicalDeviceData.declaration` is absent. The captured body preserves stored answer fields from `form.data` or the documented top-level form shape outside `medicalDeviceData`, then sends `{countriesOrRegions, medicalDeviceData:{...existing,declaration}}`. The new affirmative app-level region picker is limited to `EEA`, `GBR`, and `USA` and defaults to all three; the shipped false path instead derives its regions from the fetched form constraints. `asc web apps medical-device set --declared false` retains its existing no-confirm invocation and sends this only when the stored declaration is not already `no` with the requirement at `COLLECTED`; otherwise it reports `changed: false` without writing. Apple's captured No path preserves any existing regional rows while verifying the app-level No and `COLLECTED` status.
- When a declaration already exists, the captured web bundle uses `PUT` on the same path. For an affirmative answer it preserves matching `registrationInfo` rows for `EEA`, `GBR`, and `USA`, overwrites each row's `countriesOrRegions` and declaration according to the selected subset, and preserves the other form fields. For a No answer it preserves the existing regional rows. The CLI exposes this app-level affirmative path as `asc web apps medical-device set --declared true [--countries-or-regions EEA,GBR,USA] --confirm`; it verifies the stored declaration and persisted regional declarations before reporting success, and skips a matching affirmative answer without writing.
- `asc web apps medical-device region set --app APP_ID --region REGION --input PATH --confirm` sends the captured detailed regional form contract through `PUT /ppm/complianceform/v1/accounts/{accountId}/contents/{appId}/requirements/{requirementId}/forms`. The rootfs-anchored input has `declaration`, explicit localized `supportInfo` rows, and a `registrationNumber` for USA or EEA. The app-level declaration must already be `yes`; the command preserves the complete form, contacts, and opaque regional fields, performs one PUT, then verifies exact readback. Region availability comes only from the captured top-level `$[*].countriesOrRegions` constraint; nested contact or registration selectors cannot authorize or expand it. Legacy forms missing identity metadata use the same account/app/requirement fallbacks as the existing app-level setter; adding those comparison defaults alone does not trigger a write. Readback identity metadata is compared where available, explicit mismatches still fail, and missing optional metadata alone does not invalidate a persisted change. `declaration: false` preserves the existing regional details. This is a web-session surface; the public API has no equivalent.
- Affirmative contact preflight follows the captured UI predicates: a selected contact must carry a legal entity, cover the requested region, and have non-empty phone, email, and address object values. It also fails before PUT when the form constraints materialize a selected-region legal-entity candidate that is absent from the stored contacts; the CLI does not synthesize that UI-managed contact. The command does not embed the disposable form's additional address-subfield constraints or Apple's locale catalog, and it never accepts or prints contact values. Missing or incomplete target-region coverage fails before PUT; arbitrary affirmative legal/contact values remain provider-dependent.
- A sanitized live CLI canary on 2026-09-05 used disposable app `6759231657` and a locally built candidate CLI. The `overall-yes`, `regional-gbr-false`, and `overall-no` CLI steps all exited 0 with exact readbacks; the regional `GBR` No path was exercised while the overall declaration was Yes. One exact raw-form restore PUT returned 200 and its readback matched the saved form. The final baseline, session identity, temporary-file cleanup, and no-uncertain-retry checks all passed. This verifies the current CLI's negative regional path and restoration on that disposable app; it does not prove arbitrary affirmative legal/contact values.
- The personal-service declaration is not implemented. The captured Business frontend identifies `personalServiceDeclaration`, `personalServiceApps`, and `nonPersonalServiceApps` in dynamic DAC7/MRDP/ITA/SERR compliance forms. The current account has no qualifying live requirement/form fixture to verify the applicable form identity, constraints, and write postcondition. `asc web apps declarations list` exposes returned app requirements; source fields alone do not establish an accepted live request.
- EU DSA trader status is account-level rather than app-level: it is read from `GET /ppm/v1/accounts/{id}/sellerInfo` and filed by `POST /ppm/v1/legalEntities/{id}/sellerInfo`, whose body carries contact details, an `isAppTraderOverride` flag, base64 identity documents, and a `jwtToken` minted by a separate `authenticationDetail` call and validated interactively against `id.apple.com`. Every `ppm/v1` record also carries an `optimisticLock`. A legal filing behind an interactive identity check is out of scope for an unattended CLI write.

## Web-session API keys

- `asc web api-keys list` reuses the iris v1 team-key list (`GET /iris/v1/apiKeys?include=createdBy,revokedBy,provider&sort=-isActive,-revokingDate&limit=2000`) and the iris v2 individual-key list (`GET /iris/v2/apiKeys?include=visibleApps,createdByActor,revokedByActor&limit[visibleApps]=3&limit=2000`) already used by `asc web auth capabilities`. Both readers follow `links.next` internally, so the command has no `--paginate` flag. Individual keys sometimes carry an empty `roles` array on that list payload; list does not issue per-key actor lookups. Use `asc web auth capabilities --key-id` to resolve actor-backed roles for one key.
- `asc web api-keys view --key-id` uses the existing iris v1 team-key resource (`GET /iris/v1/apiKeys/{id}?include=provider`). Individual keys appear in `list` but are not loaded by `view`. The issue proposed `get`; current CLI taxonomy uses `view` for this leaf.
- Those payloads expose key ID, nickname, roles, `isActive`, key type, and last-used. They do not include a creation date, so list/view omit that column rather than inventing one. Private key material is never copied into command output.
- The captured App Store Connect individual-keys frontend bundle confirms the actor-filtered v2 list (`GET /iris/v2/apiKeys?include=createdByActor,revokedByActor&filter[createdByActor]=USER:{actorID}`), the empty `POST /iris/v2/apiKeys` create request, browser P-256 key generation with PKCS#8 private and SPKI public export, public-key registration by v2 `PATCH`, and v2 inactive-state revocation. Before the detail/generation flow it checks `GET /iris/v1/apiAccesses` for an `APPROVED` access resource and obtains the actor ID from initialized page state. This current capture contains no `/iris/v1/users/` or `AuthSession.UserEmail` request. The CLI's explicit `GET /iris/v1/users/{userUUID}` and case-insensitive session-email match are additional fail-closed preflight checks for a caller-supplied user ID, not claims about the Apple frontend sequence.
- `asc web api-keys create-individual --user-id USER_UUID --output-dir DIR --confirm` first generates an ECDSA P-256 keypair locally and persists the PKCS#8 private PEM in a random `0600` staging file under the selected output root. It then sends exactly `POST /iris/v2/apiKeys` with `{data:{type:"apiKeys"}}`; the POST body is not used to identify the resource. The command re-reads the same actor-filtered list and requires exactly one newly active key whose ID was absent from the preflight snapshot; a missing or ambiguous result, or a newly returned key that already has a public key, fails closed without a PATCH. After that resolution, the staged file is materialized as `ApiKey_<KEY_ID>.p8` with an atomic no-replace rename, so an existing destination is preserved. The command sends only the SPKI public PEM in exactly one `PATCH /iris/v2/apiKeys/{KEY_ID}` with `{data:{type:"apiKeys",id:KEY_ID,attributes:{publicKey:PUBLIC_PEM}}}` and post-reads the actor-filtered list, requiring the created key id to be active with the generated public key. The flow never uses the team-key `GET /iris/v1/apiKeys/{id}?fields[apiKeys]=privateKey` download path; private key material is generated and retained locally, and is omitted from output and errors. If the POST, list resolution, final rename, PATCH, or post-read state is uncertain, the available local artifact is retained for operator recovery and no automatic remote retry is attempted.
- This individual-key contract is source-backed, test-covered, and consistent with the captured frontend request shapes. No individual key was created or revoked during capture or validation, so provider acceptance of a newly created individual key remains unverified.
- The captured frontend uses `PATCH /iris/{v1|v2}/apiKeys/{id}` with a JSON:API body of `{"data":{"id":"<id>","type":"apiKeys","attributes":{"isActive":false}}}` for revocation. Team uses Iris v1 and individual uses Iris v2; the scoped CLI command is `asc web api-keys revoke --key-id KEY_ID --type team|individual --confirm`. It lists only the requested family before the write, skips an already inactive key, sends one PATCH, then re-lists that family and requires the key to be inactive.
- A sanitized 2026-09-05 live canary created and revoked one newly created temporary team key through an authenticated web session. The pre-existing keys were unchanged, the new key was verified inactive, and temporary files were removed. This verifies the team request path for that account and session; the individual revoke path remains provider-unverified.
- A 5xx/transport failure from the PATCH or any post-list/verification failure reports the revoke outcome as unknown and sends no automatic retry. Only a verified inactive post-state produces the `revoked` receipt; an already inactive pre-state produces an `already_inactive` no-op receipt. Neither receipt contains key material.

## Web app availability (iris)

- `GET /iris/v1/apps/{id}/appAvailabilityV2` returns `availableInNewTerritories` and a links-only `relationships.territoryAvailabilities`. It does not include `availableTerritories.data`. Adding `?include=availableTerritories&limit[availableTerritories]=200` returns 400 `PARAMETER_ERROR.INVALID`.
- When an app has never had availability configured, `GET /v1/apps/{id}/appAvailabilityV2` returns 404 `NOT_FOUND` with detail `There is no resource of type 'appAvailabilities' with id '{appId}'`; an unknown app ID returns the same status with `type 'apps'`. `asc pricing availability view --app` treats only the `appAvailabilities` shape as an expected negative (exit 1, `state_not_ready`) with a remediation hint. `GET /v2/inAppPurchases/{id}/inAppPurchaseAvailability` behaves the same way with `type 'inAppPurchaseAvailabilities'` versus `type 'inAppPurchases'`.
- When an app has never had a price schedule, `GET /v1/apps/{id}/appPriceSchedule` still returns 200 with a synthetic `appPriceSchedules` resource whose ID is the app ID, and `GET /v1/appPriceSchedules/{id}/baseTerritory` returns 200. The first prices read (`GET /v1/appPriceSchedules/{id}/manualPrices`) then returns 404 `NOT_FOUND` with detail `There is no resource of type 'null' with id '{appId}'`. `asc pricing current` treats that shape (and a direct `appPriceSchedules` 404) as an expected negative and prints a `configured: false` receipt.
- When an app has no availability record yet, the same endpoint can return a successful JSON:API envelope with `data: null`; the web client treats that as an expected absent state so `asc web apps availability create` can continue. HTTP permission/authentication failures and malformed envelopes remain errors and are never retried.
- The readable source is the iris v2 related collection: `GET /iris/v2/appAvailabilities/{id}/territoryAvailabilities?include=territory&limit=200`. Follow `links.next`. `filter[available]=true` is rejected with 400 `PARAMETER_ERROR.ILLEGAL`; filter client-side on `attributes.available`.
- `asc web apps delete` uses this collection for the "removed from sale in all territories" preflight. The public API counterpart is `/v2/appAvailabilities/{id}/territoryAvailabilities`.
- `asc web removed-apps restore` uses PATCH `/iris/v1/apps/{id}` with `removed:false`, verifies the app is no longer removed, then POSTs `/iris/v1/userAppPermissions` with `GRANT` (full) or `REVOKE` (limited) for `ALL_SILOABLE_USERS`. Permission writes are skipped when PATCH or verification fails.

## Developer Portal iCloud containers

- `asc web icloud-containers list` reads the modern Developer Portal collection through the cookie-authenticated web session. The logical request is `GET /services-account/v1/cloudContainers?filter[AND][hidden]=false` (or `true` with `--hidden`); Apple's browser transport sends it as `POST` with `X-HTTP-Method-Override: GET`.
- The request body carries the selected `teamId` and `urlEncodedQueryParams=limit=1000&offset=0&sort=name`. The command uses this bounded first collection and has no `--paginate` flag. Apple response envelopes are preserved for JSON output, including `links` and `meta.paging` when present. A warning on stderr identifies an incomplete collection when Apple supplies a continuation link or a paging total larger than the returned rows; the CLI does not invent or follow a next-page contract.
- Resource rows expose Apple's observed `identifier`, `hidden`, `prefix`, `canEdit`, `name`, `canDelete`, and `responseId` attributes along with the opaque resource `id` and `type`. No detail or create/rename/delete contract is assumed from this list response. `asc web icloud-containers create` validates `--identifier`, `--name`, and `--confirm`, then stops. It does not open a session or send a request.

## Web-session Resolution Center

- Resolution Center has no official App Store Connect API surface; the OpenAPI snapshot contains no `resolutionCenter*` or `reviewRejection*` path. Every reader below is a web-session (`/iris/v1`) call and needs Apple ID auth, not an API key.
- Threads have two scopes and they are not interchangeable. `asc web review show` resolves the submission scope (`GET /iris/v1/resolutionCenterThreads?filter[reviewSubmission]={id}&include=reviewSubmission`), which only returns threads Apple attached to that review submission. `asc web review threads --app` reads the app scope (`GET /iris/v1/apps/{appId}/resolutionCenterThreads?include=appStoreVersions,app,appMessageThreadDetail,build,betaBackgroundAssetReviewSubmission&limit[appStoreVersions]=2000&filter[threadType]=REJECTION_BINARY,REJECTION_METADATA,REJECTION_REVIEW_SUBMISSION,APP_MESSAGE_ARC,APP_MESSAGE_ARB,APP_MESSAGE_COMM,APP_MESSAGE_INFORMATIONAL`), which also returns binary, metadata, and informational threads that no submission owns. `show` reports the app-scoped threads the selected submission does not cover under `appThreads`.
- The app-scoped relationship is sent with the review center's captured `filter[threadType]` set rather than a narrowed one. Unsupported include or filter shapes on these surfaces answer 400 (for example `include=fromActor,rejections,resolutionCenterThread` on `resolutionCenterMessages`), so the known-good query shapes are sent verbatim.
- A thread's unsent draft reply lives at `GET /iris/v1/resolutionCenterThreads/{threadId}/resolutionCenterDraftMessage?include=resolutionCenterMessageAttachments,fromActor&limit[resolutionCenterMessageAttachments]=1000`. It is a single-resource document: a thread with no draft answers with a null `data` member, and the relationship can also answer 404. Both mean "no draft" rather than an error. `asc web review threads --drafts` reads it read-only, keeps Apple's raw HTML body, and never returns the attachments' signed download URLs.
- All of these readers follow `links.next` internally, so the commands have no `--paginate` flag.
- The draft client contract uses `POST /iris/v1/resolutionCenterDraftMessages`
  with a `messageBody` and `resolutionCenterThread` relationship, `PATCH` on
  the draft resource with `messageBody`, and `DELETE` on that resource. Sending
  is a separate `POST /iris/v1/resolutionCenterMessages` with a
  `createFromDraftMessage` relationship. These private web-session request
  shapes may change, and they do not prove that Apple will accept a write for
  every account.
- `asc web review reply --thread-id THREAD_ID --message MESSAGE --confirm` is a
  one-shot path: it creates one draft, sends it, and re-reads
  the thread to verify the returned message ID. The receipt omits the body and
  the client never retries an ambiguous send. Attachments are unsupported
  because their write encoding is not implemented, and the command has no CLI
  resume, edit, or delete-draft workflow. Inspect App Store Connect before
  retrying a failed or ambiguous operation.
- The `asc web review drafts create|update|delete` commands are
  the unsent draft CRUD path. `create` requires an app ID, thread ID, exactly
  one `--message` or `--body-file`, and `--confirm`; `update` adds the draft
  ID, while `delete` takes the app, thread, and draft IDs. Each command first
  verifies the thread under the selected app and checks the existing draft so
  create cannot replace one and update/delete cannot target another thread's
  draft. `--body-file` is limited to a regular local file and preserves the
  body byte-for-byte after rejecting blank input. These commands never send or
  upload attachments and never retry an uncertain mutation or post-read
  verification. No disposable live thread/draft fixture was available, so
  Apple provider acceptance of these writes remains unverified.
- A draft read response without `data` is malformed, not proof of absence.
  Create refuses it before writing; delete cannot report verified success
  from it. Only explicit `data: null` or a relationship 404 means no draft.

## Web-session app distribution method

- The public App Store Connect API has no distribution-method surface: `App` and `AppUpdateRequest` in `docs/openapi/latest.json` expose only `contentRightsDeclaration`, `streamlinedPurchasingEnabled`, subscription status URLs, and identity fields, and `AppAvailabilityV2` only carries `availableInNewTerritories`. The setting is web-session only.
- `asc web apps distribution view --app APP_ID` reads the internal app resource (`GET /iris/v1/apps/{id}`) and reports the `distributionType` and `educationDiscountType` attributes verbatim, alongside `name` and `bundleId`. No sparse fieldset or include is requested, because those attributes are returned on the plain resource read and an unknown `fields[apps]` value would fail the request outright.
- Observed values are `APP_STORE` (public App Store distribution) and `CUSTOM` (private distribution through Apple Business Manager or Apple School Manager). Apple omits the attribute for accounts or apps that never carried it; the command reports `unknown` in table output and omits the field in JSON rather than defaulting it to `APP_STORE`.
- `asc web apps distribution set --app APP_ID --method public|private --confirm` implements the captured app-level write. The CLI maps `public` to `APP_STORE` and `private` to `CUSTOM`; public accepts `--education-discount discounted|not-discounted`, while private sends `NOT_APPLICABLE`. When public education is omitted, the setter preserves a current `DISCOUNTED` or `NOT_DISCOUNTED` value returned by the preflight read. A current `DIRECT_URL` or unknown method is rejected before any PATCH.
- The current Apple frontend's pricing action supplies an inner `apps` resource to its request action. The shared Iris transport serializer (`module 60285`, helper `Oe`) converts that resource to `JSON.stringify({data: resource})` before `fetch` (`it`), so the wire request is JSON:API. The inner attributes always contain `educationDiscountType`; `distributionType` is included only when the staged method differs from the saved method:
  `PATCH /iris/v1/apps/{id}` with `{ "data": { "id": "{id}", "type": "apps", "attributes": { "educationDiscountType": "DISCOUNTED|NOT_DISCOUNTED|NOT_APPLICABLE", "distributionType": "APP_STORE|CUSTOM" } } }` when the method changes, or with the `distributionType` member omitted when it does not.
- Distribution reads require resource type `apps` and the selected app ID before using any returned attributes; missing or contradictory identity fails preflight or leaves a post-write result unverified.
- The setter performs a preflight read, skips an already matching pair, sends one PATCH with the paired education attribute and the conditional distribution attribute, and verifies both values with one follow-up read within the command context. It never sends custom organization/user POST or DELETE requests, so existing custom rows are preserved. There are no automatic retries. A PATCH transport failure, HTTP 408, or 5xx is read back once when the context remains usable and returns a non-zero error with `status: "uncertain"`; if the command context expires or is canceled, verification stops without extending the declared timeout and the same uncertain receipt tells the operator to inspect state before retrying. A successful PATCH whose read-back fails or mismatches uses the same receipt status.
- These writes are based on the captured frontend constructor and local HTTP tests; no live Apple mutation or live eligibility/success proof was performed. Apple-side role and eligibility restrictions can still reject a validly serialized request.
- Unlisted App Store distribution is a request form reviewed by Apple, not an attribute value on this resource. There is no captured endpoint for it, so no flag is offered.

## Web-session custom-distribution Apple Account recipients

- The public App Store Connect OpenAPI snapshot has no `customAppUsers` or `customAppOrganizations` resource. The verified user-recipient lifecycle is private web-session JSON:API under `/iris/v1`; organization recipients require a separate capture and are not covered by these commands.
- `asc web apps distribution users list --app APP_ID [--paginate]` reads `GET /iris/v1/apps/{appId}/customAppUsers?limit=100`. JSON preserves Apple's raw JSON:API envelope, including `data`, top-level `links`, `meta.paging`, unknown resource members, unknown attributes, and links. There is no synthetic `pages` wrapper. `--paginate` follows validated `links.next` values, aggregates the data, clears the returned continuation link, and keeps the first envelope's other fields; malformed data, wrong resource types, missing identities, inconsistent totals, duplicate members, cross-app or cross-host links, and loops fail closed. App membership is established by the selected app's collection; a returned user resource does not need a `relationships.app`.
- `asc web apps distribution users create --app APP_ID --recipient-apple-id APPLE_ACCOUNT --confirm` validates the selected app and sends one `POST /iris/v1/customAppUsers` with `{ "data": { "type": "customAppUsers", "attributes": { "appleId": "APPLE_ACCOUNT" }, "relationships": { "app": { "data": { "type": "apps", "id": "APP_ID" } } } } }`. The observed `201` response is a JSON:API resource with `type: "customAppUsers"`, an opaque `id`, `attributes.appleId`, and resource links. The CLI validates the returned type, ID, and account, then verifies that the new member appears in a fresh complete read of the selected app collection. An exact existing account may produce a verified unchanged receipt without a second POST; no case-normalization rule is inferred.
- `asc web apps distribution users delete --app APP_ID --id RECIPIENT_ID --confirm` deletes only a member proven by that app's complete collection read. It sends `DELETE /iris/v1/customAppUsers/{recipientID}` with `{ "data": { "type": "customAppUsers", "id": "RECIPIENT_ID" } }`, accepts the observed `204` success shape, and verifies the ID is absent from a fresh selected-app collection read. A confirmed absent ID may be an unchanged result after a complete read, without sending DELETE.
- List works in any distribution mode. Before either mutation, the CLI validates the selected app's identity and requires `distributionType: CUSTOM`; it never changes distribution implicitly. If the app is `APP_STORE`, set private distribution explicitly with `asc web apps distribution set --app APP_ID --method private --confirm`. The recipient target is `--recipient-apple-id`; `--apple-id` remains the web-session authentication selector.
- Create and delete use at most one write and no automatic retry. Transport failures, HTTP 408/5xx responses, and malformed accepted responses remain uncertain and erroring. A deterministic post-read can establish observed state, but it does not erase the original ambiguous failure or justify an automatic retry; an available created ID remains available for reconciliation. Computed mutation receipts use camelCase fields and explicit state/verification information; uncertain receipts use `changed: null` and `verified: false`, with no session credentials or other authentication secrets.
- A redacted authenticated browser canary on 2026-09-06 exercised the workflow in CUSTOM mode, including `POST /iris/v1/customAppUsers` with HTTP 201, an app-scoped list with HTTP 200, and `DELETE /iris/v1/customAppUsers/{id}` with HTTP 204. The disposable app was restored to its original public distribution and education settings; fresh app, user, and organization reads showed the baseline and zero remaining rows. This is browser contract evidence, not final CLI live verification, and no organization write was performed.
- The live canary does not establish role behavior, duplicate rejection, organization recipient rows, or a second collection page. Registration, invitations, onboarding, rename/PATCH, organization operations, and any generic user/organization union remain unsupported. The private `customAppUsers` and `customAppOrganizations` paths are absent from the public OpenAPI snapshot and may change without notice.

## App transfer status (web session)

- The captured App Information frontend includes `appTransferRequest` and models its `id` and `state`. The narrowed read is `GET /iris/v1/apps/{appId}?include=appTransferRequest`, with no body. This relationship is absent from the embedded public OpenAPI app schema.
- On 2026-09-06, an authenticated read of disposable app `6759231657` returned HTTP 200, the expected `apps` resource ID, `relationships.appTransferRequest.data: null`, and an empty `included` array. The related URL `/iris/v1/apps/{appId}/appTransferRequest` returned HTTP 409 with `ENTITY_ERROR.RELATIONSHIP.INVALID` for that fixture. The CLI uses the successful app read and does not follow the related link.
- `asc web apps transfer status --app APP_ID` preserves the original JSON envelope. Human output distinguishes explicit null (`none`), a resource reference (`present`), and omitted linkage (`unknown`). It matches an included transfer resource by both type and ID, preserves state strings, and leaves missing state unknown. App identity must match before any output. It does not normalize states into eligibility, success, or failure; no pending-transfer response was observed live.
- The legacy navigation URL `/WebObjects/iTunesConnect.woa/wa/LCAppPage/transferApp?adamId={appId}` leads to a prerequisite page. It is not a captured initiate API. No recipient list, initiate, accept, cancel, or decline action contract has been captured. Those operations are deferred to maintainer-run transfers and subsequent request/response captures.

## Last-compatible version settings (`downloadable`)

- App Store Connect's Last-Compatible Version Settings screen has no dedicated resource and no `lastCompatibleVersion` attribute. The feature is carried by the boolean `downloadable` attribute on the existing `appStoreVersions` resource; `lastCompatibleVersion` is only a client-side label App Store Connect puts on the `appStoreVersions` collection it reads back.
- The public API covers both directions. `docs/openapi/latest.json` documents `downloadable` on `AppStoreVersion` and as a nullable attribute on `AppStoreVersionUpdateRequest`. `asc versions list/view --output json` preserves the attribute when Apple returns it, and `asc versions update --downloadable true|false` writes it. The default versions table does not include the field.
- `--downloadable` is tri-state: unset sends no `downloadable` attribute at all, so an unrelated `asc versions update` never changes download availability. `--downloadable false` makes a previously released version unavailable for download on older operating systems and devices, is not reversible from every state, and therefore requires `--confirm`.
- Apple omits `downloadable` on versions that never carried the setting. Reads report the attribute as absent rather than defaulting it to `true`, so a missing key means "Apple did not say", not "downloadable".
- `appStoreState` and `appVersionState` are both returned inconsistently across versions. App Store Connect's web client populates `appStoreState` from `appVersionState` and applies legacy remapping (`READY_FOR_DISTRIBUTION` to `READY_FOR_SALE`) purely client-side. The CLI does not reproduce that remapping.
- A web-session read (`asc web apps last-compatible-version view`) briefly existed for this screen and was retired before it reached a release. It mirrored App Store Connect's own iris request (`GET /iris/v1/apps/{id}?include=appStoreVersions&fields[appStoreVersions]=...,downloadable,...&limit[appStoreVersions]=2000`), which required a web session and offered no write. The public API path supersedes it in both directions, so no web-session command is needed here.

## Web-session app status history

- The public App Store Connect API has no status-history endpoint. fastlane's only status-history path is the retired legacy tunes API (`GET ra/apps/{id}/stateHistory?platform=...`), which has no app-level iris counterpart.
- The modern equivalent is version-scoped: `GET /iris/v1/appStoreVersions/{appStoreVersionId}/appStoreVersionStateChanges`, whose resources carry `appStoreState`, `date`, and `initiator`. `initiator` is the actor App Store Connect shows for the change.
- There is no app-level history endpoint, so `asc web apps history --app APP_ID` lists the app's versions with `GET /iris/v1/apps/{id}/appStoreVersions` and then fans out one state-change request per version. `--version-id` scopes the read to a single version and skips the fan-out after verifying that version's app relationship matches `--app`.
- Both readers follow `links.next` internally, so the command has no `--paginate` flag, matching `asc web api-keys list`.
- The fan-out is serial and shares one request timeout, so an app with a long release history can exceed the 30s default. Scope the read with `--version-id`, or raise `ASC_TIMEOUT`. Requests are not parallelized, to avoid hammering a web session with concurrent internal-API calls.
- `AppStatusHistory` is a role capability in App Store Connect, so accounts without it can get an authorization error on the state-change read even when the app list succeeds.

## Web-session review submissions (iris)

- `GET /iris/v1/reviewSubmissions/{id}/items` rejects `include=appStoreVersionExperimentV2` with HTTP 400 `PARAMETER_ERROR.INVALID` even though the public OpenAPI snapshot lists that relationship. Verified live 2026-09-03. `asc web review show` omits it from the items include and keeps the iris-accepted names, including `inAppPurchaseVersion`, `subscriptionVersion`, and `subscriptionGroupVersion`.

## TestFlight Distribution

- `asc testflight distribution edit --external-testing` shipped in 0.35.3 but App Store Connect does not allow `externalBuildState` in the build beta detail PATCH request. The flag was deprecated (parseable but failing before HTTP) and removed in 5.0.0; it is now an unknown flag.
- External distribution is managed through group assignment: `asc builds add-groups --build-id "BUILD_ID" --group "GROUP_ID" --submit --confirm` to enable, and `asc builds remove-groups --build-id "BUILD_ID" --group "GROUP_ID" --confirm` to remove group assignments.
- App Store Connect can briefly return a build-specific 404 from `POST /v1/builds/{id}/relationships/betaGroups` after an uploaded build is already readable and valid. `asc publish testflight` confirms the uploaded build with `GET /v1/builds/{id}` and retries only that post-upload propagation error with bounded backoff, reporting retry attempts on stderr. A confirmation in processing state `FAILED` or `INVALID` stops immediately without retrying distribution. A later post-upload failure emits a partial publish result with the recoverable `buildId`, terminal processing or notification outcome, and completed stages before exiting non-zero; notification follow-up failures use `failureStage=notification` after beta-group distribution succeeds.
- Before a normal `asc builds add-groups` assignment, the CLI reads the current build state and, when the plan includes an external group, its build beta detail. It stops before the relationship POST for known processing, expired, export-compliance, audience, or beta-review blockers, and fails closed when App Store Connect does not provide enough state to prove readiness. Internal-only assignments do not require the external beta-detail read. `READY_FOR_BETA_SUBMISSION` remains assignment-ready for the documented two-step workflow; `--submit --confirm` keeps the existing post-assignment beta-review submission flow.
- When `POST /v1/builds/{id}/relationships/betaGroups` returns HTTP 422, `asc builds add-groups` preserves Apple's error and exit status, then makes bounded, best-effort reads of the build and, for external groups, its beta detail. The resulting diagnostic reports the current state without claiming which state caused Apple's rejection. Successful assignments and non-422 failures do not make these diagnostic reads.
- `asc builds add-groups --dry-run` is an explicit no-mutation preview: it resolves the build and groups, then may read the build and external beta detail, but never sends the relationship POST or submits beta app review. There is no documented validation-only App Store Connect endpoint, so the state shown by dry-run is observational and advisory rather than proof that a later assignment will succeed. `--submit` cannot be combined with `--dry-run`.

## Game Center

- Most Game Center endpoints require a Game Center detail ID, resolved via `/v1/apps/{id}/gameCenterDetail`.
- If Game Center is not enabled for the app, the detail lookup returns 404.
- Releases are required to make achievements/leaderboards/leaderboard-sets live (create a release after creating the resource).
- Image uploads follow a three-step flow: reserve upload slot → upload file → commit upload (using upload operations).
- The `challengesMinimumPlatformVersions` relationship on `gameCenterDetails` uses `appStoreVersions` linkages (live API rejects `gameCenterAppVersions` for this relationship).
- The relationship endpoint is replace-only (PATCH); GET relationship requests are rejected with "does not allow 'GET_RELATIONSHIP'... Allowed operation is: REPLACE".
- Setting `challengesMinimumPlatformVersions` requires a live App Store version; non-live versions fail with `ENTITY_ERROR.RELATIONSHIP.INVALID.MIN_CHALLENGES_VERSION_MUST_BE_LIVE` ("must be live to be set as a minimum challenges version.").
- App Store Connect has no direct GET for a leaderboard-set member localization. `asc game-center leaderboard-sets member-localizations view --id` resolves the localization's leaderboard and leaderboard set through their to-one endpoints, then finds the exact ID in the doubly filtered collection across all pages.
- App Store Connect exposes a group's challenge relationships as read-only, so there is no `asc game-center groups challenges set` command; create a group-owned challenge with `asc game-center challenges create --group-id` instead.
- `asc game-center details list` is backed by the app's single Game Center detail, so it does not accept `--limit`, `--next`, or `--paginate`.

## Apple Ads Platform API v1

- Platform API v1 uses `https://api.ads.apple.com/v1/` and sends
  `X-AP-Context: adAccountId=<id>;` for ad-account-scoped calls. Its
  `--ad-account` value is independent from the Campaign Management API v5
  `--org` value.
- V1 query and report requests use Platform API JSON schemas and preserve
  Apple's response envelopes. Report pagination belongs in the request body;
  the v1 report commands do not use the legacy `--paginate` flag.
- The v5 command tree remains runnable under `asc ads v5` in CLI 4.4.0 with a deprecation warning.
  Apple retires Campaign Management API v5 on January 26, 2027. The legacy raw
  `asc ads v5 api request` command stays a v5 request and is not rewritten; raw v1
  requests use `asc ads api request`.
- The version-neutral `asc ads auth discover` command calls Platform API v1
  `GET /v1/me` and `GET /v1/acls`. The direct `asc ads me view` and
  `asc ads acls list` commands expose those resources separately.
- Platform v1 has one negative-keywords resource for campaign and ad-group
  scope, and does not provide a bulk-delete operation. Product-page countries,
  product-page devices, and custom impression-share report list/view likewise
  have no one-command v1 replacement in 4.4.0.

## Authentication & Rate Limiting

- JWTs issued for App Store Connect are valid for 10 minutes (handled internally).
- For App Store Connect API requests, GET/HEAD requests automatically retry transient 408/429/5xx responses and transient transport failures. Ordinary POST/PATCH/PUT/DELETE requests are automatically replayed only after App Store Connect rejects them with 429; ambiguous 408/5xx responses and transport failures are surfaced without replay. Explicitly idempotent mutations use the broader transient retry policy because their exact payloads are safe to replay. Mutation bodies are buffered and sent identically on each retry. Set `ASC_MAX_RETRIES=0` to disable retries. Presigned uploads follow the upload-specific rules below. 404 responses are never retried, with one documented exception for Build Upload API reads (see Builds).
- Retry-After headers are honored when they do not exceed `ASC_MAX_DELAY` and fit within the remaining request context. Hints above the cap or beyond the context budget fail fast with the requested delay and the applicable limit. Configure retry settings via `ASC_MAX_RETRIES`, `ASC_BASE_DELAY`, `ASC_MAX_DELAY`, `ASC_RETRY_LOG`.
- Uploads to the presigned URLs Apple returns in `uploadOperations` retry per part rather than per file: a PUT part is retried on 408/429/500/502/503/504 and on transient transport failures, using the same retry settings and honoring Retry-After only up to `ASC_MAX_DELAY`. Over-cap hints fail fast; parts that use any other method are never replayed, and each attempt is bounded by `ASC_UPLOAD_TIMEOUT`. This applies to build, screenshot, Game Center, App Clip, subscription, in-app purchase, and app event asset uploads.
- Unauthenticated public storefront reads used by `asc apps public view`, `asc apps public search`, `asc apps public prices`, `asc apps public descriptions`, `asc apps public rank`, and `asc reviews ratings` are idempotent GET requests. They retry 429 and 5xx responses with the shared backoff settings; Apple sends `Retry-After` as either seconds or an HTTP date on these endpoints, and both forms are capped at `ASC_MAX_DELAY`. `ASC_MAX_RETRIES=0` disables the retries. Successful stdout (including table and JSON renderers) and terminal public-storefront status errors remain unchanged; non-retryable statuses, transport failures, decode failures, and context cancellation are not replayed.
- The public storefront retry path is validated with deterministic `httptest` coverage for status boundaries, Retry-After parsing/capping, response-body draining, request replay, concurrency, and cancellation. It does not perform live mutations. The additive behavior can increase latency and request volume during transient failures, and Apple's undocumented storefront responses remain an external compatibility risk.
- `--api-debug` and `ASC_DEBUG=api` log each response's raw `X-Rate-Limit` value to stderr without changing stdout.
- Some endpoints return 403 when the API key role lacks permission (e.g., finance reports, reviews).

## Builds

- `GET /v1/apps/{id}/builds` has no documented default order and rejects `sort` with 400 `PARAMETER_ERROR.ILLEGAL`; with `limit=1` it can return a weeks-stale build that reads as "latest". Use the top-level collection instead: `GET /v1/builds?filter[app]={id}&sort=-uploadedDate&limit=1`.
- General shape of the trap: a relationship endpoint (`/v1/{parent}/{id}/{children}`) and its top-level collection (`/v1/{children}?filter[{parent}]=`) accept different query parameters, so a `sort` or `filter` that works on one can 400 on the other.
- The Build Upload API intermittently answers 404 `NOT_FOUND` ("There is no resource of type 'apps' with id '…'") for apps that certainly exist, right after app creation and at random; fastlane hit the same flake after moving build-number lookups to `GET /v1/apps/{id}/buildUploads` (https://github.com/fastlane/fastlane/issues/29908). GET/HEAD requests to the top-level `/v1/buildUploads*` and `/v1/buildUploadFiles*` endpoints treat that response as transient only when Apple's detail names resource type `apps`. App-scoped `/v1/apps/{id}/buildUploads` and `/v1/apps/{id}/relationships/buildUploads` reads cannot distinguish the flake from a permanently invalid app ID by response shape, so after the first 404 they make one immediate `GET /v1/apps/{id}` check. A missing app stays terminal without backoff; only a confirmed app makes the original build-upload 404 retryable. Eligible reads reuse the shared retry policy (`ASC_MAX_RETRIES`, `ASC_BASE_DELAY`, `ASC_MAX_DELAY`, `ASC_RETRY_LOG`; 3 retries at roughly 1s/2s/4s by default). A 404 that survives the retry budget is surfaced with Apple's original detail, exit code, and `retry limit exceeded` prefix. A genuinely missing upload or file id (`There is no resource of type 'buildUploads' ...`), writes (`POST`/`PATCH`/`DELETE`) on those paths, 404s with any other error code, and 404s on every other endpoint are never retried and keep their original error shape. This affects `asc builds upload`, `asc builds next-build-number`, `asc builds uploads *`, and the publish flows that poll build uploads.

## Xcode Cloud workflows

- Private SCM reads use `GET /ci/api/teams/{teamID}/scm-providers-v2` (plain array) and `GET /ci/api/teams/{teamID}/scm-providers/{providerID}/connection-v2` (object with nonempty `status` and optional opaque `error`). Both requests have no query or body; the team comes from the selected web session's public provider ID, and the provider ID comes from the private list. `asc web xcode-cloud scm providers list` and `connection-status --scm-provider-id PROVIDER_ID` preserve the complete Apple JSON response and unknown fields. The list has no pagination mode. Human output renders absent connection booleans as unknown and status strings as returned; `connection_issue`, `auth_issue`, and future nonempty values are successful reads, not CLI failures. Registration/linking, repository assignment, disconnect, and product onboarding remain outside this read surface. An authenticated browser on 2026-09-06 returned HTTP 200 for both requests (six providers and one `{"status":"success"}` response) with zero mutations. This establishes the provider contract, not final CLI live execution. These private endpoints are absent from the public OpenAPI snapshot.

- The persistent next build number is available only through the private, web-session-backed CI API. `asc web xcode-cloud settings next-build-number show` sends `GET /ci/api/teams/{publicProviderID}/products/{productID}/next-build-number`; `set --value N --confirm` first reads the current value, requires `N` to be greater, sends a bodyless `PUT` to the same path with `next_build_number=N` as a query parameter, and verifies the value with a fresh GET. The team ID comes from the selected web session and the product is selected with `--product-id`. Apple's response can include a TestFlight URL; the CLI deliberately omits it from output because it may contain sensitive query parameters. This private endpoint is not in the public OpenAPI specification and may change without notice.
- Custom Xcode Cloud version aliases are private web-session resources. `asc web xcode-cloud settings version-aliases list` reads the captured `GET /ci/api/teams/{publicProviderID}/products/{productID}/configuration-options/version-aliases-v3?limit=100` contract. The current UI's list response is an `items` envelope and its item fields include `id`, `name`, `type`, `locked`, `build`, `build_name`, `related_workflow_summaries`, and `build_supported`; the CLI list renderer emits only safe scalar fields and omits nested build and workflow payloads. The detail contract captured from the current UI is `GET /ci/api/teams/{publicProviderID}/products/{productID}/configuration-options/version-aliases-v3/{versionAliasID}` (the client path is relative to its `/ci/api` base) with those fields plus the raw nested values. `asc web xcode-cloud settings version-aliases view` preserves that raw detail object for JSON output and uses the safe scalar fields for table and markdown output. The UI saves with `PUT` to the same detail path and the exact JSON body `{name,type,build,locked}`, using only `macos_version` or `xcode_version`. The create form initializes name and build as empty, but the captured save control is disabled until the trimmed name is nonempty, the raw JavaScript name length is at most 40 UTF-16 code units, and build is nonempty; the CLI enforces those checks, trims the name sent on the wire, and leaves product build-support validation to Apple. Creation uses a new UUID and `locked:false` by default. `asc web xcode-cloud settings version-aliases create`, `update`, and `delete` require `--confirm`; update reads first to preserve omitted fields, requires the effective build to be a nonempty JSON string before PUT, rejects unsupported stored object/number shapes without `--build`, and lets an explicit nonempty `--build` replace them. Mutations compare the requested values exactly after a fresh read; the CLI does not infer an identifier from an object build value. HTTP 408 and 5xx responses to alias writes are uncertain outcomes: create/update re-read the same alias ID and compare the requested fields, while delete requires a follow-up 404. The mutation is never retried; failed reconciliation reports that the write may have succeeded. Other 4xx rejections retain their normal error path. Delete treats a verified detail `404` as the success postcondition. The endpoint accepts a continuation offset, but its continuation response contract has not been captured, so the CLI reports at most 100 aliases and does not expose `--paginate`. The request and response details are source-captured from the current frontend; no alias mutation was performed against Apple's service during implementation. These endpoints are absent from the public OpenAPI specification and may change without notice.
- `GET /v1/ciWorkflows/{id}` returns relationships with links only by default: `repository` and `buildRuns` come back without a `data` linkage, and `product`, `xcodeVersion`, and `macOsVersion` are absent from the response entirely. `POST /v1/ciWorkflows` requires all four linkages, so any read-then-recreate flow must request `?include=product,repository,xcodeVersion,macOsVersion`, which populates them.
- `GET /v1/ciWorkflows/{id}` also emits JSON `null` for optional action and start-condition properties (`destination`, `testConfiguration`, `filesAndFoldersRule`) that `CiWorkflowCreateRequest` does not mark nullable. `workflows duplicate` omits those nulls so the create body stays schema-clean; unused nullable start conditions are omitted rather than sent as `null`.
- `CiAction` has no post-actions: the public workflow schema covers `BUILD`, `ANALYZE`, `TEST`, and `ARCHIVE` actions plus `buildDistributionAudience`, but TestFlight post-actions (beta group and tester assignment) exist only in the private `/ci/api/` workflow payload. A workflow recreated through the public API therefore loses its TestFlight post-actions.
- A live read on 2026-09-04 confirmed that private `workflows-v15` payloads expose TestFlight post-actions under `post_actions[].deployment_config.testflight_deployment_ids`. Two distinct `beta_group_ids` observed across two post-actions matched the same app's authenticated IRIS `betaGroups` IDs and the public command output from `asc testflight groups list --app APP_ID --paginate --output json` (three groups returned). Use `asc xcode-cloud products app --id PRODUCT_ID` to resolve a product to its app, then the public TestFlight group and tester commands when preparing a workflow edit; no duplicate web lookup command is needed. The scan covered 14 products/workflows; all observed `beta_tester_ids` arrays were empty, so tester-ID equivalence remains unverified.
- Workflow-scoped environment variables and secrets are also absent from `CiWorkflowCreateRequest`; they live on the private `/ci/api/` workflow payload. `workflows duplicate` cannot copy them. Use `asc web xcode-cloud env-vars` after creating the copy.

## Devices

- No DELETE endpoint; devices can only be enabled/disabled via PATCH.
- Registration requires a UDID (iOS) or Hardware UUID (macOS).
- Device management UI lives in the Apple Developer portal, not App Store Connect.
- Device reset is limited to once per membership year; disabling does not free slots.

## Subscription Offer Codes

- `POST /v1/subscriptionOfferCodes`: the `prices` relationship is required for every offer mode. For `FREE_TRIAL`, each inline price selects a territory but must omit `subscriptionPricePoint`; including one returns 409 `ENTITY_ERROR.RELATIONSHIP.INVALID`. Use `--prices "DEU,FRA"` for `FREE_TRIAL` and `--prices "DEU:PRICE_POINT_ID"` for paid modes.

## Monthly Subscriptions with a 12-Month Commitment

- Apple announced Monthly Subscriptions with a 12-Month Commitment on April 27, 2026:
  - https://developer.apple.com/news/?id=agq42lxe
  - https://developer.apple.com/help/app-store-connect/manage-subscriptions/set-availability-for-an-auto-renewable-subscription/
- The App Store Connect help docs describe this as a billing option on a regular 1-year subscription, with separate `1 Year Upfront` and `Monthly with 12-Month Commitment` availability sections for the same product.
- App Store Connect API 4.4 exposes `subscriptionPlanAvailabilities` with a `planType` attribute and `/v1/subscriptions/{id}/planAvailabilities` for reading the upfront/monthly plan availability set. Use `planType=MONTHLY` for Monthly with 12-Month Commitment, and keep `subscriptionAvailability` for the default/upfront availability.
- App Store Connect API 4.4.1 adds `/v1/subscriptionPricePoints/{id}/adjustedEqualizations`. Although OpenAPI models `filter[planType]` as an unconstrained string array, the live endpoint rejects `UPFRONT` and reports `MONTHLY` as the only supported value.
- Monthly commitment remains unavailable in the United States and Singapore; the CLI removes `USA` and `SGP` from requested monthly-commitment territories before writing plan availability.

## Subscription Plan Availability

- Reading: `GET /v1/subscriptions/{id}/planAvailabilities` accepts `include=availableTerritories`, but `limit[availableTerritories]` is capped at 50 while a plan can be available in every storefront. The complete set comes from `GET /v1/subscriptionPlanAvailabilities/{id}/relationships/availableTerritories`, whose `limit` maximum is 200 with cursor pagination. `asc subscriptions pricing plan-availability show` prints Apple's include envelope unmodified and warns on stderr when paging metadata shows the include was truncated.
- Writing: `PATCH /v1/subscriptionPlanAvailabilities/{id}` replaces the `availableTerritories` linkage array wholesale, so the request body must carry the complete desired territory set, not a delta. `SubscriptionPlanAvailabilityUpdateRequest` accepts only `availableInNewTerritories` as a mutable attribute; `planType` is create-only through `POST /v1/subscriptionPlanAvailabilities`. After a write, `set` verifies territories through the paginated relationship endpoint and `availableInNewTerritories` through a fresh `GET /v1/subscriptionPlanAvailabilities/{id}` rather than the mutation response.
- Apple's internal web (iris) API uses the same resource, path shape, and PATCH body; `asc web subscriptions availability remove-from-sale` uses it only because emptying `availableTerritories` removes an approved subscription from sale, which Apple restricts to the Account Holder. Everything else about plan availability is available through the public API, so `asc subscriptions pricing plan-availability show|set` uses the public endpoints.
- `availableInNewTerritories` is not supported for `MONTHLY` plan availability.

## Developer Portal session (web session)

- Bundle IDs, Services IDs, Website Push IDs, App Groups, and agreements share one Developer Portal session helper: `POST /services-account/QH65B2/account/listTeams.action` bootstraps CSRF and the team list, then later portal requests carry the selected `teamId` through each endpoint's captured body or session context. Same-origin redirects are enforced; cookies and CSRF tokens are never written to stdout, stderr, or debug logs.
- `--developer-team` (ID, or exact team name) is accepted only on Developer Portal-backed commands (`web bundle-ids list`, `web bundle-ids view`, `web bundle-ids capabilities enable`, `web bundle-ids capabilities disable`, every `web website-push-ids`, `web service-ids` and `web app-groups` subcommand, and `web agreements`). It is not a global web-session flag. There is no `ASC_DEVELOPER_TEAM` env fallback; `--apple-id` / `--provider-id` likewise have none.
- Team resolution: an explicit `--developer-team` wins (case-insensitive ID, then exact name) and fails closed with the available IDs and names if nothing matches. Without a selector, a previously persisted team ID is reused when it is still in the list; otherwise the selected App Store Connect provider is matched by public provider ID, then exact name, then a name-prefix heuristic only when exactly one team matches. A single remaining team is used. Multiple unmatched teams fail closed and ask for `--developer-team`. The resolved team ID is stored in the web session cache next to the provider selection; a new `--developer-team` value overrides and re-persists. `asc web auth status` reports it as additive `developerTeamId`.
- App Groups mutations still refresh CSRF from `listApplicationGroups.action` in that endpoint's scope after the shared bootstrap. Bundle ID capability and App Group assign/set/unassign paths select `bundleIdCapabilities` in `fields[bundleIds]`, request `limit[bundleIdCapabilities]=50`, skip already-satisfied writes, and abort rather than rewrite an incomplete graph. The selected relationship must resolve a `data` collection; `included` alone is not treated as proof of ownership or completeness. A continuation link, inconsistent paging total, or a full 50-item page without an exact total is also rejected. `asc web bundle-ids capabilities disable` sends one exact-resource `PATCH` with the preserved capability payload, then performs one verification read using the original command context. The read must resolve the same complete ownership graph, the same Bundle ID, and every pre-existing unrelated capability resource; success requires either the same target resource IDs with `enabled:false` for every target or complete removal of all target resources by Apple. Conflicting included representations and capabilities outside the resolved relationship are rejected. HTTP 408, 5xx, and transport failures get at most one settling read under the original deadline. A proven disabled state returns the normal receipt; an unverified state returns an error. No PATCH is retried. The public schema supports the field and limit parameters; their acceptance by the private proxy remains live-unverified.

## Developer Portal Bundle ID reads (web session)

- `asc web bundle-ids list` uses the captured cookie-authenticated JSON:API
  proxy `POST /services-account/v1/bundleIds` with
  `X-HTTP-Method-Override: GET`. Its JSON body carries the selected `teamId`
  and `urlEncodedQueryParams`; the first slice sends
  `limit=1000`, `sort=name`, and `filter[platform]=IOS,MACOS`. The response is
  a JSON:API collection with `data`, optional `included`, `links`, and `meta`.
  Bundle ID resources preserve their `type`, opaque `id`, attributes,
  relationships, and resource links. Portal-only attributes observed in the
  captured collection include `identifier`, `dateModified`,
  `entitlementGroupName`, `bundleType`, `platform`, `wildcard`, `dateCreated`,
  `bundleIdCapabilitiesSettingOption`, `seedId`, `name`, `platformName`,
  `deploymentDataNotice`, and `responseId`.
- `asc web bundle-ids view --bundle-id ID` uses the captured detail form
  `POST /services-account/v1/bundleIds/{id}` with the fields/include query in
  the URL, `X-HTTP-Method-Override: GET`, and a JSON body containing only the
  selected `teamId`.
  The requested `fields[bundleIds]` are `name,identifier,platform,seedId,wildcard,~permissions.delete,~permissions.edit`.
  The requested include graph covers `bundleIdCapabilities`, its
  `capability`, `associatedBundleIds`, `appGroups`, `merchantIds`,
  `cloudContainers`, `certificates`, `appConsentBundleId`, `macBundleId`,
  `relatedAppConsentBundleIds`, `parentBundleId`, and
  `mediaSharingProtocolIds`. The response is a single JSON:API `data` resource
  plus any included capability resources. Table/Markdown output shows the
  primary resource fields only and emits a diagnostic when included resources
  are present; use `--output json` to inspect the complete capability graph.
- Both read commands bootstrap the shared Developer Portal team session, carry
  no credentials or CSRF values in output, and do not mutate Bundle IDs or
  invalidate provisioning profiles. This first slice intentionally does not
  follow `links.next` or claim pagination; a returned continuation remains
  available in JSON for a later resource-family slice, and table/Markdown
  output emits the standard more-pages warning when it is present.

## Developer Portal Services IDs (web session)

- `asc web service-ids list` uses the private logical `GET
  /services-account/v1/bundleIds` contract with `limit=1000`, `sort=name`, and
  `filter[platform]=SERVICES`. As with the captured native Bundle ID
  collection, the cookie-authenticated transport sends an actual `POST` to
  `/services-account/v1/bundleIds` with
  `X-HTTP-Method-Override: GET` and a JSON body containing
  `urlEncodedQueryParams` and `teamId`; a live list capture returned HTTP 200.
  The command validates that every returned resource is a `bundleIds` resource
  whose platform is exactly `SERVICES` and preserves the original JSON:API
  envelope for JSON output.
- `asc web service-ids view --service-id ID` uses the private
  logical `GET /services-account/v1/bundleIds/{id}` detail contract with
  `include=bundleIdCapabilities,bundleIdCapabilities.capability,bundleIdCapabilities.appConsentBundleId`.
  The actual transport is `POST` plus `X-HTTP-Method-Override: GET`, with a
  JSON body containing only the selected `teamId`. The returned resource ID and
  `attributes.platform=SERVICES` are checked before a mutation can use it.
- `asc web service-ids create --identifier IDENTIFIER --name
  NAME --confirm` uses logical `POST /services-account/v1/bundleIds` with a
  JSON:API `data.type=bundleIds` resource whose attributes are
  `identifier`, `name`, `platform=SERVICES`, `seedId`, and `teamId`, plus an
  empty `bundleIdCapabilities.data` array. The create request must omit a
  root-level `teamId`: the captured frontend helper removes its injected team
  field when no method override is used, and Apple's live endpoint rejects that
  root member as an invalid JSON:API document. The selected team remains in
  `data.attributes.teamId`. The CLI does not invent capability, Sign in with
  Apple domain, or app-consent settings; it reads the created resource back
  before returning a receipt.
- `asc web service-ids rename --service-id ID --name NAME
  --confirm` reads and validates the current Services ID, then sends logical
  `PATCH /services-account/v1/bundleIds/{id}`. The PATCH preserves the complete
  current relationship map, including `bundleIdCapabilities`, and changes only
  the name plus the private team attribute required by the endpoint. A
  post-write detail read must match the requested name and identifier.
- `asc web service-ids delete --service-id ID --confirm` sends
  logical `DELETE /services-account/v1/bundleIds/{id}` as the captured actual
  `POST` plus `X-HTTP-Method-Override: DELETE` and a JSON body containing the
  selected root-level `teamId`. The captured frontend helper retains its
  injected team field for method-override requests; Apple's live endpoint
  returns HTTP 204 when it is present and HTTP 403 (`Please select a team.`)
  when it is omitted. The preflight rejects native iOS/Mac or otherwise
  non-`SERVICES` resources. The command reports success only after the detail
  read returns HTTP 404. A 408 or 5xx response, transport failure, malformed success body, or
  failed post-read is an unverified outcome; no Services ID mutation is
  retried automatically.
- Services ID lifecycle support is private-only because the public OpenAPI
  `BundleIdPlatform` enum does not include `SERVICES`. Capability graph
  mutation and Sign in with Apple domain configuration remain uncaptured.
  Website Push ID lifecycle and iCloud container reads use the separate
  captured workflows documented below.

## Developer Portal Website Push IDs (web session)

- `asc web website-push-ids list` reads the current Developer
  Portal identifier page at `https://developer.apple.com/account/resources/identifiers/list`.
  Its captured request is `POST /services-account/QH65B2/account/ios/identifiers/listWebsitePushIds.action`
  with an `application/x-www-form-urlencoded` body containing
  `onlyCountLists=true`, `pageSize=1000`, `pageNumber=1`, `sort=name=asc`, and
  the selected `teamId`. The request has no `sidx` field. The shared web
  session helper supplies the team and CSRF context.
- The live read returned HTTP 200 with `resultCode=0`, `pageNumber=1`,
  `pageSize=1000`, and an empty root-level `websitePushIdList` in the captured
  account state. JSON output preserves Apple's complete root response,
  including unknown provider fields; the list is not nested under `data`.
  Table and Markdown output use only a small scalar projection of each entry.
- The captured frontend (`captures/apple-developer-main.js`) maps each legacy
  item to `id=item.websitePushId` for this resource (offset 849,700), while its generic identifier table
  reads `name` and `identifier` (offset 866,718); the topic picker also reads
  `websitePushId`/`id` and `identifier` (offsets 1,806,200–1,808,480).
  These source-backed fields justify the formatted projection, but the live
  empty collection means the remaining row schema stays open-ended.
- Pagination is not exposed because no continuation contract was captured for
  this legacy response. The command reads only the first fixed page and does
  not claim a complete account-wide collection.
- Website Push mutation preflight and legacy-list verification require page number 1 and a positive returned page size. A collection filling that returned page is incomplete for mutation purposes, even if Apple reduces the requested page size; no continuation contract is assumed.
- `asc web website-push-ids view` reads one modern
  `websitepushIds` resource with a physical `POST
  /services-account/v1/websitepushIds/{id}?include=websitepushIdCapabilities`,
  `X-HTTP-Method-Override: GET`, and a JSON body containing only the selected
  `teamId`. JSON output preserves Apple's complete JSON:API envelope.
- `asc web website-push-ids create` sends the captured modern
  JSON:API `POST /services-account/v1/websitepushIds` body with
  `data.type=websitepushIds`, `name`, `identifier`, `teamId`, and an explicitly
  empty `websitepushIdCapabilities` relationship. The CLI requires the legacy
  list preflight to show no matching identifier and the capability catalog to
  be empty. It expects HTTP 201 and verifies the created resource by detail
  read; an empty or malformed create response is settled from the legacy list
  without retrying the POST.
- `asc web website-push-ids delete` preflights the modern detail
  resource and requires `canDelete=true` plus an explicitly empty capability
  relationship. It sends physical `POST
  /services-account/v1/websitepushIds/{id}` with
  `X-HTTP-Method-Override: DELETE` and a body containing only `teamId`, expects
  HTTP 204, then verifies canonical detail absence and legacy-list absence.
  Unknown outcomes are reported without an automatic retry.
  Both mutation commands retain the selected team on an unverified write,
  and the error always explains that the write may have succeeded and must
  be inspected before retrying.
- Rename remains unavailable: the captured Website Push UI exposes no rename
  or Save control. Capability configuration and non-empty capability graphs
  remain fail-closed because no writable capability contract is exposed by
  this slice. The live disposable canary used for this contract was deleted
  and the final legacy list returned empty.
- A final built-CLI canary using binary SHA-256
  `fd940938b7477aa27b0f5c793618af0c5c1e30d050f85b6bacf1738378ce31f9`
  exercised the complete happy path for one uniquely generated owned
  disposable identifier: baseline `list`, `create`, `view`, `delete`, and
  final `list` all exited successfully. The create and delete receipts and
  detail readback matched the canary identity, the capability relationship and
  included graph were empty, the baseline list remained unchanged, and final
  absence plus temporary-evidence cleanup were verified. The CLI view exposed
  no certificate fields; `certificateAbsenceIndependentlyVerified` is false
  because this command slice has no certificate endpoint, so the canary does
  not claim independent certificate absence.

- Empty capability data is accepted only when available paging metadata also proves emptiness: a non-empty next link, malformed total, or nonzero total blocks the operation. This applies to the create capability catalog and the detail relationship checked for create verification and delete preflight.
- HTTP 408 is an ambiguous Website Push write outcome, like a 5xx response. Create and delete use read-only settlement without replaying the mutation; an unproven result retains the inspection-before-retry warning.

## Developer Portal Agreements (web session)

- The public API has no agreements endpoint. `asc web agreements` uses the cookie-authenticated Developer Portal account services: `POST /services-account/QH65B2/account/getAgreementHistory` and `POST /services-account/QH65B2/account/acceptAgreements`, both with a JSON body carrying `teamId` (accept also carries an `agreementIds` array, so several agreements can be accepted in one request). They answer HTTP 200 even on failure; `resultCode` carries the outcome (`0` success).
- Each history record includes `agreementDownloadUrl`, observed as a root-relative Developer Portal path such as `/services-account/agreement/{agreementId}/content/pdf`. `asc web agreements download` resolves it against the Developer Portal origin and only follows HTTPS, same-origin targets and redirects, and rejects empty or HTML responses. The URL is treated as potentially signed: the `download` receipt and its error text never include it. `asc web agreements status` and the verified `accept` receipt still expose it as `downloadUrl` in JSON, so treat those outputs accordingly.
- `acceptAgreements` returns the updated history, but the CLI re-reads `getAgreementHistory` after the write and reports the re-read state (`dateAccepted >= dateEffective`) instead of trusting the mutation response.

## Transaction Tax reports (web session)

- The public App Store Connect API and OpenAPI snapshot do not expose Transaction Tax report generation. The `asc web finance transaction-tax download` command uses the authenticated finance web session instead.
- The captured finance page reads the selected month from `/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/{providerId}/sapVendorNumbers/{sapVendorNumber}?year={YYYY}&month={M}` and requires `hasVendorTaxReport=true` before generation. It derives the complete vendor-tax region-currency list from the transformed `reportSummaries[].proceedsByRegion[]` model, then issues exactly one GET to `/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/{providerId}/sapVendorNumbers/{sapVendorNumber}/reports?year={YYYY}&month={M}&regionCurrencyIds={ids}&reportTypes=&isVendorTaxReportReq=true`.
- The generated UUID is retained only in memory for bounded GET polling at `/reports/{uuid}/status`; readiness requires the captured literal `readyForDownload` status and a download URL. The command fetches the ready artifact once, accepts only an HTTPS same-origin URL and redirects, checks the ZIP signature, and publishes a complete `0600` file without replacing an existing destination.
- Receipts and errors omit generated job IDs, signed URLs, provider/vendor identifiers, and finance values. Generation, polling, and download failures do not trigger an automatic regeneration or retry. No report-history or caller-supplied report-ID contract was observed.

## Pass Type IDs

- Live API rejects `include=passTypeId` and `fields[passTypeIds]` on `/v1/passTypeIds/{id}/certificates` despite the OpenAPI spec allowing them.
- The CLI does not expose those parameters for `pass-type-ids certificates list` to avoid API errors.

## Sparse Fieldsets Combined with Includes

Observed 2026-09-02 against a live App Store Connect team. The CLI does not add included relationship names to the primary fieldset for these list commands.

- `GET /v1/profiles` with `fields[profiles]=name&include=devices` returns HTTP 200 and still puts related devices in `included`. Apple omits `relationships` on each profile unless `devices` is also listed in `fields[profiles]`. `fields[devices]=name,udid` still sparse-filters those included devices.
- `GET /v1/certificates` with `fields[certificates]=displayName&include=passTypeId` returns HTTP 200. This team has no `PASS_TYPE_ID` certificates, so `included` was absent both with that sparse fieldset and with `include=passTypeId` alone. Non-pass certificates expose `relationships.passTypeId.data=null` only when the relationship is in the fieldset (or when no certificate fieldset is sent).

## App Store Connect API 4.4.1

- Apple added discrete versions for in-app purchases, subscriptions, and subscription groups. Their v2 localizations and images are version-scoped; pass a version ID rather than the legacy product, subscription, or group ID.
- Review submissions accept `inAppPurchaseVersions`, `subscriptionVersions`, and `subscriptionGroupVersions` through `reviewSubmissionItems`. The CLI preserves both relationship data and `included` resources in JSON output.
- API 4.4.1 has no item-detail GET operation. List a parent submission's items with `asc review items list --submission "SUBMISSION_ID"`.
- API 4.4.1 has no marketplace-webhook instance GET operation. `asc marketplace webhooks view` preserves its released behavior by selecting the exact ID across all pages of the supported collection GET.
- Review-item updates accept only nullable `resolved` and `removed` attributes. The response-only `state` attribute cannot be patched; use `--resolved`, `--removed`, or their matching `--clear-*` flags. Setting `removed=true` requires `--confirm`.
- Review-submission updates expose nullable `platform`, `submitted`, and `canceled` values plus matching `--clear-*` flags. Setting `submitted=true` or `canceled=true` requires `--confirm`; false, null, and platform-only updates do not.
- The create schema names its second experiment relationship `appStoreVersionExperimentV2`, but its linked resource type remains `appStoreVersionExperiments`. The CLI selector is `appStoreVersionExperimentsV2`. Experiment treatments are not valid review-item create relationships.
- Review items require `appCustomProductPageVersions`; `appCustomProductPages` is not an accepted item type because a page ID cannot be silently converted to a version ID.
- The v1 localization/image commands and submission shortcuts remain available during their deprecation window. Each direct invocation warns on stderr and preserves the existing endpoint, flags, stdout, and exit behavior. No v1 localization or image command is removed in this release.
- 4.0.0 removes the review item-detail surfaces: `asc review items view` and `asc review items-get` → `asc review items list --submission "SUBMISSION_ID"`; `asc review items update --state` / `items-update --state` → `--resolved` or `--removed`.
- The 3.x `--item-type appStoreVersionExperimentV2` alias is removed in 4.0.0; the value is rejected with guidance naming the canonical `appStoreVersionExperimentsV2`.
- `asc iap setup` and `asc subscriptions setup` remain supported, but warn when localization flags request their legacy v1 localization steps. Setup calls without those flags do not warn.
- Migration mapping:
  - IAP localizations/images → create or resolve an IAP version, then use `asc iap versions localizations ...` / `asc iap versions images ...`.
  - Subscription localizations/images → create or resolve a subscription version, then use `asc subscriptions versions localizations ...` / `asc subscriptions versions images ...`.
  - Subscription group localizations → create or resolve a group version, then use `asc subscriptions groups versions localizations ...`.
  - IAP submissions → `asc review items add --submission "SUBMISSION_ID" --item-type inAppPurchaseVersions --item-id "IAP_VERSION_ID"`.
  - Subscription submissions → `asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"`.
  - Subscription group submissions → `asc review items add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions --item-id "GROUP_VERSION_ID"`.
- There is no one-command version-scoped replacement for the two legacy localization `sync` leaves. Reconcile entries through the matching version-localization list/create/update/delete commands.
- A legacy IAP image file re-upload has no one-to-one v2 update. Create the replacement version image, then delete the old version image if needed. Subscription v2 image updates do not expose the legacy checksum flag; use the version image upload workflow for a new file.
- The 33 exported `internal/asc.Client` methods that target these legacy resources remain callable and are marked with Go `Deprecated:` documentation naming their version-scoped or review-item replacement.
- Nullable v2 localization updates distinguish omitted, value, and JSON `null`; use the corresponding `--clear-*` flag for explicit clears.

## Developer Portal App Groups (web session)

- App Group resources and their Bundle ID associations are not exposed by the public App Store Connect API. `asc web app-groups` uses the Developer Portal legacy form endpoints under `/services-account/QH65B2/account/ios/identifiers/` (`listApplicationGroups.action`, `addApplicationGroup.action`, `deleteApplicationGroup.action`) plus the cookie-authenticated JSON:API proxy under `/services-account/v1`.
- Legacy form endpoints are `POST` with `application/x-www-form-urlencoded` bodies that always carry `teamId`, return a `resultCode` envelope (`0` is success; `userString`/`resultString` carry Apple's message), and require the `csrf`/`csrf_ts` headers captured from a preceding `listApplicationGroups.action` response in the same endpoint scope.
- `deleteApplicationGroup.action` takes `teamId` and `applicationGroup` (the opaque App Group resource ID shown in `asc web app-groups list`) and returns the bare `resultCode` envelope. This contract is inferred from the sibling list/create actions and the long-standing third-party portal client contract; it was not live-verified when added because no Developer Portal web session was available. The CLI verifies every delete by re-reading the team's App Group list with pagination and fails if the group is still listed; on that strict read a success envelope whose `applicationGroupList` or `totalRecords` is absent or null, whose `totalRecords` is smaller than the records returned, whose `pageNumber` does not match the requested page, whose pages stop before `totalRecords` is reached, or that repeats an App Group across pages is treated as unverified rather than as a short list. A 2xx delete response whose body cannot be parsed or that carries no `resultCode` is likewise reported as an accepted but unverified delete, because the group may already be gone; an explicit non-zero `resultCode` is a refused delete. `addApplicationGroup.action` follows the same rule: a 2xx create whose body cannot be parsed, that carries no `resultCode`, or whose `applicationGroup` receipt is incomplete is reported as an accepted but unverified create (the group may already be registered, so a blind retry is unsafe), while an explicit non-zero `resultCode` stays a refusal.
- Bundle ID association submits the complete group set: the CLI reads the Bundle ID with `bundleIdCapabilities` in both `fields[bundleIds]` and `include`, requests Apple's documented maximum `limit[bundleIdCapabilities]=50`, rewrites only the `APP_GROUPS` capability's `appGroups` relationship together with its `enabled` attribute, and `PATCH`es `/services-account/v1/bundleIds/{id}`. `assign`, `unassign`, and `set` share this path with different computed sets. They require the selected `bundleIdCapabilities` relationship to contain a resolved `data` collection; an absent or links-only relationship is not reconstructed from `included`, because included resources have no independent completeness boundary. They also reject a capability relationship with a non-empty `links.next`, a paging total that differs from the returned linkage count, or 50 returned linkages without an exact total. The nested `APP_GROUPS.appGroups` relationship is rejected when it advertises another page or a mismatched total. Other fail-closed checks cover a different Bundle ID than requested, malformed or null relationships, invalid or missing linkage identities, conflicting included representations, incomplete included capability resources, duplicate `APP_GROUPS` capabilities, and a missing or non-boolean `enabled` value. The public schema supports the selected relationship field and limit; the private proxy behavior still needs live read verification before claiming provider acceptance.
- `assign` refuses a `--group` value beginning with `group.` when a complete App Group listing for the team proves it is an identifier rather than an opaque resource ID. The match is exact and case-sensitive. The failure exits 2 with the resource ID to use; an unreadable listing leaves the assignment to proceed so the portal can return its own verdict. Every App Group failure carries a structured diagnostic code (`invalid_input`, `dependency_failed`, `state_not_ready`, `resource_conflict`, `authentication_rejected`, `resource_not_found`, `request_failed`, or an explicit `internal_error`). Real Developer Portal 401/403 responses retain their HTTP status and classify as authentication failures. Malformed team, App Group, and Bundle ID responses classify as dependency failures, and legacy portal messages and request IDs are sanitized before terminal output.
- When the portal accepts a write but the verification read fails or disagrees, the client returns a `DeveloperAppGroupUnverifiedError` and the CLI warns that the change should be assumed applied. A write that fails without a verdict (transport error, dropped connection, or context deadline after the request was handed off) is settled by the same re-read: the requested state means it applied, the prior state means it did not and the original transport error is returned, and anything else is reported as unverified. Explicit HTTP error statuses are refusals and are never re-read. `delete` settles its legacy form POST the same way against the App Group list. A disabled `APP_GROUPS` capability can still list groups; those raw relationships count as references, so `unassign` and `set` do not discard them merely because the capability is disabled.
- Before deleting, the CLI lists every Bundle ID in the team through the proxied `POST /services-account/v1/bundleIds` read (`X-HTTP-Method-Override: GET`) with `bundleIdCapabilities` selected in `fields[bundleIds]`, the nested capability and App Group includes, `limit=200`, and `limit[bundleIdCapabilities]=50`, following validated `links.next` cursors. It refuses when any `APP_GROUPS` capability references the group. A sparse or malformed capability relationship, a missing included capability, a non-Bundle-ID entry, a repeated Bundle ID, inconsistent pagination, conflicting capability representations, an unreadable nested App Group relationship, or an absent/null `data` collection is treated as an error, never as unassigned. This collection read is inferred from the single-resource contract and remains unverified against a live private proxy.

## Apple Ads Platform API v1 in 4.4.0

- Release 4.4.0 makes Platform API v1 the direct `asc ads` resource surface. Its host, request payloads, response envelopes, pagination, and ad-account context differ from Campaign Management API v5. The intermediate nested prototype is intentionally removed before release.
- Apple scheduled Campaign Management API v5 retirement for January 26, 2027. Every runnable v5 leaf moves under `asc ads v5`, emits a deprecation warning on invocation, and keeps its existing endpoint and output behavior. Use the direct v1 replacement where one exists; the seven v5 leaves without a one-command replacement retain explicit migration guidance.
- Platform account-scoped requests use `X-AP-Context: adAccountId=<AD_ACCOUNT_ID>;`. Resolve the account independently from the legacy organization context with `--ad-account`, `ASC_ADS_AD_ACCOUNT_ID`, the selected profile's `ad_account_id`, or root `ads.ad_account_id` when no named profile is selected. `ASC_ADS_ORG_ID` and `--org` are not fallbacks for an ad-account ID.
- `/v1/ad-accounts` is method-dependent: `POST /v1/ad-accounts` creates an account without `X-AP-Context`; `GET /v1/ad-accounts/{id}` and `PUT /v1/ad-accounts/{id}` require `X-AP-Context: adAccountId=<id>;`, and the header account must match the path ID.
- Authentication validation and discovery use Platform API v1. `asc ads auth login --network` and `asc ads auth status --validate` exchange an OAuth client-credentials token when needed and call `GET /v1/me` without an ad-account context. `asc ads auth discover` calls Platform API v1 `GET /v1/me` and `GET /v1/acls` without an ad-account context. A supplied `ASC_ADS_ACCESS_TOKEN` skips token exchange.
- The deprecated `asc ads v5 reports preset` warning follows `--level`: campaigns, ad groups, ads, keywords, and search terms point to their matching `asc ads reports apps` command; the two ad-group-specific keyword levels point to v1's consolidated `keywords` or `search-terms` report.

Transaction Tax archive streaming uses a bounded workflow context instead of inheriting the web client's shorter per-request timeout. Lower-level callers without a context deadline retain the configured client timeout. Caller cancellation still bounds the download, and the shared client timeout remains unchanged.

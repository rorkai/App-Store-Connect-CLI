# Self-link URLs as resource IDs

## Scope

Every `asc` JSON read prints Apple's envelope unmodified, so each resource
carries a `links.self` URL such as
`https://api.appstoreconnect.apple.com/v1/builds/2f1a3c4d-...`. Webhook
deliveries (`asc webhooks`) and other tools (for example EAS CLI's
`eas testflight:feedback`) hand the same URLs around. Until now every ID flag
accepted bare IDs only, so callers had to strip the URL by hand before piping a
`links.self` value back into `asc`.

This change lets the highest-traffic ID flags accept either a bare ID or the
resource's API self-link. It adds no command, no flag, and no output change.

## Contract

`shared.ResourceIDFromValue(value, resourceType string) (string, error)`:

- A value that is not an `http`/`https` URL is a bare ID and is returned
  trimmed and otherwise unchanged. Bundle identifiers, product IDs, app names,
  and every other selector shape keep working.
- A URL is accepted only when its host is `api.appstoreconnect.apple.com`
  and its path is exactly `/v<n>/<type>/<id>`. The `<id>` segment is returned.
  Query strings and fragments are ignored.
- When `resourceType` is non-empty and `<type>` differs, the value is rejected
  with a message naming both types, for example
  `expected a self-link of type builds, got appStoreVersions`.
- A URL on another host, with a different path shape (including
  `/relationships/...` and related-resource paths), or with an empty `<id>` is
  rejected with a message describing the accepted shape.

`shared.BindResourceIDFlag(fs, name, resourceType, usage) *string` registers a
string flag backed by the normalizer; the value also implements `flag.Getter`
like the other custom flag values in that package. Rejections surface as flag
parse failures, so they print
`Error: invalid value "<url>" for flag -<name>: <reason>` on stderr and exit
with code 2 before authentication or any network call.

`shared.ResolveAppID` also normalizes `apps` self-links for the explicit
`--app` value, `ASC_APP_ID`, and the configured app ID, so every command that
resolves its app through that helper accepts an app self-link. Because that
helper cannot return an error, a wrong-type URL there is left untouched and
fails in the app lookup instead of as a usage error; the wired commands below
reject it at parse time.

## Wired flags

| Flag | Commands | Resource type |
| --- | --- | --- |
| `--build-id` | `builds info/update/expire/wait/...` (shared build selector), `builds add-groups`, `versions attach-build`, `validate testflight`, `testflight beta-notifications` | `builds` |
| `--app` | shared build selector, `builds list`, `versions list/view`, `testflight crashes list`, `testflight feedback list`, `subscriptions groups list`, `subscriptions list` | `apps` |
| `--id` | `apps view/update` | `apps` |
| `--version-id` | `versions view/update/release/attach-build/...` | `appStoreVersions` |
| `--version` | `localizations list` | `appStoreVersions` |
| `--submission-id` | `testflight feedback view/delete` | `betaFeedbackScreenshotSubmissions` |
| `--submission-id` | `testflight crashes view/delete/log` | `betaFeedbackCrashSubmissions` |
| `--id`, `--group`, `--group-id` | `testflight groups view/update/delete/add-testers/remove-testers` and related/relationship reads | `betaGroups` |
| `--localization-id` | `localizations search-keywords ...` | `appStoreVersionLocalizations` |
| `--info-id` | `apps info ...` | `appInfos` |
| `--subscription-id`, `--id` | `subscriptions view/update/delete`, offers, pricing | `subscriptions` |
| `--id`, `--iap-id` | `iap view/update/delete`, versions, prices, offers, availability, content, review screenshots | `inAppPurchases` |
| `--version-id` | `iap versions ...`, `iap versions images/localizations ...` | `inAppPurchaseVersions` |
| `--id`, `--bundle` | `bundle-ids view/update/delete`, capabilities, relationships | `bundleIds` |

### Remaining single-value ID flags

Every other single-value `-id` flag that names an App Store Connect API
resource is wired the same way, so the remaining deferred work is only the
exclusions below. 399 flag declarations across 125 files in 36 command areas,
covering 88 resource types:

| Command area | Flags | Resource type |
| --- | --- | --- |
| `asc age-rating` | `--app-info-id` | `appInfos` |
| `asc age-rating` | `--version-id` | `appStoreVersions` |
| `asc alternative-distribution` | `--domain-id` | `alternativeDistributionDomains` |
| `asc alternative-distribution` | `--key-id` | `alternativeDistributionKeys` |
| `asc alternative-distribution` | `--delta-id` | `alternativeDistributionPackageDeltas` |
| `asc alternative-distribution` | `--variant-id` | `alternativeDistributionPackageVariants` |
| `asc alternative-distribution` | `--version-id` | `alternativeDistributionPackageVersions` |
| `asc alternative-distribution` | `--package-id` | `alternativeDistributionPackages` |
| `asc alternative-distribution` | `--app-store-version-id` | `appStoreVersions` |
| `asc analytics` | `--instance-id` | `analyticsReportInstances` |
| `asc analytics` | `--request-id` | `analyticsReportRequests` |
| `asc analytics` | `--segment-id` | `analyticsReportSegments` |
| `asc analytics` | `--report-id` | `analyticsReports` |
| `asc android-ios-mapping` | `--mapping-id` | `androidToIosAppMappingDetails` |
| `asc app-clips` | `--header-image-id` | `appClipAdvancedExperienceImages` |
| `asc app-clips` | `--experience-id` | `appClipAdvancedExperiences` |
| `asc app-clips` | `--localization-id` | `appClipDefaultExperienceLocalizations` |
| `asc app-clips` | `--experience-id`, `--template-id` | `appClipDefaultExperiences` |
| `asc app-clips` | `--app-clip-id` | `appClips` |
| `asc app-clips` | `--release-version-id` | `appStoreVersions` |
| `asc app-clips` | `--localization-id` | `betaAppClipInvocationLocalizations` |
| `asc app-clips` | `--invocation-id` | `betaAppClipInvocations` |
| `asc app-events` | `--localization-id` | `appEventLocalizations` |
| `asc app-events` | `--screenshot-id` | `appEventScreenshots` |
| `asc app-events` | `--clip-id` | `appEventVideoClips` |
| `asc app-events` | `--event-id` | `appEvents` |
| `asc apps` | `--version-id` | `appStoreVersions` |
| `asc background-assets` | `--upload-file-id` | `backgroundAssetUploadFiles` |
| `asc background-assets` | `--version-id` | `backgroundAssetVersions` |
| `asc background-assets` | `--background-asset-id` | `backgroundAssets` |
| `asc background-assets` | `--review-submission-id` | `reviewSubmissions` |
| `asc build-bundles` | `--build-id` | `builds` |
| `asc build-localizations` | `--build-id` | `builds` |
| `asc builds` | `--localization-id` | `betaBuildLocalizations` |
| `asc builds` | `--build-id` | `builds` |
| `asc categories` | `--category-id` | `appCategories` |
| `asc certificates` | `--pass-type-id` | `passTypeIds` |
| `asc certificates / merchant-ids certificates create` | `--merchant-id` | `merchantIds` |
| `asc game-center` | `--app-store-version-id` | `appStoreVersions` |
| `asc game-center` | `--localization-id` | `gameCenterAchievementLocalizations` |
| `asc game-center` | `--version-id` | `gameCenterAchievementVersions` |
| `asc game-center` | `--achievement-id` | `gameCenterAchievements` |
| `asc game-center` | `--activity-id` | `gameCenterActivities` |
| `asc game-center` | `--localization-id` | `gameCenterActivityLocalizations` |
| `asc game-center` | `--version-id` | `gameCenterActivityVersions` |
| `asc game-center` | `--localization-id` | `gameCenterChallengeLocalizations` |
| `asc game-center` | `--version-id` | `gameCenterChallengeVersions` |
| `asc game-center` | `--challenge-id` | `gameCenterChallenges` |
| `asc game-center` | `--game-center-group-id`, `--group-id` | `gameCenterGroups` |
| `asc game-center` | `--localization-id` | `gameCenterLeaderboardLocalizations` |
| `asc game-center` | `--localization-id` | `gameCenterLeaderboardSetLocalizations` |
| `asc game-center` | `--version-id` | `gameCenterLeaderboardSetVersions` |
| `asc game-center` | `--leaderboard-set-id`, `--set-id` | `gameCenterLeaderboardSets` |
| `asc game-center` | `--version-id` | `gameCenterLeaderboardVersions` |
| `asc game-center` | `--default-leaderboard-id`, `--default-leaderboard-v2-id`, `--default-group-leaderboard-id`, `--default-group-leaderboard-v2-id`, `--leaderboard-id` | `gameCenterLeaderboards` |
| `asc game-center` | `--queue-id` | `gameCenterMatchmakingQueues` |
| `asc game-center` | `--experiment-rule-set-id`, `--rule-set-id` | `gameCenterMatchmakingRuleSets` |
| `asc game-center` | `--rule-id` | `gameCenterMatchmakingRules` |
| `asc iap` | `--screenshot-id` | `inAppPurchaseAppStoreReviewScreenshots` |
| `asc iap` | `--content-id` | `inAppPurchaseContents` |
| `asc iap` | `--image-id` | `inAppPurchaseImages` |
| `asc iap` | `--localization-id` | `inAppPurchaseLocalizations` |
| `asc iap` | `--custom-code-id` | `inAppPurchaseOfferCodeCustomCodes` |
| `asc iap` | `--one-time-code-id` | `inAppPurchaseOfferCodeOneTimeUseCodes` |
| `asc iap` | `--offer-code-id` | `inAppPurchaseOfferCodes` |
| `asc iap` | `--schedule-id` | `inAppPurchasePriceSchedules` |
| `asc iap / subscriptions promoted-purchases` | `--promoted-purchase-id` | `promotedPurchases` |
| `asc marketplace` | `--search-detail-id` | `marketplaceSearchDetails` |
| `asc marketplace` | `--webhook-id` | `marketplaceWebhooks` |
| `asc merchant-ids` | `--merchant-id` | `merchantIds` |
| `asc metadata` | `--version-id` | `appStoreVersions` |
| `asc migrate` | `--version-id` | `appStoreVersions` |
| `asc offer-codes` | `--batch-id` | `subscriptionOfferCodeOneTimeUseCodes` |
| `asc offer-codes` | `--offer-code-id` | `subscriptionOfferCodes` |
| `asc offer-codes` | `--subscription-id` | `subscriptions` |
| `asc pass-type-ids` | `--pass-type-id` | `passTypeIds` |
| `asc performance` | `--build-id` | `builds` |
| `asc product-pages` | `--localization-id` | `appCustomProductPageLocalizations` |
| `asc product-pages` | `--custom-page-version-id` | `appCustomProductPageVersions` |
| `asc product-pages` | `--custom-page-id` | `appCustomProductPages` |
| `asc product-pages` | `--localization-id` | `appStoreVersionExperimentTreatmentLocalizations` |
| `asc product-pages` | `--treatment-id` | `appStoreVersionExperimentTreatments` |
| `asc product-pages` | `--experiment-id` | `appStoreVersionExperiments` |
| `asc product-pages` | `--version-id` | `appStoreVersions` |
| `asc publish` | `--build-id` | `builds` |
| `asc release` | `--build-id` | `builds` |
| `asc reviews` | `--version-id` | `appStoreVersions` |
| `asc reviews` | `--build-id` | `builds` |
| `asc reviews` | `--review-id` | `customerReviews` |
| `asc routing-coverage` | `--version-id` | `appStoreVersions` |
| `asc screenshots` | `--version-id` | `appStoreVersions` |
| `asc submit` | `--version-id` | `appStoreVersions` |
| `asc subscriptions` | `--screenshot-id` | `subscriptionAppStoreReviewScreenshots` |
| `asc subscriptions` | `--availability-id` | `subscriptionAvailabilities` |
| `asc subscriptions` | `--version-id` | `subscriptionGroupVersions` |
| `asc subscriptions` | `--group-id` | `subscriptionGroups` |
| `asc subscriptions` | `--batch-id` | `subscriptionOfferCodeOneTimeUseCodes` |
| `asc subscriptions` | `--offer-code-id` | `subscriptionOfferCodes` |
| `asc subscriptions` | `--price-point-id` | `subscriptionPricePoints` |
| `asc subscriptions` | `--price-id` | `subscriptionPrices` |
| `asc subscriptions` | `--version-id` | `subscriptionVersions` |
| `asc subscriptions` | `--source-subscription-id`, `--target-subscription-id` | `subscriptions` |
| `asc testflight` | `--crash-log-id` | `betaCrashLogs` |
| `asc testflight` | `--tester-id` | `betaTesters` |
| `asc testflight` | `--build-id` | `builds` |
| `asc validate` | `--version-id` | `appStoreVersions` |
| `asc versions` | `--treatment-id` | `appStoreVersionExperimentTreatments` |
| `asc webhooks` | `--webhook-id` | `webhooks` |
| `asc xcode-cloud` | `--action-id` | `ciBuildActions` |
| `asc xcode-cloud` | `--run-id`, `--source-run-id` | `ciBuildRuns` |
| `asc xcode-cloud` | `--workflow-id` | `ciWorkflows` |
| `asc xcode-cloud` | `--git-reference-id` | `scmGitReferences` |
| `asc xcode-cloud` | `--pull-request-id` | `scmPullRequests` |

The `--id` alias of `--tester-id` on the `asc testflight testers` apps,
groups, builds, links, and metrics reads is bound to `betaTesters` too, so the
alias and the canonical flag behave identically.

`--source-subscription-id` and `--target-subscription-id` on
`asc subscriptions pricing derive`, and `--subscription-id` on
`asc offer-codes`, are selector-style (see below). Flags typed
`gameCenterMatchmakingRuleSets`, `appStoreVersionExperiments`, and the other
collections Apple serves under both `/v1` and `/v2` accept either version: the
normalizer matches only the `<type>` segment, never the version segment.

### Selector-style flags

`--subscription-id` and `--iap-id` on most `subscriptions` and `iap`
subcommands accept "ID, product ID, or exact current name". For those flags
the extracted `<id>` then follows exactly the path a pasted bare ID takes:
`shared.SelectorNeedsLookup` treats a numeric value as a stable ASC ID (the
resolvers already define numeric as the ID shape for these resources, and
App Store Connect issues numeric IDs for in-app purchases and subscriptions),
so it is used directly, or resolved and then used directly if the
app-scoped lookup misses. A self-link therefore never introduces a failure
mode that the equivalent bare ID does not already have; provenance is not
carried past flag parsing on purpose, so callers keep a plain `*string`.

### Excluded on purpose

- Comma-separated ID filters, such as `--subscription-id` on
  `subscriptions price-points equalizations` and `adjusted-equalizations`,
  `--build-id` on `testflight testers add-builds`/`remove-builds`, and
  `--certificate-id` on `merchant-ids certificates list`. The normalizer takes one resource URL, so binding
  it to a CSV flag would accept a single link but reject a list of them with a
  message about single resources. A per-element variant is follow-up work.
- `--bundle-id` and `--iap-id` under `asc web`: those name Developer Portal
  and Iris resource IDs, not App Store Connect API resources, so there is no
  self-link shape to accept. Every `-id` flag under `asc web` is excluded for
  the same reason.
- Values that are identifiers rather than resource IDs: reverse-DNS bundle
  identifiers (`asc apps update --bundle-id`, `asc signing ...`,
  `asc shots ...`), Game Center vendor identifiers, in-app purchase and
  subscription `productId` attributes, `offerId` on
  `asc win-back-offers create --offer-id`, seed IDs, scoped player IDs, and
  locales or territories.
  A URL is never a valid value for these, so the normalizer would only add a
  misleading error message.
- Credentials and local identifiers, which are not API resources at all:
  `asc auth --key-id`/`--issuer-id`, the Apple Ads OAuth IDs, Apple Developer
  team IDs for `xcodebuild`, the cached Apple Account value on
  `asc validate --apple-id`, `asc xcode install --device-id` (a local
  CoreDevice identifier), and the `asc storekit` flags, which address
  `api.storekit.apple.com` and local StoreKit configuration files.
- Flags whose resource type depends on another flag:
  `asc review items add --item-id` (and its `asc review items-add` alias)
  names one of five collections chosen by `--item-type`, and
  `asc iap promoted-purchases create --product-id` names either `subscriptions` or
  `inAppPurchases` depending on `--product-type`. Binding a single type would
  reject valid links.
- Resources with no single-resource path in `docs/openapi/paths.txt`:
  `buildBundles`, `webhookDeliveries`, `diagnosticSignatures`,
  `inAppPurchasePricePoints`, and `gameCenterEnabledVersions` are only
  reachable through a parent or nested path, so there is no
  `/v<n>/<type>/<id>` self-link to accept.
- `asc localizations update --id` and `asc localizations view --id` name an `appStoreVersionLocalizations` or
  an `appInfoLocalizations` resource depending on `--type`, like the
  type-dependent flags above.
- `asc versions rating-reset` runs against a web session, so it is excluded
  with the other `asc web` surfaces.
- `asc screenshots review-approve --id` names an entry in the local review
  manifest, and `asc reviews ratings --app` and the `asc apps public`
  commands query the public App Store rather than the App Store Connect API.

### Completing coverage

A command-level audit after the two passes above found the localization media
set commands (`asc localizations preview-sets` and `screenshot-sets`) still
sending a pasted link as a path segment, along with every flag built by the
shared command builders, the bare `--id` flags, and a few resource flags
without an `-id` suffix. They are wired the same way now:

- `shared.IDGetCommandConfig` and `shared.ConfirmDeleteCommandConfig` take an
  `IDType`, and `shared.PaginatedListCommandConfig` and
  `shared.PricePointEqualizationsCommandConfig` take a `ParentType`. A builder
  given a type binds its flag through `BindResourceIDFlag`; one without a type
  keeps a plain string flag, which the guard test below rejects unless the
  flag is an excluded one.
- `--app` flags that read the raw value instead of calling
  `shared.ResolveAppID` now normalize too: `asc performance download` and
  `asc signing fetch` bind `--app` as an `apps` flag, and the commands whose
  `--app` default comes from `ASC_APP_ID` (`asc app-setup info set`,
  `asc app-setup categories set`, `asc categories set`) keep their plain
  string flag, so help is unchanged, and normalize it with
  `shared.AppIDFlagValue`. A link of another type there prints the same
  `invalid value ... for flag -app` message and exits 2 before any request.

Flags wired in this pass (382 command flags in the tree):

| Command area | Flags | Resource type |
| --- | --- | --- |
| `asc accessibility` | `--id` | `accessibilityDeclarations` |
| `asc actors` | `--id` | `actors` |
| `asc age-rating` | `--id` | `ageRatingDeclarations` |
| `asc agreements` | `--id` | `endUserLicenseAgreements` |
| `asc app-clips` | `--id` | `appClipAdvancedExperienceImages` |
| `asc app-clips` | `--id` | `appClipAppStoreReviewDetails` |
| `asc app-clips` | `--id` | `appClipHeaderImages` |
| `asc app-clips` | `--id` | `appClips` |
| `asc app-setup` | `--primary`, `--primary-subcategory-one`, `--primary-subcategory-two`, `--secondary`, `--secondary-subcategory-one`, `--secondary-subcategory-two` | `appCategories` |
| `asc app-setup` | `--app-info` | `appInfos` |
| `asc app-setup` | `--price-point` | `appPricePoints` |
| `asc app-setup` | `--version` | `appStoreVersions` |
| `asc app-tags` | `--id` | `appTags` |
| `asc apps` | `--app-info` | `appInfos` |
| `asc apps` | `--id` | `apps` |
| `asc background-assets` | `--id` | `backgroundAssetVersionAppStoreReleases` |
| `asc background-assets` | `--id` | `backgroundAssetVersionExternalBetaReleases` |
| `asc background-assets` | `--id` | `backgroundAssetVersionInternalBetaReleases` |
| `asc background-assets` | `--id` | `backgroundAssets` |
| `asc build-localizations` | `--id` | `appStoreVersionLocalizations` |
| `asc builds` | `--id` | `buildUploadFiles` |
| `asc builds` | `--id`, `--upload` | `buildUploads` |
| `asc bundle-ids` | `--id` | `bundleIdCapabilities` |
| `asc bundle-ids` | `--bundle` | `bundleIds` |
| `asc categories` | `--primary`, `--primary-subcategory-one`, `--primary-subcategory-two`, `--secondary`, `--secondary-subcategory-one`, `--secondary-subcategory-two` | `appCategories` |
| `asc categories` | `--app-info` | `appInfos` |
| `asc certificates` | `--id` | `certificates` |
| `asc devices` | `--id` | `devices` |
| `asc diff` | `--from-version`, `--to-version`, `--version` | `appStoreVersions` |
| `asc encryption` | `--id` | `appEncryptionDeclarationDocuments` |
| `asc encryption` | `--declaration`, `--id` | `appEncryptionDeclarations` |
| `asc eula` | `--id` | `endUserLicenseAgreements` |
| `asc game-center` | `--id` | `gameCenterAchievementImages` |
| `asc game-center` | `--id` | `gameCenterAchievementLocalizations` |
| `asc game-center` | `--id` | `gameCenterAchievementReleases` |
| `asc game-center` | `--id` | `gameCenterAchievementVersions` |
| `asc game-center` | `--id` | `gameCenterAchievements` |
| `asc game-center` | `--id` | `gameCenterActivities` |
| `asc game-center` | `--id` | `gameCenterActivityImages` |
| `asc game-center` | `--id` | `gameCenterActivityLocalizations` |
| `asc game-center` | `--id` | `gameCenterActivityVersionReleases` |
| `asc game-center` | `--id` | `gameCenterActivityVersions` |
| `asc game-center` | `--id` | `gameCenterAppVersions` |
| `asc game-center` | `--id` | `gameCenterChallengeImages` |
| `asc game-center` | `--id` | `gameCenterChallengeLocalizations` |
| `asc game-center` | `--id` | `gameCenterChallengeVersionReleases` |
| `asc game-center` | `--id` | `gameCenterChallengeVersions` |
| `asc game-center` | `--id` | `gameCenterChallenges` |
| `asc game-center` | `--filter-detail`, `--id` | `gameCenterDetails` |
| `asc game-center` | `--id` | `gameCenterGroups` |
| `asc game-center` | `--id` | `gameCenterLeaderboardImages` |
| `asc game-center` | `--id` | `gameCenterLeaderboardLocalizations` |
| `asc game-center` | `--id` | `gameCenterLeaderboardReleases` |
| `asc game-center` | `--id` | `gameCenterLeaderboardSetImages` |
| `asc game-center` | `--id` | `gameCenterLeaderboardSetLocalizations` |
| `asc game-center` | `--id` | `gameCenterLeaderboardSetMemberLocalizations` |
| `asc game-center` | `--id` | `gameCenterLeaderboardSetReleases` |
| `asc game-center` | `--id` | `gameCenterLeaderboardSetVersions` |
| `asc game-center` | `--id` | `gameCenterLeaderboardSets` |
| `asc game-center` | `--id` | `gameCenterLeaderboardVersions` |
| `asc game-center` | `--id` | `gameCenterLeaderboards` |
| `asc game-center` | `--filter-queue`, `--id` | `gameCenterMatchmakingQueues` |
| `asc game-center` | `--id` | `gameCenterMatchmakingRuleSets` |
| `asc game-center` | `--id` | `gameCenterMatchmakingRules` |
| `asc game-center` | `--id` | `gameCenterMatchmakingTeams` |
| `asc iap` | `--id` | `inAppPurchaseAvailabilities` |
| `asc iap` | `--submission` | `reviewSubmissions` |
| `asc localizations` | `--app-info` | `appInfos` |
| `asc localizations` | `--id` | `appPreviewSets` |
| `asc localizations` | `--id` | `appScreenshotSets` |
| `asc localizations` | `--localization-id` | `appStoreVersionLocalizations` |
| `asc localizations` | `--version` | `appStoreVersions` |
| `asc metadata` | `--app-info` | `appInfos` |
| `asc migrate` | `--app-info` | `appInfos` |
| `asc nominations` | `--id` | `nominations` |
| `asc optimize` | `--app-info` | `appInfos` |
| `asc performance` | `--app` | `apps` |
| `asc pre-orders` | `--availability` | `appAvailabilities` |
| `asc pre-orders` | `--territory-availability` | `territoryAvailabilities` |
| `asc pricing` | `--availability`, `--id` | `appAvailabilities` |
| `asc pricing` | `--price-point` | `appPricePoints` |
| `asc pricing` | `--id`, `--schedule` | `appPriceSchedules` |
| `asc profiles` | `--bundle` | `bundleIds` |
| `asc profiles` | `--id` | `profiles` |
| `asc review` | `--id` | `appStoreReviewAttachments` |
| `asc review` | `--id`, `--review-detail` | `appStoreReviewDetails` |
| `asc review` | `--id` | `reviewSubmissionItems` |
| `asc review` | `--id`, `--submission` | `reviewSubmissions` |
| `asc reviews` | `--id` | `customerReviewResponses` |
| `asc reviews` | `--id` | `customerReviews` |
| `asc routing-coverage` | `--id` | `routingAppCoverages` |
| `asc sandbox` | `--id` | `sandboxTesters` |
| `asc screenshots` | `--id` | `appScreenshots` |
| `asc screenshots` | `--version-localization` | `appStoreVersionLocalizations` |
| `asc signing` | `--app` | `apps` |
| `asc submit` | `--id` | `reviewSubmissions` |
| `asc subscriptions` | `--id` | `subscriptionGracePeriods` |
| `asc subscriptions` | `--id` | `subscriptionGroupLocalizations` |
| `asc subscriptions` | `--id` | `subscriptionGroups` |
| `asc subscriptions` | `--id` | `subscriptionImages` |
| `asc subscriptions` | `--id` | `subscriptionIntroductoryOffers` |
| `asc subscriptions` | `--id` | `subscriptionLocalizations` |
| `asc subscriptions` | `--custom-code-id` | `subscriptionOfferCodeCustomCodes` |
| `asc subscriptions` | `--offer-code-id` | `subscriptionOfferCodes` |
| `asc subscriptions` | `--price-point` | `subscriptionPricePoints` |
| `asc subscriptions` | `--id` | `subscriptionPromotionalOffers` |
| `asc subscriptions` | `--id` | `subscriptionVersions` |
| `asc subscriptions` | `--id` | `winBackOffers` |
| `asc testflight` | `--id` | `betaAppLocalizations` |
| `asc testflight` | `--id` | `betaAppReviewDetails` |
| `asc testflight` | `--id` | `betaAppReviewSubmissions` |
| `asc testflight` | `--group` | `betaGroups` |
| `asc testflight` | `--id` | `betaLicenseAgreements` |
| `asc testflight` | `--id` | `betaRecruitmentCriteria` |
| `asc testflight` | `--filter-tester`, `--id` | `betaTesters` |
| `asc testflight` | `--id` | `buildBetaDetails` |
| `asc testflight` | `--id` | `preReleaseVersions` |
| `asc users` | `--id` | `userInvitations` |
| `asc users` | `--id` | `users` |
| `asc versions` | `--id` | `appStoreVersionPhasedReleases` |
| `asc video-previews` | `--id` | `appPreviews` |
| `asc video-previews` | `--version-localization` | `appStoreVersionLocalizations` |
| `asc xcode-cloud` | `--id` | `ciArtifacts` |
| `asc xcode-cloud` | `--id` | `ciBuildActions` |
| `asc xcode-cloud` | `--id` | `ciBuildRuns` |
| `asc xcode-cloud` | `--id` | `ciIssues` |
| `asc xcode-cloud` | `--id` | `ciMacOsVersions` |
| `asc xcode-cloud` | `--id` | `ciProducts` |
| `asc xcode-cloud` | `--id` | `ciTestResults` |
| `asc xcode-cloud` | `--id` | `ciWorkflows` |
| `asc xcode-cloud` | `--id` | `ciXcodeVersions` |
| `asc xcode-cloud` | `--id` | `scmGitReferences` |
| `asc xcode-cloud` | `--provider-id` | `scmProviders` |
| `asc xcode-cloud` | `--id` | `scmPullRequests` |
| `asc xcode-cloud` | `--id`, `--repo-id` | `scmRepositories` |

`internal/cli/cmdtest/self_link_ids_coverage_test.go` walks the real command
tree and fails when a flag named `id` or `*-id` is not bound to a typed
self-link normalizer, unless it is comma-separated, is an identifier flag
(`--bundle-id`, `--product-id`, `--vendor-id`, `--team-id`, credentials, and
similar), sits under an excluded command tree (`asc web`,
`asc versions rating-reset`, `asc ads`, `asc storekit`, `asc notarization`,
`asc xcode`, `asc auth`), or has a reasoned entry in its exclusion list. A
stale exclusion also fails the test, so the list tracks the tree.

## Alternatives considered

- Normalizing inside every resolver (`ResolveBuild`, `ResolveSubscriptionID`,
  ...): scattered, and the flag definition is the single place each command
  already declares which resource it names.
- Accepting any path and taking the third segment: silently turns
  `/v1/builds/<id>/relationships/betaGroups` into a build ID. Rejecting the
  longer path keeps the operator's intent visible.
- Accepting URLs from any host: a Developer Portal or App Store Connect web
  URL is not an API self-link; passing it through as an ID would produce a
  confusing not-found error from Apple.

## Verification

- Unit tests for the normalizer: bare ID, valid link, wrong type, longer path,
  other host, empty ID, query string, non-URL selectors.
- `cmdtest` coverage through the real command tree with a stub transport
  asserting the request path uses the extracted ID for `builds info`,
  `versions view`, and `testflight crashes view`, plus a wrong-type rejection
  exiting 2 before any request.
- `cmdtest` coverage for the remaining families
  (`internal/cli/cmdtest/self_link_ids_remaining_test.go`): twelve acceptance
  rows drive the real command tree against an `httptest` server and assert the
  request path carries the extracted ID (webhooks, app events, categories,
  custom product pages, Android-to-iOS mapping, alternative distribution
  domains, merchant IDs, analytics reports, Game Center achievements, App Clip
  default experiences, build test notes, beta testers), and twenty-six
  rejection rows assert exit 2 with the flag-parse message and no request at
  all, one per resource family plus the `--id` alias, a relationship-path row,
  and an other-host row.
- `cmdtest` coverage for the completing pass
  (`internal/cli/cmdtest/self_link_ids_coverage_test.go`): acceptance rows
  for the localization preview and screenshot sets, `screenshots` and
  `video-previews` listing by localization, builder-backed Xcode Cloud and
  offer-code commands, and representative bare `--id` reads; request-path
  checks for the `--app` flags that bypassed `shared.ResolveAppID`; one
  wrong-type rejection row per newly wired resource family; and the
  command-tree guard described above.

Certificate creation coverage also checks both the general and merchant-scoped
commands: a merchant self-link becomes the bare ID in the HTTP request
relationship, and a wrong-type link fails before any request.

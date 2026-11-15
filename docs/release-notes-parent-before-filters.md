# A missing parent flag is reported before bad filter values in every list command

Changed in 7.0.0. When a list command is missing the flag that names what to
list (the app, group, version or other parent resource) and another flag value
is also wrong, every list command now reports the missing flag first. Before,
62 commands checked some filter, field, sort or limit values first.

All list commands now check their flags in this order:

1. `--limit`
2. `--next`, then flags that cannot be combined with `--next`
3. the parent flag
4. filters, fields, sort, include limits and other flag values, including
   checks that combine two of them

```text
$ asc builds list --sort bogus
Error: --app is required (or set ASC_APP_ID)                  # was "builds: --sort must be one of: ..."

$ asc subscriptions offers win-back list --fields bogus
Error: --subscription-id is required                          # was "subscriptions offers win-back list: --fields must be one of: ..."
```

Only which error comes first changes. The error texts, exit code `2` and the
usage page are unchanged. When the parent flag or `--next` is given, every
command reports the same errors as before.

## Commands that now report the parent flag first

Each line names the command, its missing-parent error, and the flags whose
checks now come after it. Checks that combine two of these flags also come
after it, for example `--territory-fields requires --include territories`,
`--internal and --external are mutually exclusive` (`asc testflight groups
list`) and `--include-versions, --versions-limit, and --version-fields require
the v2 endpoint` (`asc iap list --legacy`).

- `asc accessibility list` (`--app is required (or set ASC_APP_ID)`): `--device-family`, `--fields`, `--state`
- `asc app-clips advanced-experiences list` (`--app-clip-id is required`): `--action`
- `asc app-events list` (`--app is required (or set ASC_APP_ID)`): `--event-state`, `--fields`, `--id`, `--include`, `--localization-fields`, `--localizations-limit`
- `asc app-events localizations placements list` (`app-events localizations placements list: --localization-id is required`): `--asset-image-fields`, `--asset-video-fields`, `--custom-page-localization`, `--image-id`, `--include`, `--placement-id`, `--placement-type`, `--retention-message-version-fields`, `--sort`, `--state`, `--treatment-localization`, `--version-localization`, `--video-id`
- `asc app-tags list` (`--app is required (or set ASC_APP_ID)`): `--fields`, `--include`, `--sort`, `--territory-fields`, `--territory-limit`, `--visible-in-app-store`
- `asc apps app-encryption-declarations list` (`--id is required (or set ASC_APP_ID)`): `--app-fields`, `--build-fields`, `--build-limit`, `--document-fields`, `--fields`, `--include`
- `asc asset-library images list` (`asset-library: --library-id is required`): `--category`, `--id`, `--reference-name`, `--sort`, `--spec-id`, `--state`
- `asc asset-library images placements` (`asset-library: --id is required`): `--custom-page-localization`, `--event-localization`, `--placement-group`, `--placement-id`, `--placement-type`, `--sort`, `--state`, `--treatment-localization`, `--version-localization`, `--video-id`
- `asc asset-library videos list` (`asset-library: --library-id is required`): `--category`, `--id`, `--reference-name`, `--sort`, `--spec-id`, `--state`
- `asc asset-library videos placements` (`asset-library: --id is required`): `--custom-page-localization`, `--event-localization`, `--image-id`, `--placement-group`, `--placement-id`, `--placement-type`, `--sort`, `--state`, `--treatment-localization`, `--version-localization`
- `asc builds list` (`--app is required (or set ASC_APP_ID)`): `--app-fields`, `--app-store-version-fields`, `--beta-app-review-submission-fields`, `--beta-build-localization-fields`, `--beta-group-fields`, `--beta-review-state`, `--build-beta-detail-fields`, `--build-bundle-fields`, `--build-icon-fields`, `--build-upload-fields`, `--encryption-declaration-fields`, `--include`, `--platform`, `--pre-release-version-fields`, `--processing-state`, `--sort`, `--tester-fields`
- `asc builds uploads list` (`--app is required (or set ASC_APP_ID)`): `--sort`
- `asc encryption declarations list` (`--app is required (or set ASC_APP_ID)`): `--app-fields`, `--build-fields`, `--build-limit`, `--document-fields`, `--fields`, `--include`
- `asc iap list` (`--app is required (or set ASC_APP_ID)`): `--fields`, `--iap-availability-fields`, `--iap-content-fields`, `--iap-image-fields`, `--iap-localization-fields`, `--iap-offer-code-fields`, `--iap-price-schedule-fields`, `--iap-review-screenshot-fields`, `--include`, `--name`, `--product-id`, `--promoted-purchase-fields`, `--sort`, `--state`, `--type`, `--version-fields`, `--versions-limit`
- `asc iap pricing price-points equalizations` (`--id is required`): `--include`, `--territory-fields`
- `asc iap pricing price-points list` (`--iap-id is required`): `--include`, `--max-price`, `--min-price`, `--price`, `--territory-fields`
- `asc iap promoted-purchases list` (`--app is required (or set ASC_APP_ID)`): `--iap-fields`, `--subscription-fields`
- `asc localizations list` (`--version is required for version localizations (or pass --app to use the app's editable or live version)`): `--app-info-fields`, `--app-store-version-fields`, `--include`, `--placements-limit`, `--platform`, `--preview-set-fields`, `--screenshot-set-fields`
- `asc localizations placements list` (`localizations placements list: --localization-id is required`): `--asset-image-fields`, `--asset-video-fields`, `--custom-page-localization`, `--event-localization`, `--image-id`, `--include`, `--placement-id`, `--placement-type`, `--retention-message-version-fields`, `--sort`, `--state`, `--treatment-localization`, `--video-id`
- `asc localizations screenshot-sets list` (`--localization-id is required`): `--include`, `--screenshot-fields`
- `asc nominations list` (`--status is required`): `--in-app-events-limit`, `--related-apps-limit`, `--sort`, `--supported-territories-limit`
- `asc pricing price-points equalizations` (`--price-point is required`): `--include`, `--territory-fields`
- `asc product-pages custom-pages list` (`--app is required (or set ASC_APP_ID)`): `--app-fields`, `--fields`, `--include`, `--version-fields`, `--versions-limit`, `--visible`
- `asc product-pages custom-pages localizations list` (`--custom-page-version-id is required`): `--include`, `--placements-limit`
- `asc product-pages custom-pages localizations placements list` (`product-pages custom-pages localizations placements list: --localization-id is required`): `--asset-image-fields`, `--asset-video-fields`, `--event-localization`, `--image-id`, `--include`, `--placement-id`, `--placement-type`, `--retention-message-version-fields`, `--sort`, `--state`, `--treatment-localization`, `--version-localization`, `--video-id`
- `asc product-pages experiments list` (`--version-id is required`): `--state`
- `asc product-pages experiments treatments localizations list` (`--treatment-id is required`): `--include`, `--placements-limit`
- `asc product-pages experiments treatments localizations placements list` (`product-pages experiments treatments localizations placements list: --localization-id is required`): `--asset-image-fields`, `--asset-video-fields`, `--custom-page-localization`, `--event-localization`, `--image-id`, `--include`, `--placement-id`, `--placement-type`, `--retention-message-version-fields`, `--sort`, `--state`, `--version-localization`, `--video-id`
- `asc review attachments-list` (`--review-detail is required`): `--detail-fields`, `--fields`, `--include`
- `asc review items list` (`--submission is required`): `--app-event-fields`, `--app-store-version-fields`, `--asset-image-fields`, `--asset-video-fields`, `--background-asset-version-fields`, `--custom-product-page-version-fields`, `--experiment-fields`, `--fields`, `--game-center-achievement-version-fields`, `--game-center-activity-version-fields`, `--game-center-challenge-version-fields`, `--game-center-leaderboard-set-version-fields`, `--game-center-leaderboard-version-fields`, `--iap-version-fields`, `--include`, `--retention-message-version-fields`, `--subscription-group-version-fields`, `--subscription-version-fields`
- `asc review items-list` (`--submission is required`): `--app-event-fields`, `--app-store-version-fields`, `--asset-image-fields`, `--asset-video-fields`, `--background-asset-version-fields`, `--custom-product-page-version-fields`, `--experiment-fields`, `--fields`, `--game-center-achievement-version-fields`, `--game-center-activity-version-fields`, `--game-center-challenge-version-fields`, `--game-center-leaderboard-set-version-fields`, `--game-center-leaderboard-version-fields`, `--iap-version-fields`, `--include`, `--retention-message-version-fields`, `--subscription-group-version-fields`, `--subscription-version-fields`
- `asc review submissions list` (`--app or --global is required (or set ASC_APP_ID)`): `--actor-fields`, `--app-fields`, `--app-store-version-fields`, `--include`, `--item-fields`, `--platform`, `--state`
- `asc review submissions-list` (`--app or --global is required (or set ASC_APP_ID)`): `--actor-fields`, `--app-fields`, `--app-store-version-fields`, `--include`, `--item-fields`, `--platform`, `--state`
- `asc subscriptions groups list` (`--app is required (or set ASC_APP_ID)`): `--fields`, `--include`, `--localization-fields`, `--subscription-fields`, `--version-fields`, `--versions-limit`
- `asc subscriptions list` (`--group-id or --app is required (or set ASC_APP_ID)`): `--availability-fields`, `--fields`, `--group-fields`, `--image-fields`, `--include`, `--introductory-offer-fields`, `--localization-fields`, `--offer-code-fields`, `--plan-availability-fields`, `--price-fields`, `--promoted-purchase-fields`, `--promotional-offer-fields`, `--retention-message-fields`, `--retention-offer-fields`, `--review-screenshot-fields`, `--version-fields`, `--versions-limit`, `--win-back-offer-fields`
- `asc subscriptions offers introductory list` (`--subscription-id is required`): `--include`, `--price-point-fields`, `--subscription-fields`, `--territory`
- `asc subscriptions offers offer-codes list` (`--subscription-id is required`): `--subscription-fields`
- `asc subscriptions offers offer-codes prices` (`--offer-code-id is required`): `--price-point-fields`
- `asc subscriptions offers promotional list` (`--subscription-id is required`): `--subscription-fields`
- `asc subscriptions offers promotional prices` (`--id is required`): `--price-point-fields`
- `asc subscriptions offers retention list` (`--subscription-id is required`): `--fields`, `--include`, `--price-fields`, `--prices-limit`, `--subscription-fields`
- `asc subscriptions offers retention presented list` (`--subscription-id is required`): `--fields`, `--include`, `--price-fields`, `--prices-limit`, `--subscription-fields`
- `asc subscriptions offers retention prices list` (`--id is required`): `--fields`, `--include`, `--price-point-fields`, `--territory`, `--territory-fields`
- `asc subscriptions offers win-back list` (`--subscription-id is required`): `--fields`, `--include`, `--price-fields`, `--prices-limit`
- `asc subscriptions pricing price-points list` (`--subscription-id is required`): `--max-price`, `--min-price`, `--price`, `--stream`, `--app with --stream`, `--fields with --stream`, `--include with --stream`, `--plan-type with --stream`, `--territory with --stream`, `--territory-fields with --stream`, `--upfront-price-point-id with --stream`
- `asc subscriptions promoted-purchases list` (`--app is required (or set ASC_APP_ID)`): `--iap-fields`, `--subscription-fields`
- `asc subscriptions versions images list` (`--version-id is required`): `--fields`
- `asc subscriptions versions list` (`--subscription-id is required`): `--fields`, `--image-fields`, `--images-limit`, `--include`, `--localization-fields`, `--localizations-limit`, `--state`, `--subscription-fields`
- `asc subscriptions versions localizations list` (`--version-id is required`): `--fields`, `--include`, `--version-fields`
- `asc testflight crashes list` (`--app is required (or set ASC_APP_ID)`): `--build-fields`, `--include`, `--sort`, `--tester-fields`
- `asc testflight feedback list` (`--app is required (or set ASC_APP_ID)`): `--build-fields`, `--include`, `--sort`, `--tester-fields`
- `asc testflight groups builds list` (`--group-id is required`): `--fields`
- `asc testflight groups list` (`--app or --global is required (or set ASC_APP_ID)`): `--app-fields`, `--build-fields`, `--builds-limit`, `--fields`, `--id`, `--include`, `--name`, `--public-link`, `--public-link-enabled`, `--public-link-limit-enabled`, `--recruitment-criteria-fields`, `--sort`, `--tester-fields`, `--testers-limit`
- `asc testflight groups testers list` (`--group-id is required`): `--fields`
- `asc testflight metrics app-testers` (`--app is required (or set ASC_APP_ID)`): `--period`
- `asc testflight pre-release list` (`--app is required (or set ASC_APP_ID)`): `--platform`
- `asc testflight testers list` (`--app is required (or set ASC_APP_ID)`): `--app-fields`, `--beta-group-fields`, `--build-fields`, `--fields`, `--include`, `--invite-type`, `--sort`
- `asc testflight testers metrics` (`--tester-id is required`): `--period`
- `asc versions customer-reviews list` (`--version-id is required`): `--fields`, `--include`, `--response-fields`, `--response-state`, `--review-territory`, `--sort`, `--stars`, `--territory`, `--territory-fields`
- `asc versions list` (`--app is required (or set ASC_APP_ID)`): `--alternative-distribution-package-fields`, `--app-clip-default-experience-fields`, `--app-fields`, `--build-fields`, `--experiment-fields`, `--game-center-app-version-fields`, `--include`, `--phased-release-fields`, `--platform`, `--review-detail-fields`, `--routing-app-coverage-fields`, `--state`, `--version-localization-fields`, `--version-submission-fields`
- `asc xcode-cloud build-runs` (`--workflow-id is required. Find workflow IDs with: asc xcode-cloud workflows list --app "APP_ID"`): `--sort`
- `asc xcode-cloud build-runs list` (`--workflow-id is required. Find workflow IDs with: asc xcode-cloud workflows list --app "APP_ID"`): `--sort`

`asc localizations list` keeps `--type` and the pairing of `--version` and
`--app-info` with `--type` first, because `--type` decides which parent flag is
required (`--version` or `--app`). `asc product-pages experiments list` keeps
reading `--v2` first, because it decides between `--version-id` and `--app`.
`asc testflight testers metrics` keeps `--tester-id and --id must match`
first, because `--id` is another name for the parent flag.

## Commands that keep another check first

- `asc localizations download`: `--type` decides which parent flag is required,
  so a bad `--type` is still reported first.
- `asc builds test-notes list`: the build is chosen by `--build-id` or by `--app`
  with `--latest` or `--build-number`, and that check also validates how those
  flags combine. `--locale` is still checked before it.

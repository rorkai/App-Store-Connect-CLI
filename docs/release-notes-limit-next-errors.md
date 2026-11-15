# `--limit` and `--next` mistakes are reported the same way in every list command

Changed in 7.0.0. Every list command now reports a mistake with `--limit` or
`--next` with the same text, and checks its flags in the same order:

1. `--limit` outside the command's range:
   `<command>: --limit must be between 1 and <maximum>`
2. a `--next` value that is not an App Store Connect URL:
   `<command>: --next must be an App Store Connect URL`
3. a flag that cannot be combined with `--next`:
   `<command>: --next cannot be combined with --<flag>`
4. the parent flag (the app, group, version or other resource the list
   belongs to), then filters and other flag values.

`<command>` is the path you typed without `asc`. Exit code `2` and the usage
page are unchanged.

```text
$ asc builds list --app 123456789 --limit 500
Error: builds list: --limit must be between 1 and 200      # was "builds: --limit ..."

$ asc webhooks list --limit 500
Error: webhooks list: --limit must be between 1 and 200    # was "--app is required (or set ASC_APP_ID)"
```

Each command keeps its own maximum (for example 50 for `asc build-bundles list`,
8000 for the price point equalization lists, 100 for
`asc storekit groups members`). `--limit 0` still means "use the server
default" wherever it did before; the commands that reject `--limit 0` still do.
Which flags may be combined with `--next` is unchanged.

## Messages that now name the command

`--limit` messages (60 commands):

- `asc app-events localizations placements list`
- `asc apps info view`
- `asc apps list`
- `asc apps public search`
- `asc apps wall`
- `asc asset-library images list`
- `asc asset-library images placements`
- `asc asset-library videos list`
- `asc asset-library videos placements`
- `asc builds list`
- `asc builds metrics beta-usages`
- `asc builds uploads list`
- `asc bundle-ids list`
- `asc categories subcategories`
- `asc iap pricing availabilities available-territories`
- `asc iap pricing price-points list`
- `asc iap promoted-purchases list`
- `asc localizations placements list`
- `asc performance download`
- `asc product-pages custom-pages list`
- `asc product-pages custom-pages localizations list`
- `asc product-pages custom-pages localizations placements list`
- `asc product-pages custom-pages localizations preview-sets list`
- `asc product-pages custom-pages localizations screenshot-sets list`
- `asc product-pages custom-pages versions list`
- `asc product-pages experiments list`
- `asc product-pages experiments treatments list`
- `asc product-pages experiments treatments localizations list`
- `asc product-pages experiments treatments localizations placements list`
- `asc product-pages experiments treatments localizations preview-sets list`
- `asc product-pages experiments treatments localizations screenshot-sets list`
- `asc review history`
- `asc review items list`
- `asc review items-list`
- `asc review submissions list`
- `asc review submissions-items-ids`
- `asc review submissions-list`
- `asc reviews list`
- `asc screenshots list`
- `asc storekit groups members`
- `asc subscriptions promoted-purchases list`
- `asc subscriptions versions images links`
- `asc subscriptions versions images list`
- `asc subscriptions versions links`
- `asc subscriptions versions list`
- `asc subscriptions versions localizations links`
- `asc subscriptions versions localizations list`
- `asc testflight agreements list`
- `asc testflight groups list`
- `asc testflight metrics app-testers`
- `asc testflight testers list`
- `asc testflight testers metrics`
- `asc xcode-cloud actions list`
- `asc xcode-cloud build-runs list`
- `asc xcode-cloud macos-versions list`
- `asc xcode-cloud products list`
- `asc xcode-cloud scm providers list`
- `asc xcode-cloud scm repositories list`
- `asc xcode-cloud workflows list`
- `asc xcode-cloud xcode-versions list`

`--next` URL messages (31 commands):

- `asc apps info view`
- `asc apps list`
- `asc asset-library images list`
- `asc asset-library images placements`
- `asc asset-library videos list`
- `asc asset-library videos placements`
- `asc builds list`
- `asc iap pricing availabilities available-territories`
- `asc iap pricing price-points list`
- `asc product-pages custom-pages list`
- `asc product-pages custom-pages localizations list`
- `asc product-pages custom-pages localizations preview-sets list`
- `asc product-pages custom-pages localizations screenshot-sets list`
- `asc product-pages custom-pages versions list`
- `asc product-pages experiments list`
- `asc product-pages experiments treatments list`
- `asc product-pages experiments treatments localizations list`
- `asc product-pages experiments treatments localizations preview-sets list`
- `asc product-pages experiments treatments localizations screenshot-sets list`
- `asc reviews list`
- `asc testflight agreements list`
- `asc testflight groups list`
- `asc testflight testers list`
- `asc xcode-cloud actions list`
- `asc xcode-cloud build-runs list`
- `asc xcode-cloud macos-versions list`
- `asc xcode-cloud products list`
- `asc xcode-cloud scm providers list`
- `asc xcode-cloud scm repositories list`
- `asc xcode-cloud workflows list`
- `asc xcode-cloud xcode-versions list`

`--next cannot be combined with ...` messages (37 commands). Messages
that named several flags at once, or "query flags", now name the first flag
given:

- `asc apps info view`
- `asc apps list`
- `asc asset-library images list`
- `asc asset-library images placements`
- `asc asset-library videos list`
- `asc asset-library videos placements`
- `asc iap pricing price-points list`
- `asc iap promoted-purchases list`
- `asc product-pages custom-pages list`
- `asc product-pages custom-pages localizations list`
- `asc product-pages custom-pages localizations screenshot-sets list`
- `asc product-pages experiments treatments localizations list`
- `asc product-pages experiments treatments localizations screenshot-sets list`
- `asc reviews list`
- `asc subscriptions groups list`
- `asc subscriptions groups versions list`
- `asc subscriptions groups versions localizations list`
- `asc subscriptions list`
- `asc subscriptions offers introductory list`
- `asc subscriptions offers offer-codes list`
- `asc subscriptions offers offer-codes prices`
- `asc subscriptions offers promotional list`
- `asc subscriptions offers promotional prices`
- `asc subscriptions offers retention list`
- `asc subscriptions offers retention presented list`
- `asc subscriptions offers retention prices list`
- `asc subscriptions pricing price-points adjusted-equalizations`
- `asc subscriptions pricing price-points equalizations`
- `asc subscriptions pricing price-points list`
- `asc subscriptions pricing prices list`
- `asc subscriptions promoted-purchases list`
- `asc subscriptions versions images list`
- `asc subscriptions versions list`
- `asc subscriptions versions localizations list`
- `asc testflight groups list`
- `asc versions customer-reviews list`
- `asc xcode-cloud products list`

## `--limit` out of range together with `--next`

In every list command with both flags, an out-of-range `--limit` given with a
valid `--next` URL is now reported as a `--limit` error. Before, every command
reported `--next cannot be combined with --limit` first. An in-range `--limit`
with `--next` is still rejected as before:

```text
$ asc apps list --limit 500 --next "https://api.appstoreconnect.apple.com/v1/apps?cursor=AQ"
Error: apps list: --limit must be between 1 and 200       # was "apps list: --next cannot be combined with --limit"
```

`asc marketplace webhooks list` now prints its deprecation warning before this
error, as it does before its other errors. `asc web removed-apps list` checks
that `--next` is a web URL before it reports `--limit`.

## Commands that now check `--limit` first

When `--limit` is out of range and another flag is also wrong (a missing
parent flag, a bad `--next`, a flag conflict, or another flag value), these
92 commands now report `--limit`:

- `asc accessibility list`
- `asc actors list`
- `asc agreements territories list`
- `asc alternative-distribution packages versions deltas`
- `asc alternative-distribution packages versions list`
- `asc alternative-distribution packages versions variants`
- `asc analytics view`
- `asc android-ios-mapping list`
- `asc app-events list`
- `asc app-events localizations list`
- `asc app-tags territories`
- `asc app-tags territories-links`
- `asc apps public search`
- `asc background-assets list`
- `asc background-assets upload-files list`
- `asc background-assets versions list`
- `asc bundle-ids list`
- `asc bundle-ids profiles list`
- `asc categories list`
- `asc categories subcategories`
- `asc certificates list`
- `asc iap list`
- `asc iap versions images list`
- `asc iap versions links images`
- `asc iap versions links localizations`
- `asc iap versions links versions`
- `asc iap versions list`
- `asc iap versions localizations list`
- `asc localizations screenshot-sets list`
- `asc merchant-ids certificates list`
- `asc merchant-ids certificates view`
- `asc merchant-ids list`
- `asc pass-type-ids list`
- `asc performance diagnostics list`
- `asc performance diagnostics view`
- `asc performance download`
- `asc pricing availability territory-availabilities`
- `asc pricing price-points equalizations`
- `asc product-pages custom-pages list`
- `asc product-pages custom-pages localizations preview-sets list`
- `asc product-pages custom-pages localizations screenshot-sets list`
- `asc product-pages experiments treatments localizations preview-sets list`
- `asc product-pages experiments treatments localizations screenshot-sets list`
- `asc profiles links certificates`
- `asc profiles links devices`
- `asc profiles list`
- `asc review attachments-list`
- `asc review items list`
- `asc review items-list`
- `asc review submissions list`
- `asc review submissions-items-ids`
- `asc review submissions-list`
- `asc reviews`
- `asc reviews list`
- `asc reviews summarizations`
- `asc screenshots list`
- `asc storekit groups members`
- `asc subscriptions groups versions links localizations`
- `asc subscriptions groups versions links versions`
- `asc subscriptions groups versions list`
- `asc subscriptions groups versions localizations list`
- `asc subscriptions pricing price-points list`
- `asc subscriptions retention-messages links localizations`
- `asc subscriptions retention-messages links messages`
- `asc subscriptions retention-messages links subscriptions`
- `asc subscriptions retention-messages links versions`
- `asc subscriptions retention-messages list`
- `asc subscriptions retention-messages localizations list`
- `asc subscriptions retention-messages subscriptions list`
- `asc subscriptions retention-messages versions list`
- `asc subscriptions versions images links`
- `asc subscriptions versions images list`
- `asc subscriptions versions links`
- `asc subscriptions versions list`
- `asc subscriptions versions localizations links`
- `asc subscriptions versions localizations list`
- `asc testflight review submissions list`
- `asc users invites visible-apps list`
- `asc users list`
- `asc users visible-apps list`
- `asc users visible-apps view`
- `asc webhooks deliveries`
- `asc webhooks deliveries links`
- `asc webhooks list`
- `asc xcode-cloud actions`
- `asc xcode-cloud actions list`
- `asc xcode-cloud artifacts list`
- `asc xcode-cloud build-runs`
- `asc xcode-cloud build-runs builds`
- `asc xcode-cloud build-runs list`
- `asc xcode-cloud issues list`
- `asc xcode-cloud test-results list`

## Commands that now check `--next` before its conflicts, the parent flag and filters

- `asc analytics view`
- `asc apps app-encryption-declarations list`
- `asc apps list`
- `asc builds list`
- `asc encryption declarations list`
- `asc iap pricing price-points equalizations`
- `asc iap versions images list`
- `asc iap versions list`
- `asc iap versions localizations list`
- `asc localizations screenshot-sets list`
- `asc performance diagnostics list`
- `asc pricing availability territory-availabilities`
- `asc pricing price-points equalizations`
- `asc product-pages custom-pages localizations screenshot-sets list`
- `asc product-pages experiments treatments localizations screenshot-sets list`
- `asc subscriptions groups versions list`
- `asc subscriptions groups versions localizations list`
- `asc subscriptions list`
- `asc subscriptions versions images list`
- `asc subscriptions versions list`
- `asc subscriptions versions localizations list`
- `asc testflight review submissions list`
- `asc xcode-cloud artifacts list`
- `asc xcode-cloud build-runs`
- `asc xcode-cloud build-runs list`
- `asc xcode-cloud issues list`
- `asc xcode-cloud test-results list`

## Commands that keep their own `--limit` wording

These read other services or local data, not an App Store Connect list, and
keep their own limits and messages: `asc ads apps search`,
`asc ads change-history view` (Apple Ads), `asc optimize keywords discover`
(Apple Ads), `asc notarization list` (Notary service, trims locally),
`asc web removed-apps list` and `asc web xcode-cloud workflows options schemes`
(web session, `--next` takes a web URL), and `asc search` (searches the
command list).

## What to do

A script that matched the old first line of stderr should match the new text
above. A script that relied on a missing parent flag being reported before a
bad `--limit` gets the `--limit` error first; fix `--limit` and run again to see
the next problem. Exit code `2` is unchanged in every case.

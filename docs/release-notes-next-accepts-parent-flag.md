# `--next` accepts the parent flag the first page used

Changed in 7.0.0. A list command prints a `links.next` URL when there are more
pages. Running the same command again with `--next URL` added now works in
the list commands below, as it already did in the others: with `--next`, the
parent flag (the app, group, version or other resource the list belongs to) is
optional, and when it is given it is accepted. The next URL already names the page, so the parent value is not sent
and is not compared with the URL.

```text
$ asc reviews list --app 123456789 --next "https://api.appstoreconnect.apple.com/v1/apps/123456789/customerReviews?cursor=AQ"
```

Before, 70 commands stopped with `Error: --next cannot be combined with --app`
(or the command's own parent flag) and exit code `2`. They now fetch the page.
Filter, sort, field, include and `--limit` flags are unchanged: they still
cannot be combined with `--next`, because the next URL already carries them.

| Command | Parent flags now accepted with `--next` |
| --- | --- |
| `asc accessibility list` | `--app` |
| `asc app-events localizations placements list` | `--localization-id` |
| `asc asset-library images list` | `--library-id` |
| `asc asset-library images placements` | `--id` |
| `asc asset-library videos list` | `--library-id` |
| `asc asset-library videos placements` | `--id` |
| `asc builds list` | `--app` |
| `asc builds uploads list` | `--app` |
| `asc categories subcategories` | `--category-id` |
| `asc game-center details blocked-players list` | `--detail-id` |
| `asc game-center leaderboards v2 score-moderations list` | `--leaderboard-id` |
| `asc iap list` | `--app` |
| `asc iap versions images list` | `--version-id` |
| `asc iap versions links images` | `--version-id` |
| `asc iap versions links localizations` | `--version-id` |
| `asc iap versions links versions` | `--iap-id` |
| `asc iap versions list` | `--iap-id` |
| `asc iap versions localizations list` | `--version-id` |
| `asc localizations placements list` | `--localization-id` |
| `asc pricing availability territory-availabilities` | `--app` |
| `asc product-pages custom-pages localizations placements list` | `--localization-id` |
| `asc product-pages experiments treatments localizations placements list` | `--localization-id` |
| `asc review attachments-list` | `--review-detail` |
| `asc review items list` | `--submission` |
| `asc review items-list` | `--submission` |
| `asc review submissions list` | `--app` |
| `asc review submissions-items-ids` | `--id` |
| `asc review submissions-list` | `--app` |
| `asc reviews` | `--app` |
| `asc reviews list` | `--app` |
| `asc reviews summarizations` | `--app` |
| `asc subscriptions groups list` | `--app` |
| `asc subscriptions groups versions links localizations` | `--version-id` |
| `asc subscriptions groups versions links versions` | `--group-id` |
| `asc subscriptions groups versions list` | `--group-id` |
| `asc subscriptions groups versions localizations list` | `--version-id` |
| `asc subscriptions list` | `--group-id` |
| `asc subscriptions offers introductory list` | `--subscription-id`, `--app` |
| `asc subscriptions offers offer-codes list` | `--subscription-id`, `--app` |
| `asc subscriptions offers offer-codes prices` | `--offer-code-id` |
| `asc subscriptions offers promotional list` | `--subscription-id`, `--app` |
| `asc subscriptions offers promotional prices` | `--id` |
| `asc subscriptions offers retention list` | `--subscription-id`, `--app` |
| `asc subscriptions offers retention presented list` | `--subscription-id`, `--app` |
| `asc subscriptions offers retention prices list` | `--id` |
| `asc subscriptions pricing availability available-territories` | `--availability-id`, `--subscription-id`, `--app` |
| `asc subscriptions pricing plan-availability available-territories` | `--plan-availability-id` |
| `asc subscriptions pricing price-points adjusted-equalizations` | `--price-point-id` |
| `asc subscriptions pricing price-points equalizations` | `--price-point-id` |
| `asc subscriptions pricing price-points list` | `--subscription-id`, `--app` |
| `asc subscriptions pricing prices list` | `--subscription-id`, `--app` |
| `asc subscriptions retention-messages links localizations` | `--version-id` |
| `asc subscriptions retention-messages links messages` | `--app` |
| `asc subscriptions retention-messages links subscriptions` | `--message-id` |
| `asc subscriptions retention-messages links versions` | `--message-id` |
| `asc subscriptions retention-messages list` | `--app` |
| `asc subscriptions retention-messages localizations list` | `--version-id` |
| `asc subscriptions retention-messages subscriptions list` | `--message-id` |
| `asc subscriptions retention-messages versions list` | `--message-id` |
| `asc subscriptions versions images links` | `--version-id` |
| `asc subscriptions versions images list` | `--version-id` |
| `asc subscriptions versions links` | `--subscription-id`, `--app` |
| `asc subscriptions versions list` | `--subscription-id`, `--app` |
| `asc subscriptions versions localizations links` | `--version-id` |
| `asc subscriptions versions localizations list` | `--version-id` |
| `asc testflight groups builds list` | `--group-id` |
| `asc testflight groups testers list` | `--group-id` |
| `asc versions customer-reviews list` | `--version-id` |
| `asc versions list` | `--app` |
| `asc webhooks list` | `--app` |

These 17 commands required their parent flag even with `--next`
(`Error: --app-clip-id is required`, for example). The flag is now optional
with `--next`:

| Command | Parent flag no longer required with `--next` |
| --- | --- |
| `asc alternative-distribution packages versions deltas` | `--version-id` |
| `asc alternative-distribution packages versions list` | `--package-id` |
| `asc alternative-distribution packages versions variants` | `--version-id` |
| `asc android-ios-mapping list` | `--app` |
| `asc app-clips advanced-experiences list` | `--app-clip-id` |
| `asc app-clips default-experiences list` | `--app-clip-id` |
| `asc app-clips default-experiences localizations list` | `--experience-id` |
| `asc app-clips invocations list` | `--build-bundle-id` |
| `asc app-clips list` | `--app` |
| `asc apps info view` | `--app`, `--version-id` |
| `asc build-bundles app-clip invocations list` | `--id` |
| `asc build-bundles file-sizes list` | `--id` |
| `asc product-pages custom-pages localizations list` | `--custom-page-version-id` |
| `asc product-pages custom-pages versions list` | `--custom-page-id` |
| `asc product-pages experiments list` | `--version-id`, or `--app` with `--v2` |
| `asc product-pages experiments treatments list` | `--experiment-id` |
| `asc product-pages experiments treatments localizations list` | `--treatment-id` |

With a malformed `--next` value and the parent flag, these 17 commands
reported `--next cannot be combined with --version-id` instead of the real
problem. They now report the URL, as every other list command does:

```text
$ asc iap versions list --iap-id 123456789 --next bad
Error: iap versions list: --next must be an App Store Connect URL
```

| Command | Parent flag |
| --- | --- |
| `asc iap versions images list` | `--version-id` |
| `asc iap versions links images` | `--version-id` |
| `asc iap versions links localizations` | `--version-id` |
| `asc iap versions links versions` | `--iap-id` |
| `asc iap versions list` | `--iap-id` |
| `asc iap versions localizations list` | `--version-id` |
| `asc pricing availability territory-availabilities` | `--app` |
| `asc subscriptions groups versions links localizations` | `--version-id` |
| `asc subscriptions groups versions links versions` | `--group-id` |
| `asc subscriptions groups versions list` | `--group-id` |
| `asc subscriptions groups versions localizations list` | `--version-id` |
| `asc subscriptions versions images links` | `--version-id` |
| `asc subscriptions versions images list` | `--version-id` |
| `asc subscriptions versions links` | `--subscription-id` |
| `asc subscriptions versions list` | `--subscription-id` |
| `asc subscriptions versions localizations links` | `--version-id` |
| `asc subscriptions versions localizations list` | `--version-id` |

Other visible changes that come with this:

- When the parent flag and `--next` are combined with a flag that is still
  rejected, the error now names that flag instead of the parent flag, for
  example `asc reviews list --app 1 --next URL --stars 5` reports
  `--next cannot be combined with --stars`.
- `asc subscriptions pricing price-points list`, `equalizations` and
  `adjusted-equalizations` no longer list "owner flags" in their
  `--next cannot be combined with ...` message.
- The help of `asc testflight groups builds list` and
  `asc testflight groups testers list` no longer names `--group-id` among the
  flags that cannot be combined with `--next`.

These keep their current behaviour:

- `asc subscriptions list --app` still cannot be combined with `--next`: with
  `--app` the command reads every group and prints no next URL.
- The `--type` flag of `asc builds links view`, `asc versions links`,
  `asc testflight groups links view`, `asc testflight testers links view` and
  `asc testflight pre-release links view`, and `--build-id` of
  `asc performance diagnostics list`, stay required with `--next`: they decide
  how the response is read.
- `asc localizations download` still requires `--version`: it writes files and
  names the version in its result.
- Required query filters, such as `--granularity` of the Game Center metrics
  commands, `--status` of `asc nominations list`, `--platform` of
  `asc reviews summarizations`, `--id` of `asc actors list` and
  `--build-id` of `asc testflight review submissions list`, are unchanged.

A script that relied on the usage error (exit code `2`) to detect that it had
passed the parent flag together with `--next` now gets the page instead. Pass
only `--next` if the error was the point; otherwise nothing needs to change.

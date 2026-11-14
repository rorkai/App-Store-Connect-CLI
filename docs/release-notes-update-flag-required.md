# Update commands name the flags they need when given none

Changed in 7.0.0. An update or edit command run without any of its update flags
stops before it does any work, prints one `Error:` line and its usage page, and
exits with code `2`. That line now has the same text in every command and names
each flag that counts as an update, so the next attempt does not need `--help`:

```text
$ asc devices update --id DEVICE_ID
Error: at least one update flag is required (--name, --status)
DESCRIPTION
  Update a device by ID.
...
$ echo $?
2
```

Before, 72 commands said this in about ten different ways, most without naming
any flag:

| Before | Commands |
| --- | --- |
| `Error: at least one update flag is required` | 48 |
| `Error: <command>: at least one update flag is required` | 3 |
| `Error: at least one of --a, --b, or --c is required` | 7 |
| `Error: --a, --b, or --c is required` | 8 |
| other wording (`provide at least one update flag`, `at least one update field is required (...)`, `(e.g. --a)`, `... or a matching --clear-* flag`, `at least one version field is required (...)`) | 6 |

The list contains exactly the flags that let the command proceed on their own,
in the order the command defines them. `--clear-*` switches are listed like any
other update flag. Flags that only select the resource or modify another flag,
such as `--id`, `--locale`, `--confirm`, or `--v2`, are not listed. A few listed
flags need a companion flag: for example `asc app-events update --start` also
needs `--end`, and the command says so when only one is given.
`asc localizations update` lists the flags of the localization `--type` it was
given.

The exit code, the usage page, and stdout are unchanged. A script that matched
the old line exactly should match the start of the new one,
`Error: at least one update flag is required`, which is the same in every
command.

<details>
<summary>All 72 commands</summary>

| Command | Flags now listed |
| --- | --- |
| `asc accessibility update` | `--publish`, `--supports-audio-descriptions`, `--supports-captions`, `--supports-dark-interface`, `--supports-differentiate-without-color-alone`, `--supports-larger-text`, `--supports-reduced-motion`, `--supports-sufficient-contrast`, `--supports-voice-control`, `--supports-voiceover` |
| `asc age-rating edit` | `--all-none`, `--advertising`, `--gambling`, `--health-or-wellness-topics`, `--loot-box`, `--messaging-and-chat`, `--parental-controls`, `--age-assurance`, `--social-media`, `--social-media-age-restricted`, `--unrestricted-web-access`, `--user-generated-content`, `--alcohol-tobacco-drug-use`, `--contests`, `--gambling-simulated`, `--guns-or-other-weapons`, `--medical-treatment`, `--profanity-humor`, `--sexual-content-nudity`, `--sexual-content-graphic-nudity`, `--horror-fear`, `--mature-suggestive`, `--violence-cartoon`, `--violence-realistic`, `--violence-realistic-graphic`, `--kids-age-band`, `--age-rating-override`, `--age-rating-override-v2`, `--korea-age-rating-override`, `--grac-rating-classification-number`, `--clear-grac-rating-classification-number`, `--developer-age-rating-info-url` |
| `asc android-ios-mapping update` | `--android-package-name`, `--fingerprints`, `--clear-android-package-name`, `--clear-fingerprints` |
| `asc app-clips advanced-experiences update` | `--app-clip-id`, `--action`, `--category`, `--place`, `--default-language`, `--is-powered-by`, `--removed`, `--header-image-id`, `--localization-id` |
| `asc app-clips default-experiences localizations update` | `--subtitle` |
| `asc app-clips default-experiences update` | `--action`, `--release-version-id` |
| `asc app-clips invocations localizations update` | `--title` |
| `asc app-clips invocations update` | `--url` |
| `asc app-clips review-details update` | `--url` |
| `asc app-events localizations update` | `--name`, `--short-description`, `--long-description` |
| `asc app-events update` | `--name`, `--event-type`, `--start`, `--end`, `--publish-start`, `--territories`, `--deep-link`, `--purchase-requirement`, `--primary-locale`, `--priority`, `--purpose` |
| `asc app-setup info set` | `--bundle-id`, `--primary-locale`, `--name`, `--subtitle`, `--privacy-policy-url`, `--privacy-choices-url`, `--privacy-policy-text`, `--content-rights` |
| `asc apps info edit` | `--copy-from-locale`, `--from-dir`, `--description`, `--keywords`, `--support-url`, `--marketing-url`, `--promotional-text`, `--whats-new` |
| `asc apps update` | `--bundle-id`, `--primary-locale`, `--content-rights`, `--accessibility-url`, `--subscription-status-url`, `--sandbox-subscription-status-url`, `--streamlined-purchasing-enabled` |
| `asc build-localizations update` | `--whats-new` |
| `asc builds test-notes update` | `--whats-new` |
| `asc builds update` | `--uses-non-exempt-encryption` |
| `asc bundle-ids capabilities update` | `--capability`, `--settings` |
| `asc devices update` | `--name`, `--status` |
| `asc eula update` | `--agreement-text`, `--territory` |
| `asc game-center achievements localizations update` | `--name`, `--before-earned-description`, `--after-earned-description` |
| `asc game-center achievements update` | `--reference-name`, `--points`, `--show-before-earned`, `--repeatable`, `--archived`, `--activity-properties` |
| `asc game-center achievements v2 localizations update` | `--name`, `--before-earned-description`, `--after-earned-description` |
| `asc game-center activities localizations update` | `--name`, `--description` |
| `asc game-center activities update` | `--reference-name`, `--play-style`, `--min-players`, `--max-players`, `--supports-party-code`, `--archived`, `--properties` |
| `asc game-center challenges localizations update` | `--name`, `--description` |
| `asc game-center challenges update` | `--reference-name`, `--repeatable`, `--archived`, `--leaderboard-id` |
| `asc game-center leaderboard-sets update` | `--reference-name` |
| `asc game-center leaderboard-sets v2 update` | `--reference-name` |
| `asc game-center leaderboards localizations update` | `--name`, `--formatter-override`, `--formatter-suffix`, `--formatter-suffix-singular`, `--description` |
| `asc game-center leaderboards update` | `--reference-name`, `--archived`, `--formatter`, `--sort`, `--submission-type`, `--score-range-start`, `--score-range-end`, `--recurrence-start-date`, `--recurrence-duration`, `--recurrence-rule`, `--visibility`, `--activity-properties` |
| `asc game-center leaderboards v2 localizations update` | `--name`, `--formatter-override`, `--formatter-suffix`, `--formatter-suffix-singular`, `--description` |
| `asc game-center matchmaking queues update` | `--rule-set-id`, `--experiment-rule-set-id`, `--classic-bundle-ids` |
| `asc game-center matchmaking rule-sets update` | `--min-players`, `--max-players` |
| `asc game-center matchmaking rules update` | `--description`, `--expression`, `--weight` |
| `asc game-center matchmaking teams update` | `--min-players`, `--max-players` |
| `asc iap promoted-purchases update` | `--visible-for-all-users`, `--enabled` |
| `asc iap review-screenshots update` | `--checksum`, `--uploaded`, `--file` |
| `asc iap update` | `--ref-name`, `--family-sharable`, `--review-note` |
| `asc iap versions localizations update` | `--name`, `--clear-name`, `--description`, `--clear-description` |
| `asc localizations update` | `--description`, `--keywords`, `--whats-new`, `--promotional-text`, `--clear-promotional-text`, `--support-url`, `--marketing-url` |
| `asc marketplace search-details update` | `--catalog-url` |
| `asc marketplace webhooks update` | `--url`, `--secret` |
| `asc nominations update` | `--name`, `--type`, `--description`, `--submitted`, `--archived`, `--publish-start-date`, `--publish-end-date`, `--device-families`, `--locales`, `--supplemental-materials-uris`, `--has-in-app-events`, `--launch-in-select-markets-first`, `--notes`, `--pre-order-enabled`, `--app`, `--in-app-events`, `--supported-territories` |
| `asc pre-orders update` | `--release-date`, `--pre-order-enabled`, `--available` |
| `asc product-pages custom-pages update` | `--name`, `--visible` |
| `asc product-pages experiments treatments update` | `--name`, `--app-icon-name` |
| `asc product-pages experiments update` | `--name`, `--traffic-proportion`, `--started` |
| `asc review details-update` | `--contact-first-name`, `--contact-last-name`, `--contact-email`, `--contact-phone`, `--demo-account-name`, `--demo-account-password`, `--demo-account-required`, `--notes` |
| `asc review items update` | `--resolved`, `--removed`, `--clear-resolved`, `--clear-removed` |
| `asc review items-update` | `--resolved`, `--removed`, `--clear-resolved`, `--clear-removed` |
| `asc review submissions-update` | `--platform`, `--submitted`, `--canceled`, `--clear-platform`, `--clear-submitted`, `--clear-canceled` |
| `asc sandbox update` | `--territory`, `--subscription-renewal-rate`, `--interrupt-purchases` |
| `asc subscriptions grace-periods update` | `--opt-in`, `--sandbox-opt-in`, `--duration`, `--renewal-type` |
| `asc subscriptions groups update` | `--reference-name` |
| `asc subscriptions groups versions localizations update` | `--name`, `--custom-app-name`, `--clear-name`, `--clear-custom-app-name` |
| `asc subscriptions offers introductory update` | `--end-date` |
| `asc subscriptions offers retention update` | `--eligibility-paid-months`, `--eligibility-wait-months` |
| `asc subscriptions offers win-back update` | `--eligibility-paid-months`, `--eligibility-last-subscribed-min`, `--eligibility-last-subscribed-max`, `--eligibility-wait-months`, `--start-date`, `--end-date`, `--priority`, `--promotion-intent` |
| `asc subscriptions promoted-purchases update` | `--visible-for-all-users`, `--enabled` |
| `asc subscriptions retention-messages localizations update` | `--header`, `--body`, `--clear-header`, `--clear-body` |
| `asc subscriptions review screenshots update` | `--checksum`, `--uploaded` |
| `asc subscriptions update` | `--reference-name`, `--review-note`, `--subscription-period`, `--group-level`, `--family-sharable`, `--market-settings`, `--multi-seat-status` |
| `asc subscriptions versions localizations update` | `--name`, `--description`, `--clear-name`, `--clear-description` |
| `asc testflight app-localizations update` | `--description`, `--feedback-email`, `--marketing-url`, `--privacy-policy-url`, `--tv-os-privacy-policy` |
| `asc testflight distribution edit` | `--auto-notify` |
| `asc testflight groups edit` | `--name`, `--public-link-enabled`, `--public-link-limit-enabled`, `--public-link-limit`, `--feedback-enabled`, `--ios-builds-available-for-apple-silicon-mac`, `--ios-builds-available-for-apple-vision` |
| `asc testflight review edit` | `--contact-first-name`, `--contact-last-name`, `--contact-email`, `--contact-phone`, `--demo-account-name`, `--demo-account-password`, `--demo-account-required`, `--notes` |
| `asc versions update` | `--copyright`, `--release-type`, `--review-type`, `--earliest-release-date`, `--version`, `--downloadable` |
| `asc web apps compatibility edit` | `--ios-app-on-mac`, `--ios-app-on-vision-pro` |
| `asc webhooks update` | `--name`, `--url`, `--secret`, `--events`, `--enabled` |
| `asc xcode version edit` | `--version`, `--build-number`, `--next-build-number` |

`asc localizations update --type app-info` lists `--name`, `--subtitle`,
`--clear-subtitle`, `--privacy-policy-url`, `--clear-privacy-policy-url`,
`--privacy-choices-url`, and `--privacy-policy-text`.

</details>

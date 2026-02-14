# ASC CLI Reference

AI-friendly command catalog and workflow notes for the `asc` CLI.
Use this alongside the ASC CLI README (examples) and `asc --help` (source of truth).
Generate this file in any repo with `asc init` (or `asc docs init`).

## Command Discovery (Source of Truth)

```bash
asc --help
asc <command> --help
asc <command> <subcommand> --help
```

Do not memorize flags. Always use `--help` for the current interface.

## Core Principles

- Explicit flags (prefer `--app` over short flags)
- JSON-first output (minified JSON by default)
- No interactive prompts (use `--confirm` for destructive actions)
- Pagination via `--paginate` on list commands

## Common Patterns

- IDs are App Store Connect resource IDs (use list commands to find them).
- `--app "APP_ID"` is often required (or set `ASC_APP_ID`).
- `--paginate` fetches all pages; use `--limit` and `--next` for manual pagination.
- Output formats: `--output json|table|markdown` and `--pretty` for readable JSON.
- Destructive operations require `--confirm`.
- Profiles: `--profile "NAME"` and `--strict-auth` for auth resolution safety.
- Debugging: `--debug`, `--api-debug`, `--retry-log`.
- Disable update checks: `--no-update`.

## Quick Lookup

| Task | Command |
|------|---------|
| Check auth status | `asc auth status` |
| Generate ASC.md | `asc init` |
| List apps | `asc apps` |
| List builds | `asc builds list --app "APP_ID"` |
| List TestFlight apps | `asc testflight apps list` |
| List beta groups | `asc testflight beta-groups list --app "APP_ID"` |
| Submit for review | `asc submit create --app "APP_ID" --version "VERSION" --build "BUILD_ID" --confirm` |
| Download localizations | `asc localizations download --version "VERSION_ID" --path "./localizations"` |

## Common Workflows

### Find an App ID and Recent Builds

```bash
asc apps
asc builds list --app "APP_ID" --sort -uploadedDate --limit 5
```

### Attach Build and Submit for Review

```bash
asc versions list --app "APP_ID"
asc versions attach-build --version-id "VERSION_ID" --build "BUILD_ID"
asc submit create --app "APP_ID" --version "1.0.0" --build "BUILD_ID" --confirm
```

### Distribute to TestFlight Group

```bash
asc testflight beta-groups list --app "APP_ID"
asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
```

### Migrate Metadata (Fastlane)

```bash
asc migrate validate --fastlane-dir ./metadata
asc migrate import --app "APP_ID" --fastlane-dir ./metadata
asc migrate export --app "APP_ID" --output ./exported-metadata
```

## Command Groups

Use `asc <command> --help` for subcommands and flags.

- `auth` - Manage App Store Connect API authentication.
- `install` - Install optional ASC components.
- `init` - Initialize ASC helper docs in the current repo.
- `docs` - Generate ASC CLI reference docs for a repo.
- `feedback` - List TestFlight feedback from beta testers.
- `crashes` - List and export TestFlight crash reports.
- `reviews` - List and manage App Store customer reviews.
- `review` - Manage App Store review details, attachments, and submissions.
- `analytics` - Request and download analytics and sales reports.
- `performance` - Access performance metrics and diagnostic logs.
- `finance` - Download payments and financial reports.
- `apps` - List and manage apps from App Store Connect.
- `app-clips` - Manage App Clip experiences and invocations.
- `android-ios-mapping` - Manage Android-to-iOS app mapping details.
- `app-setup` - Post-create app setup automation.
- `app-tags` - Manage app tags for App Store visibility.
- `marketplace` - Manage marketplace resources.
- `alternative-distribution` - Manage alternative distribution resources.
- `webhooks` - Manage App Store Connect webhooks.
- `nominations` - Manage featuring nominations.
- `bundle-ids` - Manage bundle IDs and capabilities.
- `merchant-ids` - Manage merchant IDs and certificates.
- `certificates` - Manage signing certificates.
- `pass-type-ids` - Manage pass type IDs.
- `profiles` - Manage provisioning profiles.
- `offer-codes` - Manage subscription offer codes.
- `win-back-offers` - Manage win-back offers for subscriptions.
- `users` - Manage App Store Connect users and invitations.
- `actors` - Lookup actors (users, API keys) by ID.
- `devices` - Manage App Store Connect devices.
- `testflight` - Manage TestFlight resources.
- `builds` - Manage builds in App Store Connect.
- `build-bundles` - Manage build bundles and App Clip data.
- `publish` - End-to-end publish workflows for TestFlight and App Store.
- `versions` - Manage App Store versions.
- `product-pages` - Manage custom product pages and product page experiments.
- `routing-coverage` - Manage routing app coverage files.
- `app-info` - Manage App Store version metadata.
- `app-infos` - List app info records for an app.
- `eula` - Manage End User License Agreements (EULA).
- `agreements` - Manage App Store Connect agreements.
- `pricing` - Manage app pricing and availability.
- `pre-orders` - Manage app pre-orders.
- `pre-release-versions` - Manage TestFlight pre-release versions.
- `localizations` - Manage App Store localization metadata.
- `screenshots` - Capture, frame, review, and upload App Store screenshots.
- `background-assets` - Manage background assets.
- `build-localizations` - Manage build release notes localizations.
- `beta-app-localizations` - Manage TestFlight beta app localizations.
- `beta-build-localizations` - Manage TestFlight beta build localizations.
- `sandbox` - Manage App Store Connect sandbox testers.
- `video-previews` - Manage App Store app preview videos.
- `signing` - Manage signing certificates and profiles.
- `notarization` - Manage macOS notarization submissions.
- `iap` - Manage in-app purchases in App Store Connect.
- `app-events` - Manage App Store in-app events.
- `subscriptions` - Manage subscription groups and subscriptions.
- `submit` - Submit builds for App Store review.
- `xcode-cloud` - Trigger and monitor Xcode Cloud workflows.
- `categories` - Manage App Store categories.
- `age-rating` - Manage App Store age rating declarations.
- `accessibility` - Manage accessibility declarations.
- `encryption` - Manage app encryption declarations and documents.
- `promoted-purchases` - Manage promoted purchases for subscriptions and in-app purchases.
- `migrate` - Migrate metadata from/to fastlane format.
- `validate` - Run pre-submission metadata and asset validation checks.
- `notify` - Send notifications to external services.
- `game-center` - Manage Game Center resources in App Store Connect.
- `version` - Print version information and exit.
- `completion` - Print shell completion scripts.

## Global Flags

- `--api-debug` - HTTP request/response logging (redacted)
- `--debug` - Debug logging
- `--no-update` - Disable update checks and auto-update
- `--profile` - Use a named authentication profile
- `--report` - Report format for CI output
- `--report-file` - Path to write CI report file
- `--retry-log` - Enable retry logging
- `--strict-auth` - Fail on mixed credential sources
- `--version` - Print version and exit

## Environment Variables (Selected)

- `ASC_APP_ID` - Default app ID
- `ASC_PROFILE` - Default auth profile
- `ASC_TIMEOUT`, `ASC_TIMEOUT_SECONDS` - Request timeout
- `ASC_UPLOAD_TIMEOUT`, `ASC_UPLOAD_TIMEOUT_SECONDS` - Upload timeout
- `ASC_DEBUG` - Debug output (`api` enables HTTP logs)
- `ASC_NO_UPDATE` - Disable update checks

## API References (Offline)

In the ASC CLI repo, see:
- `docs/openapi/latest.json`
- `docs/openapi/paths.txt`

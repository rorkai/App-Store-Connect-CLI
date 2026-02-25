# Command Reference Guide

This file is generated from live CLI help output.
For authoritative command behavior, also use:

```bash
asc --help
asc <command> --help
asc <command> <subcommand> --help
```

To regenerate:

```bash
make generate-command-docs
```

## Usage Pattern

```bash
asc <subcommand> [flags]
```

## Global Flags

- `--api-debug` - Enable HTTP debug logging to stderr (redacts sensitive values)
- `--debug` - Enable debug logging to stderr
- `--profile` - Use named authentication profile
- `--report` - Report format for CI output (e.g., junit)
- `--report-file` - Path to write CI report file
- `--retry-log` - Enable retry logging to stderr (overrides ASC_RETRY_LOG/config when set)
- `--strict-auth` - Fail when credentials are resolved from multiple sources (default: false)
- `--version` - Print version and exit (default: false)

## Command Families

### Getting Started

- `auth` - Manage authentication for the App Store Connect API.
- `doctor` - Diagnose authentication configuration issues.
- `install-skills` - Install the asc skill pack for App Store Connect workflows.
- `init` - Initialize asc helper docs in the current repo.
- `docs` - Access embedded documentation guides and reference helpers.

### Experimental Commands

- `web` - EXPERIMENTAL: Unofficial web-session workflows (discouraged).

### Analytics and Finance

- `analytics` - Request and download analytics and sales reports.
- `insights` - Generate weekly and daily insights from App Store data sources.
- `finance` - Download payments and financial reports.
- `performance` - Access performance metrics and diagnostic logs.
- `feedback` - List TestFlight feedback from beta testers.
- `crashes` - List and export TestFlight crash reports.

### App Management

- `apps` - List and manage apps in App Store Connect.
- `app-setup` - Post-create app setup automation.
- `app-tags` - Manage app tags for App Store visibility.
- `app-info` - Manage App Store version metadata.
- `app-infos` - List app info records for an app.
- `versions` - Manage App Store versions.
- `localizations` - Manage App Store localization metadata.
- `screenshots` - Capture, frame, review (experimental local workflow), and upload App Store screenshots.
- `video-previews` - Manage App Store app preview videos.
- `background-assets` - Manage background assets.
- `product-pages` - Manage custom product pages and product page experiments.
- `routing-coverage` - Manage routing app coverage files.
- `pricing` - Manage app pricing and availability.
- `pre-orders` - Manage app pre-orders.
- `categories` - Manage App Store categories.
- `age-rating` - Manage App Store age rating declarations.
- `accessibility` - Manage accessibility declarations.
- `encryption` - Manage app encryption declarations and documents.
- `eula` - Manage End User License Agreements (EULA).
- `agreements` - Manage agreements in App Store Connect.
- `app-clips` - Manage App Clip experiences and invocations.
- `android-ios-mapping` - Manage Android-to-iOS app mapping details.
- `marketplace` - Manage marketplace resources.
- `alternative-distribution` - Manage alternative distribution resources.
- `nominations` - Manage featuring nominations.
- `game-center` - Manage Game Center resources in App Store Connect.

### TestFlight and Builds

- `testflight` - Manage TestFlight resources.
- `builds` - Manage builds in App Store Connect.
- `build-bundles` - Manage build bundles and App Clip data.
- `pre-release-versions` - Manage TestFlight pre-release versions.
- `build-localizations` - Manage build release notes localizations.
- `beta-app-localizations` - Manage TestFlight beta app localizations.
- `beta-build-localizations` - Manage TestFlight beta build localizations.
- `sandbox` - Manage sandbox testers in App Store Connect.

### Review and Release

- `review` - Manage App Store review details, attachments, and submissions.
- `reviews` - List and manage App Store customer reviews.
- `submit` - Submit builds for App Store review.
- `validate` - Validate App Store version readiness before submission.
- `publish` - End-to-end publish workflows for TestFlight and App Store.

### Monetization

- `iap` - Manage in-app purchases in App Store Connect.
- `app-events` - Manage App Store in-app events.
- `subscriptions` - Manage subscription groups and subscriptions.
- `offer-codes` - Manage subscription offer codes.
- `win-back-offers` - Manage win-back offers for subscriptions.
- `promoted-purchases` - Manage promoted purchases for subscriptions and in-app purchases.

### Signing

- `signing` - Manage signing certificates and profiles.
- `bundle-ids` - Manage bundle IDs and capabilities.
- `certificates` - Manage signing certificates.
- `profiles` - Manage provisioning profiles.
- `merchant-ids` - Manage merchant IDs and certificates.
- `pass-type-ids` - Manage pass type IDs.
- `notarization` - Manage macOS notarization submissions.

### Team and Access

- `account` - Inspect account-level health and access signals.
- `users` - Manage users and invitations in App Store Connect.
- `actors` - Lookup actors (users, API keys) by ID.
- `devices` - Manage devices in App Store Connect.

### Automation

- `webhooks` - Manage webhooks in App Store Connect.
- `xcode-cloud` - Trigger and monitor Xcode Cloud workflows.
- `notify` - Send notifications to external services.
- `migrate` - Migrate metadata from/to fastlane format.

### Utility

- `version` - Print version information and exit.
- `completion` - Print shell completion scripts.

### Additional

- `diff` - Generate deterministic non-mutating diff plans.
- `status` - Show a release pipeline dashboard for an app.
- `release-notes` - Generate and manage App Store release notes.
- `workflow` - Run multi-step automation workflows.
- `metadata` - Manage app metadata with deterministic file workflows.

## Scripting Tips

- JSON output is minified by default and optimized for machine parsing.
- Use `--output table` or `--output markdown` for human-readable output.
- Use `--paginate` on list commands to fetch all pages automatically.
- Use `--limit` and `--next` for manual pagination control.
- Prefer explicit flags and deterministic outputs in CI scripts.

## High-Signal Examples

```bash
# List apps
asc apps list --output table

# Upload a build
asc builds upload --app "123456789" --file "/path/to/MyApp.ipa"

# Validate and submit an App Store version
asc validate --app "123456789" --version "1.2.3"
asc submit --app "123456789" --version "1.2.3"

# Run a local automation workflow
asc workflow run --file .asc/workflow.json --workflow release
```

## Related Documentation

- [../README.md](../README.md) - onboarding and common workflows
- [API_NOTES.md](API_NOTES.md) - API-specific behavior and caveats
- [TESTING.md](TESTING.md) - test strategy and patterns
- [CONTRIBUTING.md](CONTRIBUTING.md) - contribution and dev workflow

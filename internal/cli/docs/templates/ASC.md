# asc cli reference

Unofficial CLI for the App Store Connect API. AI-friendly command catalog and
workflow notes for the asc cli. Use this alongside the asc cli readme
(examples) and `asc --help` (source of truth). Generate this file in any repo
with `asc init` (or `asc docs init`).

## Command Discovery (Source of Truth)

```bash
asc --help
asc <command> --help
asc <command> <subcommand> --help
```

Do not memorize flags. Always use `--help` for the current interface.

## Core Principles

- Explicit flags (prefer `--app` over short flags)
- TTY-aware output defaults (`table` in terminals, `json` when piped/non-interactive)
- No interactive prompts (use `--confirm` for destructive actions)
- Pagination via `--paginate` on list commands

## Common Patterns

- IDs are App Store Connect API resource IDs (use list commands to find them).
- `--app "APP_ID"` is often required (or set `ASC_APP_ID`).
- `--paginate` fetches all pages; use `--limit` and `--next` for manual pagination.
- Output formats: `--output json|table|markdown` and `--pretty` for readable JSON.
- `ASC_DEFAULT_OUTPUT` can pin the default output mode across contexts.
- Destructive operations require `--confirm`.
- Profiles: `--profile "NAME"` and `--strict-auth` for auth resolution safety.
- Debugging: `--debug`, `--api-debug`, `--retry-log`.

## Quick Lookup

| Task | Command |
|------|---------|
| Check auth status | `asc auth status` |
| Run auth doctor | `asc doctor --output json` |
| Check account health | `asc account status` |
| Generate ASC.md | `asc init` |
| Create an app (unofficial web flow) | `asc web apps create --name "My App" --bundle-id "com.example.app" --sku "SKU123"` |
| List apps | `asc apps` |
| List builds | `asc builds list --app "APP_ID"` |
| List TestFlight groups | `asc testflight groups list --app "APP_ID"` |
| List internal TestFlight groups | `asc testflight groups list --app "APP_ID" --internal` |
| Stage a release (pre-submit) | `asc release stage --app "APP_ID" --version "VERSION" --build "BUILD_ID" --copy-metadata-from "PREVIOUS_VERSION" --dry-run` |
| Release (full pipeline) | `asc release run --app "APP_ID" --version "VERSION" --build "BUILD_ID" --metadata-dir "./metadata/version/VERSION" --dry-run` |
| Submit for review (low-level) | `asc submit create --app "APP_ID" --version "VERSION" --build "BUILD_ID" --confirm` |
| Weekly insights summary | `asc insights weekly --app "APP_ID" --source analytics --week "YYYY-MM-DD"` |
| Download localizations | `asc localizations download --version "VERSION_ID" --path "./localizations"` |

## Common Workflows

### Find an App ID and Recent Builds

```bash
asc apps
asc builds list --app "APP_ID" --sort -uploadedDate --limit 5
```

### Stage for Review (high-level: ensure version + copy/apply metadata + attach + validate)

```bash
# Dry-run the staging plan using metadata carry-forward
asc release stage --app "APP_ID" --version "1.0.0" --build "BUILD_ID" --copy-metadata-from "0.9.0" --dry-run

# Stage the version without submitting it for review yet
asc release stage --app "APP_ID" --version "1.0.0" --build "BUILD_ID" --copy-metadata-from "0.9.0" --confirm
```

### Release (high-level: ensure version + apply metadata + attach + validate + submit)

```bash
# Dry-run first to preview all planned steps
asc release run --app "APP_ID" --version "1.0.0" --build "BUILD_ID" --metadata-dir "./metadata/version/1.0.0" --dry-run

# Run the full pipeline
asc release run --app "APP_ID" --version "1.0.0" --build "BUILD_ID" --metadata-dir "./metadata/version/1.0.0" --confirm

# Monitor status after submission
asc status --app "APP_ID"
```

Lower-level alternatives for scripting or partial workflows:

```bash
asc versions list --app "APP_ID"
asc versions attach-build --version-id "VERSION_ID" --build "BUILD_ID"
asc validate --app "APP_ID" --version "1.0.0"
asc submit create --app "APP_ID" --version "1.0.0" --build "BUILD_ID" --confirm
```

### Distribute to TestFlight Group

```bash
asc testflight groups list --app "APP_ID"
asc testflight groups list --app "APP_ID" --internal
asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
asc builds add-groups --build "BUILD_ID" --group "GROUP_ID" --submit --confirm
```

### Migrate Metadata (Fastlane)

```bash
asc migrate validate --fastlane-dir ./metadata
asc migrate import --app "APP_ID" --fastlane-dir ./metadata
asc migrate export --app "APP_ID" --output ./exported-metadata
```

## Command Groups

Use `asc <command> --help` for subcommands and flags.

- `auth` - Manage authentication for the App Store Connect API.
- `doctor` - Diagnose authentication configuration issues.
- `web` - `[experimental]` Unofficial Apple web-session `/iris` workflows (discouraged; not part of the official API). Uses low-rate calls, user-owned Apple ID sessions, and signed-URL redaction by default. Use `asc web apps create` as the canonical app-creation path in this family.
- `account` - Inspect account-level health and access signals.
- `install-skills` - Install the asc skill pack for App Store Connect workflows.
- `init` - Initialize asc helper docs in the current repo.
- `docs` - Generate asc cli reference docs for a repo.
- `diff` - Generate deterministic non-mutating diff plans.
- `status` - Show a release pipeline dashboard for an app.
- `insights` - Generate weekly insights from App Store data sources.
- `release-notes` - Generate and manage App Store release notes.
- `reviews` - List and manage App Store customer reviews.
- `review` - Manage App Store review details, attachments, and submissions.
- `analytics` - Request and download analytics and sales reports.
- `performance` - Access performance metrics and diagnostic logs.
- `finance` - Download payments and financial reports.
- `apps` - List and manage apps in App Store Connect. `asc apps create` is a deprecated compatibility shim; use `asc web apps create` for new app creation.
- `app-clips` - Manage App Clip experiences and invocations.
- `android-ios-mapping` - Manage Android-to-iOS app mapping details.
- `app-setup` - Post-create app setup automation.
- `app-tags` - Manage app tags for App Store visibility.
- `marketplace` - Manage marketplace resources.
- `alternative-distribution` - Manage alternative distribution resources.
- `webhooks` - Manage webhooks in App Store Connect.
- `nominations` - Manage featuring nominations.
- `bundle-ids` - Manage bundle IDs and capabilities.
- `merchant-ids` - Manage merchant IDs and certificates.
- `certificates` - Manage signing certificates.
- `pass-type-ids` - Manage pass type IDs.
- `profiles` - Manage provisioning profiles.
- `users` - Manage users and invitations in App Store Connect.
- `actors` - Lookup actors (users, API keys) by ID.
- `devices` - Manage devices in App Store Connect.
- `testflight` - Manage TestFlight workflows.
- `builds` - Manage builds (TestFlight/App Store).
- `build-bundles` - Manage build bundles and App Clip data.
- `publish` - End-to-end publish workflows for TestFlight and App Store.
- `release` - Run high-level App Store release workflows.
- `workflow` - Run multi-step automation workflows.
- `xcode` - Produce deterministic `.xcarchive` and `.ipa` artifacts with local Xcode build/export helpers (macOS only).
- `versions` - Manage App Store versions.
- `product-pages` - Manage custom product pages and product page experiments.
- `routing-coverage` - Manage routing app coverage files.
- `eula` - Manage End User License Agreements (EULA).
- `agreements` - Manage agreements in App Store Connect.
- `pricing` - Manage app pricing and availability.
- `pre-orders` - Manage app pre-orders.
- `localizations` - Manage App Store localization metadata.
- `metadata` - Pull, validate, push, and keyword-sync canonical metadata workflows.
- `screenshots` - Upload and manage App Store screenshots; local capture/frame workflow is `[experimental]`.
- `background-assets` - Manage background assets.
- `build-localizations` - Manage build release notes localizations.
- `sandbox` - Manage sandbox testers in App Store Connect.
- `video-previews` - Manage App Store app preview videos.
- `signing` - Manage signing certificates and profiles.
- `notarization` - Manage macOS notarization submissions.
- `iap` - Manage in-app purchases.
- `app-events` - Manage App Store in-app events.
- `subscriptions` - Manage subscription groups and subscriptions.
- `submit` - Submit builds for App Store review.
- `xcode-cloud` - Trigger and monitor Xcode Cloud workflows.
- `categories` - Manage App Store categories.
- `age-rating` - Manage App Store age rating declarations.
- `accessibility` - Manage accessibility declarations.
- `encryption` - Manage app encryption declarations and documents.
- `migrate` - Migrate metadata from/to fastlane format.
- `validate` - Run pre-submission metadata and asset validation checks.
- `notify` - Send notifications to external services.
- `game-center` - Manage Game Center resources.
- `version` - Print version information and exit.
- `completion` - Print shell completion scripts.
- `schema` - Inspect App Store Connect API endpoint schemas at runtime.
- `snitch` - Report CLI friction as a GitHub issue.

## Global Flags

- `--api-debug` - HTTP request/response logging (redacted)
- `--debug` - Debug logging
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
- Web password environment variable (`ASC_WEB` + `_PASSWORD`) - Password source for `asc web auth login` and `asc web apps create`
- `ASC_WEB_SESSION_CACHE`, `ASC_WEB_SESSION_CACHE_DIR`, `ASC_WEB_SESSION_CACHE_BACKEND` - Web-session cache controls for unofficial web flows
- `ASC_IRIS_SESSION_CACHE`, `ASC_IRIS_SESSION_CACHE_DIR` - Deprecated legacy app-create cache settings; imported into the web session cache during the transition window
- `ASC_SPINNER_DISABLED` - Disable interactive stderr spinner
- `ASC_SKILLS_AUTO_CHECK` - Automatic skills update checks (`true`/`1`/`yes`/`y`/`on` enables, `false`/`0`/`no`/`n`/`off` disables; default enabled)

## API References (Offline)

In the asc cli repo, see:
- `docs/openapi/latest.json`
- `docs/openapi/paths.txt`

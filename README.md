# Unofficial App Store Connect CLI

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/badge/Homebrew-compatible-blue?style=for-the-badge" alt="Homebrew">
</p>

A **fast**, **lightweight**, and **scriptable** CLI for App Store Connect. Automate your iOS app workflows from your IDE/terminal.

## Why ASC?

| Problem | Solution |
|---------|----------|
| Manual App Store Connect work | Automate everything from CLI |
| Slow, heavy tooling | Single Go binary, instant startup |
| Poor scripting support | JSON output, explicit flags, clean exit codes |

<!-- WALL-OF-APPS:START -->
## Wall of Apps

Apps shipping with asc-cli. [Add yours via PR](https://github.com/rudrankriyam/App-Store-Connect-CLI/pulls)!

| App | Link | Creator | Platform |
|:----|:-----|:--------|:---------|
| CodexMonitor | [Open](https://github.com/Dimillian/CodexMonitor) | Dimillian | macOS, iOS |
| DoubleMemory | [Open](https://doublememory.com) | Shaomeng Zhang | iOS |
| Fisherman SMS Filtering | [Open](https://apps.apple.com/app/id6449192504) | MGidnian | iOS |
| kora: Music Reviews & Ratings | [Open](https://apps.apple.com/app/id6502549140) | adamjhf | iOS |
| MileIO | [Open](https://apps.apple.com/app/id6758225631) | Juergen | iOS |
| Repetti | [Open](https://apps.apple.com/us/app/repetti-the-chores-list-app/id6758055413) | rursache | iOS |
| TV Show Tracker | [Open](https://apps.apple.com/us/app/tv-show-tracker-tv-club/id6497563903) | rursache | iOS |
| Unlimited Clipboard History | [Open](https://apps.apple.com/us/app/unlimited-clipboard-history/id6705136056) | yspreen | macOS |
<!-- WALL-OF-APPS:END -->

## ASC Skills

Agent Skills for automating `asc` workflows including builds, TestFlight, metadata sync, submissions, and signing. https://github.com/rudrankriyam/app-store-connect-cli-skills

## Table of Contents

- [Why ASC?](#why-asc)
- [ASC Skills](#asc-skills)
- [Quick Start](#quick-start)
  - [Install](#install)
  - [Authenticate](#authenticate)
- [Commands](#commands)
  - [Scripting Tips](#scripting-tips)
  - [TestFlight](#testflight)
  - [Beta Groups](#beta-groups)
  - [Beta Testers](#beta-testers)
  - [Devices](#devices)
  - [App Store](#app-store)
  - [App Tags](#app-tags)
  - [App Events](#app-events)
  - [Alternative Distribution](#alternative-distribution)
  - [Analytics & Sales](#analytics--sales)
  - [Finance Reports](#finance-reports)
  - [Sandbox Testers](#sandbox-testers)
  - [Xcode Cloud](#xcode-cloud)
  - [Notarization](#notarization)
  - [Game Center](#game-center)
  - [Signing](#signing)
  - [Certificates](#certificates)
  - [Profiles](#profiles)
  - [Bundle IDs](#bundle-ids)
  - [Subscriptions](#subscriptions)
  - [In-App Purchases](#in-app-purchases)
  - [Performance](#performance)
  - [Webhooks](#webhooks)
  - [Publish (End-to-End Workflows)](#publish-end-to-end-workflows)
  - [App Clips](#app-clips)
  - [Encryption](#encryption)
  - [Screenshots & Video Previews](#screenshots--video-previews)
  - [Background Assets](#background-assets)
  - [Routing Coverage](#routing-coverage)
  - [Notify](#notify)
  - [Apps & Builds](#apps--builds)
- [App Setup](#app-setup)
  - [Categories](#categories)
  - [Offer Codes (Subscriptions)](#offer-codes-subscriptions)
  - [Versions](#versions)
  - [App Info](#app-info)
  - [Pre-Release Versions](#pre-release-versions)
  - [Localizations](#localizations)
  - [Build Localizations](#build-localizations)
  - [Migrate (Fastlane Compatibility)](#migrate-fastlane-compatibility)
  - [Validate (Pre-Submission)](#validate-pre-submission)
  - [Submit](#submit)
  - [Utilities](#utilities)
  - [Output Formats](#output-formats)
  - [Authentication](#authentication)
- [Design Philosophy](#design-philosophy)
  - [Explicit Over Cryptic](#explicit-over-cryptic)
  - [JSON-First Output](#json-first-output)
  - [No Interactive Prompts](#no-interactive-prompts)
- [Installation](#installation)
- [Documentation](#documentation)
- [How to test in <10 minutes](#how-to-test-in-10-minutes)
- [Security](#security)
- [Contributing](#contributing)
- [License](#license)
- [Author](#author)
- [Star History](#star-history)

## Quick Start

### Install

```bash
# Via Homebrew (recommended)
brew tap rudrankriyam/tap
brew install rudrankriyam/tap/asc

# Install script (macOS/Linux)
curl -fsSL https://raw.githubusercontent.com/rudrankriyam/App-Store-Connect-CLI/main/install.sh | bash

# Installs to ~/.local/bin by default (ensure it's on your PATH)

# Or build from source
git clone https://github.com/rudrankriyam/App-Store-Connect-CLI.git
cd App-Store-Connect-CLI
make build
./asc --help
```

### Updates

`asc` checks for updates on startup and auto-updates when installed via the GitHub release install script. Homebrew installs will show a `brew upgrade` hint instead. Disable update checks with `--no-update` or `ASC_NO_UPDATE=1`.

### Authenticate

```bash
# Register your App Store Connect API key
asc auth login \
  --name "MyApp" \
  --key-id "ABC123" \
  --issuer-id "DEF456" \
  --private-key /path/to/AuthKey.p8

# Validate credentials via network during login
asc auth login \
  --network \
  --name "MyApp" \
  --key-id "ABC123" \
  --issuer-id "DEF456" \
  --private-key /path/to/AuthKey.p8

# Skip JWT + network validation (useful in CI)
asc auth login \
  --skip-validation \
  --name "MyApp" \
  --key-id "ABC123" \
  --issuer-id "DEF456" \
  --private-key /path/to/AuthKey.p8

# Add another profile and switch defaults
asc auth login \
  --name "ClientApp" \
  --key-id "XYZ789" \
  --issuer-id "LMN000" \
  --private-key /path/to/ClientAuthKey.p8

asc auth switch --name "ClientApp"

# Use a profile for a single command
asc --profile "ClientApp" apps list

# Fail if credentials resolve from mixed sources
asc --strict-auth apps list

# Create a template config.json (global, no secrets)
asc auth init

# Create a repo-local config.json
asc auth init --local

# Store credentials in global config.json (bypass keychain)
asc auth login \
  --bypass-keychain \
  --name "MyApp" \
  --key-id "ABC123" \
  --issuer-id "DEF456" \
  --private-key /path/to/AuthKey.p8

# Store credentials in repo-local config.json
asc auth login \
  --bypass-keychain \
  --local \
  --name "MyApp" \
  --key-id "ABC123" \
  --issuer-id "DEF456" \
  --private-key /path/to/AuthKey.p8
```

Generate API keys at: https://appstoreconnect.apple.com/access/integrations/api

Open the API keys page in your browser:
```bash
asc auth init --open
```

Credentials are stored in the system keychain when available, with a config fallback
at `~/.asc/config.json` (restricted permissions). A repo-local `./.asc/config.json`
takes precedence when present. Override with `ASC_CONFIG_PATH`. When
`ASC_BYPASS_KEYCHAIN` is set to a truthy value (`1`, `true`, `yes`, `on`) and
environment credentials are fully provided, the environment values take
precedence over config.
Environment variable fallback:
- `ASC_KEY_ID`
- `ASC_ISSUER_ID`
- `ASC_PRIVATE_KEY_PATH`
- `ASC_PRIVATE_KEY` (raw key content; CLI writes a temp key file)
- `ASC_PRIVATE_KEY_B64` (base64 key content; CLI writes a temp key file)
- `ASC_CONFIG_PATH` (absolute path to config.json)
- `ASC_PROFILE`
- `ASC_BYPASS_KEYCHAIN` (ignore keychain and use config/env auth; truthy values: `1`, `true`, `yes`, `on`; falsey values: `0`, `false`, `no`, `off`)
- `ASC_STRICT_AUTH` (fail when credentials resolve from multiple sources)
- `ASC_STRICT_AUTH` (fail when credentials resolve from multiple sources; accepts `true/false`, `1/0`, `yes/no`, `y/n`, `on/off`)

Use `--strict-auth` or `ASC_STRICT_AUTH=true` (also `1`, `yes`, `y`, `on`) to fail when credentials are resolved from multiple sources.

App ID fallback:
- `ASC_APP_ID`

Analytics & sales env:
- `ASC_VENDOR_NUMBER` (Sales, Trends, and Finance reports)
- `ASC_ANALYTICS_VENDOR_NUMBER` (fallback for analytics vendor number)
- `ASC_TIMEOUT` (e.g., `90s`, `2m`)
- `ASC_TIMEOUT_SECONDS` (e.g., `120`)
- `ASC_UPLOAD_TIMEOUT` (e.g., `60s`, `2m`)
- `ASC_UPLOAD_TIMEOUT_SECONDS` (e.g., `120`)

Retry behavior env:
- `ASC_MAX_RETRIES` (default: 3) for GET/HEAD requests
- `ASC_BASE_DELAY` (default: `1s`)
- `ASC_MAX_DELAY` (default: `30s`)
- `ASC_RETRY_LOG=1` to log retries to stderr
- Retry errors include `retry after` in the final error message when available

Output format:
- `ASC_DEFAULT_OUTPUT` sets the default `--output` format (`json`, `table`, `markdown`, or `md`)
- Explicit `--output` flags always override the environment variable

Debug logging:
- `ASC_DEBUG=1` to enable debug output
- `ASC_DEBUG=api` to include HTTP request/response details (redacted)
- Use `--debug` for per-command debug output
- Use `--api-debug` for per-command HTTP debug output (redacted)

Config.json keys (same semantics, snake_case):
- `app_id`
- `vendor_number`
- `analytics_vendor_number`
- `timeout`, `timeout_seconds`
- `upload_timeout`, `upload_timeout_seconds`
- `max_retries`
- `base_delay`
- `max_delay`
- `retry_log` (set to `1` or `true` to enable)
- `debug` (set to `1` for debug output or `api` for HTTP details)

## Commands

### Scripting Tips

- JSON output is default for easy parsing; add `--pretty` when debugging.
- Use `--paginate` to automatically fetch all pages.
- `--paginate` works on list commands including apps, builds list, builds uploads list, app-tags list, app-tags territories, offer-codes list, devices list, feedback, crashes, reviews, versions list, pre-release versions list, localizations list, build-localizations list, beta-groups list, beta-testers list, sandbox list, analytics requests/get, testflight apps list, game-center achievements/leaderboards/leaderboard-sets lists (including localizations/releases/members), Xcode Cloud workflows/build-runs, certificates list, profiles list, bundle-ids list, subscriptions groups/list, iap list, webhooks list, app-clips list, encryption declarations list, background-assets list, and performance diagnostics list.
- Use `--limit` + `--next "<links.next>"` for manual pagination control.
- Sort with `--sort` (prefix `-` for descending):
  - Feedback/Crashes: `createdDate` / `-createdDate`
  - Reviews: `rating` / `-rating`, `createdDate` / `-createdDate`
  - Apps: `name` / `-name`, `bundleId` / `-bundleId`
  - Builds: `uploadedDate` / `-uploadedDate`

### TestFlight

```bash
# List beta feedback (JSON output)
asc feedback --app "123456789"

# Filter feedback by device model and OS version
asc feedback --app "123456789" --device-model "iPhone15,3" --os-version "17.2"

# Filter feedback by platform/build/tester
asc feedback --app "123456789" --app-platform IOS --device-platform IOS --build "BUILD_ID" --tester "TESTER_ID"

# Fetch all feedback pages automatically
asc feedback --app "123456789" --paginate

# Get crash reports (table format - for humans)
asc crashes --app "123456789" --output table

# Get crash reports (markdown - for docs)
asc crashes --app "123456789" --output markdown

# Limit results per page (pagination)
asc crashes --app "123456789" --limit 25

# Sort crashes by created date (newest first)
asc crashes --app "123456789" --sort -createdDate --limit 5

# Fetch all crash pages automatically
asc crashes --app "123456789" --paginate

# List TestFlight apps
asc testflight apps list

# Fetch a TestFlight app by ID
asc testflight apps get --app "APP_ID"

# Export TestFlight configuration to YAML
asc testflight sync pull --app "APP_ID" --output "./testflight.yaml"
asc testflight sync pull --app "APP_ID" --output "./testflight.yaml" --include-builds --include-testers

# TestFlight review and submission
asc testflight review get --app "APP_ID"
asc testflight review submit --build "BUILD_ID" --confirm

# Beta details
asc testflight beta-details get --build "BUILD_ID"
asc testflight beta-details update --id "DETAIL_ID" --auto-notify

# Beta license agreements
asc testflight beta-license-agreements list --app "APP_ID"
asc testflight beta-license-agreements update --id "AGREEMENT_ID" --agreement-text "New terms..."

# Send beta notification for a build
asc testflight beta-notifications create --build "BUILD_ID"

# TestFlight metrics
asc testflight metrics public-link --group "GROUP_ID"
asc testflight metrics beta-tester-usages --app "APP_ID"
```

### Beta Groups

```bash
# List beta groups for an app
asc testflight beta-groups list --app "APP_ID"

# Fetch all beta groups (all pages)
asc testflight beta-groups list --app "APP_ID" --paginate

# Create, fetch, update, delete
asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"
asc testflight beta-groups get --id "GROUP_ID"
asc testflight beta-groups update --id "GROUP_ID" --name "New Name"
asc testflight beta-groups update --id "GROUP_ID" --public-link-enabled true --feedback-enabled true
asc testflight beta-groups delete --id "GROUP_ID" --confirm

# Add/remove testers
asc testflight beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID"
asc testflight beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID"

# View linked app and recruitment criteria
asc testflight beta-groups app get --group-id "GROUP_ID"
```

### Beta Testers

```bash
# List beta testers
asc testflight beta-testers list --app "APP_ID"

# Filter by build or group
asc testflight beta-testers list --app "APP_ID" --build "BUILD_ID"
asc testflight beta-testers list --app "APP_ID" --group "Beta"

# Fetch all beta testers (all pages)
asc testflight beta-testers list --app "APP_ID" --paginate

# Get, add, remove, invite
asc testflight beta-testers get --id "TESTER_ID"
asc testflight beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta"
asc testflight beta-testers remove --app "APP_ID" --email "tester@example.com"
asc testflight beta-testers invite --app "APP_ID" --email "tester@example.com"

# Manage group membership
asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID"
asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID"

# Manage build access
asc testflight beta-testers add-builds --id "TESTER_ID" --build "BUILD_ID"
asc testflight beta-testers remove-builds --id "TESTER_ID" --build "BUILD_ID" --confirm

# Tester metrics
asc testflight beta-testers metrics --tester-id "TESTER_ID" --app "APP_ID"
```

### Devices

```bash
# List devices
asc devices list

# Filter by platform/status/UDID
asc devices list --platform IOS --status ENABLED --udid "UDID1,UDID2"

# Filter by name or ID
asc devices list --name "My iPhone"
asc devices list --id "DEVICE_ID1,DEVICE_ID2"

# Sort and select fields
asc devices list --sort name --fields "name,platform,udid"

# Fetch all devices (all pages)
asc devices list --paginate

# Get a device by ID
asc devices get --id "DEVICE_ID"

# Register a device
asc devices register --name "My iPhone" --udid "UDID" --platform IOS

# Register using the local macOS hardware UDID
asc devices register --name "My Mac" --udid-from-system --platform MAC_OS

# Update device name/status
asc devices update --id "DEVICE_ID" --name "New Name"
asc devices update --id "DEVICE_ID" --status DISABLED

# Get local macOS hardware UDID
asc devices local-udid
```

### App Store

```bash
# List customer reviews (JSON output)
asc reviews --app "123456789"

# Filter by stars (table format - for humans)
asc reviews --app "123456789" --stars 1 --output table

# Filter by territory (markdown - for docs)
asc reviews --app "123456789" --territory US --output markdown

# Sort reviews by created date (newest first)
asc reviews --app "123456789" --sort -createdDate --limit 5

# Fetch all reviews pages automatically
asc reviews --app "123456789" --paginate

# Get a specific review by ID
asc reviews get --id "REVIEW_ID"

# Get review ratings summary
asc reviews ratings --app "123456789"

# Get review summarizations
asc reviews summarizations --app "123456789" --platform IOS --territory USA

# Respond to a customer review
asc reviews respond --review-id "REVIEW_ID" --response "Thanks for your feedback!"

# Get a review response by ID
asc reviews response get --id "RESPONSE_ID"

# Get the response for a specific review
asc reviews response for-review --review-id "REVIEW_ID"

# Delete a review response
asc reviews response delete --id "RESPONSE_ID" --confirm
```

### App Tags

```bash
# List app tags for an app
asc app-tags list --app "APP_ID"

# Filter, sort, and request specific fields
asc app-tags list --app "APP_ID" --visible-in-app-store true --sort -name --fields "name,visibleInAppStore"

# Include territories (requires explicit include)
asc app-tags list --app "APP_ID" --include territories --territory-fields currency --territory-limit 50

# Fetch all tag pages
asc app-tags list --app "APP_ID" --paginate

# Get a tag by ID
asc app-tags get --app "APP_ID" --id "TAG_ID"

# Update tag visibility (requires confirm)
asc app-tags update --id "TAG_ID" --visible-in-app-store=false --confirm

# List territories attached to a tag
asc app-tags territories --id "TAG_ID" --fields currency

# List territory relationships for a tag
asc app-tags territories-relationships --id "TAG_ID"

# List tag relationships for an app
asc app-tags relationships --app "APP_ID"
```

### App Events

```bash
# List in-app events for an app
asc app-events list --app "APP_ID"

# List localizations for an event
asc app-events localizations list --event-id "EVENT_ID"

# List localization screenshots and video clips
asc app-events localizations screenshots list --localization-id "LOC_ID"
asc app-events localizations video-clips list --localization-id "LOC_ID"

# List localization relationships for an event
asc app-events relationships --event-id "EVENT_ID"

# List localization media relationships
asc app-events localizations screenshots-relationships --localization-id "LOC_ID"
asc app-events localizations video-clips-relationships --localization-id "LOC_ID"
```

### Alternative Distribution

```bash
# List domains and keys
asc alternative-distribution domains list
asc alternative-distribution keys list

# Create and delete a domain
asc alternative-distribution domains create --domain "example.com" --reference-name "Example"
asc alternative-distribution domains delete --domain-id "DOMAIN_ID" --confirm

# Create a key and fetch the key for an app
asc alternative-distribution keys create --app "APP_ID" --public-key-path "./key.pem"
asc alternative-distribution keys app --app "APP_ID"

# Create and fetch packages and versions
asc alternative-distribution packages create --app-store-version-id "APP_STORE_VERSION_ID"
asc alternative-distribution packages get --package-id "PACKAGE_ID"
asc alternative-distribution packages versions list --package-id "PACKAGE_ID"
asc alternative-distribution packages versions get --version-id "VERSION_ID"
asc alternative-distribution packages versions deltas --version-id "VERSION_ID"
asc alternative-distribution packages versions variants --version-id "VERSION_ID"
```

### Analytics & Sales

```bash
# Download daily sales summary (writes .tsv.gz)
asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"

# Download and decompress
asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20" --decompress

# Create analytics report request
asc analytics request --app "123456789" --access-type ONGOING

# List analytics report requests (all pages)
asc analytics requests --app "123456789" --paginate

# Get analytics reports with instances
asc analytics get --request-id "REQUEST_ID"

# Get analytics report instances for a specific date
asc analytics get --request-id "REQUEST_ID" --date "2024-01-20"

# Include report segments in the output
asc analytics get --request-id "REQUEST_ID" --include-segments

# Fetch a specific instance and include segments
asc analytics get --request-id "REQUEST_ID" --instance-id "INSTANCE_ID" --include-segments

# Download analytics report data
asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID"
```

Notes:
- Sales report date formats: DAILY/WEEKLY `YYYY-MM-DD`, MONTHLY `YYYY-MM`, YEARLY `YYYY`
- Reports may not be available yet; ASC returns availability errors when data is pending
- Use `ASC_TIMEOUT` or `ASC_TIMEOUT_SECONDS` for long analytics pagination
- `asc analytics get --date ... --paginate` will scan all report pages (slower, but avoids missing instances)

### Finance Reports

```bash
# Download consolidated report (all regions in one file)
asc finance reports --vendor "12345678" --report-type FINANCIAL --region "ZZ" --date "2025-12"

# Download US-only monthly report
asc finance reports --vendor "12345678" --report-type FINANCIAL --region "US" --date "2025-12"

# Download detailed report (transaction-level data) and decompress
asc finance reports --vendor "12345678" --report-type FINANCE_DETAIL --region "Z1" --date "2025-12" --decompress

# List finance report region codes and currencies
asc finance regions --output table
```

**Report Types (API to UI mapping):**

| API Report Type  | UI Option                              | Region Code(s)          |
|------------------|----------------------------------------|-------------------------|
| `FINANCIAL`      | All Countries or Regions (Single File) | `ZZ` (consolidated)     |
| `FINANCIAL`      | All Countries or Regions (Multiple Files) | `US`, `EU`, `JP`, etc. |
| `FINANCE_DETAIL` | All Countries or Regions (Detailed)    | `Z1` (required)         |
| Not available    | Transaction Tax (Single File)          | N/A - manual download   |

**Notes:**
- Report date format: `YYYY-MM` (Apple fiscal month)
- Reports typically appear the first Friday of the following fiscal month
- `FINANCE_DETAIL` requires region code `Z1` (the only valid region for detailed reports)
- Transaction Tax reports are not available via API - download manually from App Store Connect
- Use `asc finance regions` to list all valid region codes and currencies
- Requires Account Holder, Admin, or Finance role

**Region codes reference:** https://developer.apple.com/help/app-store-connect/reference/financial-report-regions-and-currencies/

### Sandbox Testers

```bash
# List sandbox testers
asc sandbox list

# Filter by email or territory
asc sandbox list --email "tester@example.com"
asc sandbox list --territory "USA"

# Fetch all sandbox testers (all pages)
asc sandbox list --paginate

# Get sandbox tester details
asc sandbox get --id "SANDBOX_TESTER_ID"
asc sandbox get --email "tester@example.com"

# Update a sandbox tester
asc sandbox update --id "SANDBOX_TESTER_ID" --territory "USA"
asc sandbox update --email "tester@example.com" --interrupt-purchases
asc sandbox update --id "SANDBOX_TESTER_ID" --subscription-renewal-rate "MONTHLY_RENEWAL_EVERY_ONE_HOUR"

# Clear purchase history
asc sandbox clear-history --id "SANDBOX_TESTER_ID" --confirm
asc sandbox clear-history --email "tester@example.com" --confirm
```

Notes:
- Territory uses 3-letter App Store territory codes (e.g., `USA`, `JPN`)
- Sandbox list/get/update/clear-history use the v2 API

### Xcode Cloud

```bash
# List workflows for an app (find workflow IDs)
asc xcode-cloud workflows --app "123456789"

# List build runs for a workflow (find run IDs)
asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID"

# Fetch all workflows/build runs (all pages)
asc xcode-cloud workflows --app "123456789" --paginate
asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID" --paginate

# Trigger a workflow by name (requires --app)
asc xcode-cloud run --app "123456789" --workflow "CI Build" --branch "main"

# Trigger a workflow by ID (no app needed)
asc xcode-cloud run --workflow-id "WORKFLOW_ID" --git-reference-id "REF_ID"

# Trigger and wait for completion
asc xcode-cloud run --app "123456789" --workflow "Deploy" --branch "release/1.0" --wait

# Trigger with custom polling interval and timeout
asc xcode-cloud run --app "123456789" --workflow "CI" --branch "main" --wait --poll-interval 30s --timeout 1h

# Check build run status
asc xcode-cloud status --run-id "BUILD_RUN_ID"

# Check status with table output
asc xcode-cloud status --run-id "BUILD_RUN_ID" --output table

# Wait for an existing build run to complete
asc xcode-cloud status --run-id "BUILD_RUN_ID" --wait

# CI Products
asc xcode-cloud products --app "APP_ID"

# Source code management
asc xcode-cloud scm providers list
asc xcode-cloud scm providers get --provider-id "PROVIDER_ID"
asc xcode-cloud scm providers repositories --provider-id "PROVIDER_ID"
asc xcode-cloud scm repositories list
asc xcode-cloud scm repositories get --id "REPO_ID"
asc xcode-cloud scm repositories git-references --repo-id "REPO_ID"
asc xcode-cloud scm git-references get --id "REF_ID"

# Build actions and artifacts
asc xcode-cloud actions --run-id "BUILD_RUN_ID"
asc xcode-cloud artifacts list --action-id "ACTION_ID"
asc xcode-cloud artifacts get --id "ARTIFACT_ID"
asc xcode-cloud artifacts download --id "ARTIFACT_ID" --path "./artifact.zip"

# Test results and issues
asc xcode-cloud test-results list --action-id "ACTION_ID"
asc xcode-cloud test-results get --id "RESULT_ID"
asc xcode-cloud issues list --action-id "ACTION_ID"

# Available macOS and Xcode versions
asc xcode-cloud macos-versions
asc xcode-cloud xcode-versions
```

Notes:
- Workflows must have manual start conditions enabled to be triggered via API
- Use `--workflow` with `--app` for human-friendly workflow lookup by name
- Use `--workflow-id` and `--git-reference-id` for direct ID-based triggering
- Use `asc xcode-cloud workflows` and `asc xcode-cloud build-runs` to discover IDs
- When using `--wait`, the command polls until the build completes (or times out)
- Exit code is non-zero if the build fails, errors, or is canceled
- Use `ASC_TIMEOUT` env var or `--timeout` flag for long-running builds

### Notarization

```bash
# Submit a file for macOS notarization
asc notarization submit --file ./MyApp.zip

# Submit and wait for notarization to complete
asc notarization submit --file ./MyApp.zip --wait

# Submit with custom polling interval and timeout
asc notarization submit --file ./MyApp.zip --wait --poll-interval 30s --timeout 1h

# Check notarization status
asc notarization status --id "SUBMISSION_ID"

# Get the developer log URL for a submission
asc notarization log --id "SUBMISSION_ID"

# List previous notarization submissions
asc notarization list
asc notarization list --output table
```

Notes:
- Supported file formats: zip, dmg, pkg
- The submit command computes the SHA-256 hash, creates a submission, and uploads the file to Apple
- Use `--wait` to poll until notarization completes (default timeout: 30 minutes)
- If notarization fails, use `asc notarization log --id` to retrieve the developer log URL with detailed results
- Uses the Apple Notary API v2 (`appstoreconnect.apple.com/notary/v2`)

### Game Center

```bash
# Achievements
asc game-center achievements list --app "APP_ID"
asc game-center achievements get --id "ACHIEVEMENT_ID"
asc game-center achievements create --app "APP_ID" --reference-name "First Win" --vendor-id "com.example.firstwin" --points 10
asc game-center achievements update --id "ACHIEVEMENT_ID" --points 20
asc game-center achievements delete --id "ACHIEVEMENT_ID" --confirm

# Achievement localizations
asc game-center achievements localizations list --achievement-id "ACHIEVEMENT_ID"
asc game-center achievements localizations create --achievement-id "ACHIEVEMENT_ID" --locale en-US --name "First Win" --before-earned-description "Win your first game" --after-earned-description "You won!"
asc game-center achievements localizations update --id "LOC_ID" --name "New Name"
asc game-center achievements localizations delete --id "LOC_ID" --confirm

# Achievement images
asc game-center achievements images upload --localization-id "LOC_ID" --file "path/to/image.png"
asc game-center achievements images get --id "IMAGE_ID"
asc game-center achievements images delete --id "IMAGE_ID" --confirm

# Achievement releases
asc game-center achievements releases list --achievement-id "ACHIEVEMENT_ID"
asc game-center achievements releases create --app "APP_ID" --achievement-id "ACHIEVEMENT_ID"
asc game-center achievements releases delete --id "RELEASE_ID" --confirm

# Leaderboards
asc game-center leaderboards list --app "APP_ID"
asc game-center leaderboards get --id "LEADERBOARD_ID"
asc game-center leaderboards create --app "APP_ID" --reference-name "High Score" --vendor-id "com.example.highscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE
asc game-center leaderboards update --id "LEADERBOARD_ID" --reference-name "New Name"
asc game-center leaderboards delete --id "LEADERBOARD_ID" --confirm

# Leaderboard localizations
asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID"
asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale en-US --name "High Score"
asc game-center leaderboards localizations update --id "LOC_ID" --name "New Name"
asc game-center leaderboards localizations delete --id "LOC_ID" --confirm

# Leaderboard images
asc game-center leaderboards images upload --localization-id "LOC_ID" --file "path/to/image.png"
asc game-center leaderboards images delete --id "IMAGE_ID" --confirm

# Leaderboard releases
asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID"
asc game-center leaderboards releases create --app "APP_ID" --leaderboard-id "LEADERBOARD_ID"
asc game-center leaderboards releases delete --id "RELEASE_ID" --confirm

# Leaderboard Sets
asc game-center leaderboard-sets list --app "APP_ID"
asc game-center leaderboard-sets get --id "SET_ID"
asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
asc game-center leaderboard-sets update --id "SET_ID" --reference-name "Season 1 - Updated"
asc game-center leaderboard-sets delete --id "SET_ID" --confirm

# Leaderboard Set members
asc game-center leaderboard-sets members list --set-id "SET_ID"
asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3"

# Leaderboard Set localizations
asc game-center leaderboard-sets localizations list --set-id "SET_ID"
asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale en-US --name "Season 1"
asc game-center leaderboard-sets localizations update --id "LOC_ID" --name "New Name"
asc game-center leaderboard-sets localizations delete --id "LOC_ID" --confirm

# Leaderboard Set images
asc game-center leaderboard-sets images upload --localization-id "LOC_ID" --file "path/to/image.png"
asc game-center leaderboard-sets images delete --id "IMAGE_ID" --confirm

# Leaderboard Set releases
asc game-center leaderboard-sets releases list --set-id "SET_ID"
asc game-center leaderboard-sets releases create --app "APP_ID" --set-id "SET_ID"
asc game-center leaderboard-sets releases delete --id "RELEASE_ID" --confirm
```

### Signing

```bash
# Fetch signing files (certificates + profiles) for an app
asc signing fetch --bundle-id "com.example.app" --profile-type IOS_APP_STORE --output "./signing"

# Create missing profiles automatically
asc signing fetch --bundle-id "com.example.app" --profile-type IOS_APP_DEVELOPMENT --device "DEVICE_ID" --create-missing

# Filter by certificate type
asc signing fetch --bundle-id "com.example.app" --profile-type IOS_APP_STORE --certificate-type IOS_DISTRIBUTION
```

### Certificates

```bash
# List signing certificates
asc certificates list
asc certificates list --certificate-type "IOS_DISTRIBUTION,IOS_DEVELOPMENT"

# Fetch all certificates (all pages)
asc certificates list --paginate

# Get a certificate by ID
asc certificates get --id "CERT_ID"

# Create a signing certificate
asc certificates create --certificate-type "IOS_DISTRIBUTION" --csr "./CertificateSigningRequest.certSigningRequest"

# Update a certificate
asc certificates update --id "CERT_ID" --activated true

# Revoke a certificate (irreversible)
asc certificates revoke --id "CERT_ID" --confirm
```

### Profiles

```bash
# List provisioning profiles
asc profiles list
asc profiles list --profile-type "IOS_APP_STORE,IOS_APP_DEVELOPMENT"

# Fetch all profiles (all pages)
asc profiles list --paginate

# Get a profile by ID (with related resources)
asc profiles get --id "PROFILE_ID" --include "bundleId,certificates,devices"

# Create a provisioning profile
asc profiles create --name "My App Store Profile" --profile-type IOS_APP_STORE --bundle "BUNDLE_ID" --certificate "CERT_ID"

# Create a development profile with devices
asc profiles create --name "Dev Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID" --device "DEVICE_ID1,DEVICE_ID2"

# Download a profile
asc profiles download --id "PROFILE_ID" --output "./profile.mobileprovision"

# Delete a profile
asc profiles delete --id "PROFILE_ID" --confirm

# View profile relationships
asc profiles relationships bundle-id --id "PROFILE_ID"
asc profiles relationships certificates --id "PROFILE_ID"
asc profiles relationships devices --id "PROFILE_ID"
```

### Bundle IDs

```bash
# List bundle IDs
asc bundle-ids list
asc bundle-ids list --paginate

# Get a bundle ID by ID
asc bundle-ids get --id "BUNDLE_ID"

# Create a bundle ID
asc bundle-ids create --identifier "com.example.app" --name "My App" --platform IOS

# Update a bundle ID
asc bundle-ids update --id "BUNDLE_ID" --name "New Name"

# Delete a bundle ID
asc bundle-ids delete --id "BUNDLE_ID" --confirm

# View linked app
asc bundle-ids app get --id "BUNDLE_ID"

# List linked profiles
asc bundle-ids profiles list --id "BUNDLE_ID"

# Manage capabilities
asc bundle-ids capabilities list --bundle "BUNDLE_ID"
asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability IN_APP_PURCHASE
asc bundle-ids capabilities remove --id "CAPABILITY_ID" --confirm
```

### Subscriptions

```bash
# Subscription groups
asc subscriptions groups list --app "APP_ID"
asc subscriptions groups create --app "APP_ID" --reference-name "Premium"
asc subscriptions groups get --id "GROUP_ID"
asc subscriptions groups update --id "GROUP_ID" --reference-name "Premium+"
asc subscriptions groups delete --id "GROUP_ID" --confirm

# Group localizations
asc subscriptions groups localizations list --group-id "GROUP_ID"
asc subscriptions groups localizations create --group-id "GROUP_ID" --locale en-US --name "Premium"
asc subscriptions groups localizations update --id "LOC_ID" --name "Premium+"
asc subscriptions groups localizations delete --id "LOC_ID" --confirm

# Submit a group for review
asc subscriptions groups submit --group-id "GROUP_ID" --confirm

# Subscriptions within a group
asc subscriptions list --group "GROUP_ID"
asc subscriptions create --group "GROUP_ID" --ref-name "Monthly" --product-id "com.example.monthly" --subscription-period "ONE_MONTH"
asc subscriptions get --id "SUB_ID"
asc subscriptions update --id "SUB_ID" --ref-name "Monthly Premium"
asc subscriptions delete --id "SUB_ID" --confirm
asc subscriptions submit --subscription-id "SUB_ID" --confirm

# Pricing
asc subscriptions pricing --app "APP_ID"
asc subscriptions pricing --subscription-id "SUB_ID" --territory "USA"

# Prices
asc subscriptions prices list --id "SUB_ID"
asc subscriptions prices add --id "SUB_ID" --price-point "PRICE_POINT_ID"
asc subscriptions prices delete --price-id "PRICE_ID" --confirm

# Availability
asc subscriptions availability get --subscription-id "SUB_ID"
asc subscriptions availability set --id "SUB_ID" --territory "USA,GBR,JPN"
asc subscriptions availability available-territories --id "AVAILABILITY_ID"

# Localizations
asc subscriptions localizations list --subscription-id "SUB_ID"
asc subscriptions localizations create --subscription-id "SUB_ID" --locale en-US --name "Monthly"
asc subscriptions localizations update --id "LOC_ID" --name "Monthly Premium"
asc subscriptions localizations delete --id "LOC_ID" --confirm

# Introductory offers
asc subscriptions introductory-offers list --subscription-id "SUB_ID"
asc subscriptions introductory-offers create --subscription-id "SUB_ID" --offer-duration "ONE_MONTH" --offer-mode "FREE_TRIAL" --number-of-periods 1
asc subscriptions introductory-offers delete --id "OFFER_ID" --confirm

# Promotional offers
asc subscriptions promotional-offers list --subscription-id "SUB_ID"
asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "PROMO1" --name "Holiday" --offer-duration "ONE_MONTH" --offer-mode "PAY_AS_YOU_GO" --number-of-periods 3 --prices "PRICE_ID1,PRICE_ID2"
asc subscriptions promotional-offers delete --id "OFFER_ID" --confirm

# Price points
asc subscriptions price-points list --subscription-id "SUB_ID"
asc subscriptions price-points get --id "PRICE_POINT_ID"
asc subscriptions price-points equalizations --id "PRICE_POINT_ID"

# Images and review screenshots
asc subscriptions images list --subscription-id "SUB_ID"
asc subscriptions images create --subscription-id "SUB_ID" --file "./image.png"
asc subscriptions review-screenshots create --subscription-id "SUB_ID" --file "./screenshot.png"
```

### In-App Purchases

```bash
# List in-app purchases
asc iap list --app "APP_ID"
asc iap list --app "APP_ID" --paginate

# Legacy in-app purchases
asc iap list --app "APP_ID" --legacy
asc iap get --id "IAP_ID" --legacy

# Create an in-app purchase
asc iap create --app "APP_ID" --type CONSUMABLE --ref-name "100 Coins" --product-id "com.example.coins100"

# Update and delete
asc iap update --id "IAP_ID" --ref-name "200 Coins"
asc iap delete --id "IAP_ID" --confirm

# Pricing summary
asc iap prices --app "APP_ID"
asc iap prices --iap-id "IAP_ID" --territory "USA"

# Submit for review
asc iap submit --iap-id "IAP_ID" --confirm

# Localizations
asc iap localizations list --iap-id "IAP_ID"
asc iap localizations create --iap-id "IAP_ID" --locale en-US --name "100 Coins" --description "Buy 100 coins"
asc iap localizations update --id "LOC_ID" --name "200 Coins"
asc iap localizations delete --id "LOC_ID" --confirm

# Images and review screenshots
asc iap images list --iap-id "IAP_ID"
asc iap images create --iap-id "IAP_ID" --file "./image.png"
asc iap review-screenshots create --iap-id "IAP_ID" --file "./screenshot.png"

# Availability
asc iap availability get --iap-id "IAP_ID"
asc iap availability set --iap-id "IAP_ID" --territories "USA,GBR,JPN"

# Price points and schedules
asc iap price-points list --iap-id "IAP_ID"
asc iap price-schedules get --iap-id "IAP_ID"
asc iap price-schedules create --iap-id "IAP_ID" --base-territory "USA" --prices "PRICE_POINT_ID"
```

### Performance

```bash
# App-level performance metrics
asc performance metrics list --app "APP_ID"
asc performance metrics list --app "APP_ID" --metric-type "LAUNCH,HANG" --platform IOS

# Build-level performance metrics
asc performance metrics get --build "BUILD_ID"
asc performance metrics get --build "BUILD_ID" --metric-type "BATTERY,MEMORY"

# Diagnostic signatures for a build
asc performance diagnostics list --build "BUILD_ID"
asc performance diagnostics list --build "BUILD_ID" --diagnostic-type "DISK_WRITES,HANGS"

# Diagnostic logs for a signature
asc performance diagnostics get --id "SIGNATURE_ID"

# Download metrics or diagnostics
asc performance download --app "APP_ID" --output "./metrics.json"
asc performance download --build "BUILD_ID" --output "./build-metrics.json"
asc performance download --diagnostic-id "SIGNATURE_ID" --output "./diagnostic.json"
```

### Webhooks

```bash
# List webhooks for an app
asc webhooks list --app "APP_ID"

# Get a webhook by ID
asc webhooks get --webhook-id "WEBHOOK_ID"

# Create a webhook
asc webhooks create --app "APP_ID" --name "Build Notifications" --url "https://example.com/webhook" --secret "my-secret" --events "BUILD_CREATED,BUILD_UPDATED" --enabled true

# Update a webhook
asc webhooks update --webhook-id "WEBHOOK_ID" --enabled false
asc webhooks update --webhook-id "WEBHOOK_ID" --events "BUILD_CREATED"

# Delete a webhook
asc webhooks delete --webhook-id "WEBHOOK_ID" --confirm

# Webhook deliveries
asc webhooks deliveries --webhook-id "WEBHOOK_ID" --created-after "2025-01-01"

# Redeliver a failed delivery
asc webhooks deliveries redeliver --delivery-id "DELIVERY_ID"

# Ping a webhook
asc webhooks ping --webhook-id "WEBHOOK_ID"
```

### Publish (End-to-End Workflows)

```bash
# Upload and distribute to TestFlight in one step
asc publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta Testers"

# With multiple groups and notification
asc publish testflight --app "APP_ID" --ipa "app.ipa" --group "Internal,External" --notify --wait

# With "What to Test" notes
asc publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta" --test-notes "Test login flow" --locale "en-US" --wait

# Upload and submit to App Store in one step
asc publish appstore --app "APP_ID" --ipa "app.ipa" --submit --confirm --wait
```

Notes:
- `--version` and `--build-number` are auto-extracted from the IPA if not provided
- Default timeout is 30 minutes; override with `--timeout`

### App Clips

```bash
# List App Clips for an app
asc app-clips list --app "APP_ID"

# Get App Clip details
asc app-clips get --id "APP_CLIP_ID"

# Default experiences
asc app-clips default-experiences list --app-clip-id "APP_CLIP_ID"
asc app-clips default-experiences get --experience-id "EXP_ID"
asc app-clips default-experiences create --app-clip-id "APP_CLIP_ID" --action OPEN
asc app-clips default-experiences update --experience-id "EXP_ID" --action VIEW
asc app-clips default-experiences delete --experience-id "EXP_ID" --confirm

# Advanced experiences
asc app-clips advanced-experiences list --app-clip-id "APP_CLIP_ID"
asc app-clips advanced-experiences create --app-clip-id "APP_CLIP_ID" --link "https://example.com/clip" --default-language en --is-powered-by true
asc app-clips advanced-experiences update --experience-id "EXP_ID" --removed true
asc app-clips advanced-experiences delete --experience-id "EXP_ID" --confirm

# Header images
asc app-clips header-images create --localization-id "LOC_ID" --file "header.png"
asc app-clips header-images get --id "IMAGE_ID"
asc app-clips header-images delete --id "IMAGE_ID" --confirm

# Beta invocations (for testing)
asc app-clips invocations list --build-bundle-id "BUNDLE_ID"
asc app-clips invocations create --build-bundle-id "BUNDLE_ID" --url "https://example.com/clip"
asc app-clips invocations delete --invocation-id "INVOCATION_ID" --confirm

# Domain status
asc app-clips domain-status cache --build-bundle-id "BUNDLE_ID"
asc app-clips domain-status debug --build-bundle-id "BUNDLE_ID"
```

### Encryption

```bash
# List encryption declarations for an app
asc encryption declarations list --app "APP_ID"

# Get a declaration by ID
asc encryption declarations get --id "DECLARATION_ID"

# Create an encryption declaration
asc encryption declarations create --app "APP_ID" --app-description "Uses HTTPS only" --contains-proprietary-cryptography false --contains-third-party-cryptography false --available-on-french-store true

# Assign builds to a declaration
asc encryption declarations assign-builds --id "DECLARATION_ID" --build "BUILD_ID1,BUILD_ID2"

# Upload an encryption document
asc encryption documents upload --declaration "DECLARATION_ID" --file "./encryption-doc.pdf"

# Get a document
asc encryption documents get --id "DOC_ID"
```

### Screenshots & Video Previews

```bash
# Capture and frame screenshots locally
asc screenshots capture --bundle-id "com.example.app" --name home
asc screenshots frame --input "./screenshots/raw/home.png" --device iphone-air

# Generate and approve review artifacts
asc screenshots review-generate --framed-dir "./screenshots/framed" --output-dir "./screenshots/review"
asc screenshots review-open --output-dir "./screenshots/review"
asc screenshots review-approve --all-ready --output-dir "./screenshots/review"

# List screenshots for a version localization
asc screenshots list --version-localization "LOC_ID"

# List supported screenshot sizes
asc screenshots sizes
asc screenshots sizes --display-type "APP_IPHONE_65"

# Upload screenshots
asc screenshots upload --version-localization "LOC_ID" --path "./screenshots/" --device-type IPHONE_65

# Delete a screenshot
asc screenshots delete --id "SCREENSHOT_ID" --confirm

# List and upload video previews
asc video-previews list --version-localization "LOC_ID"
asc video-previews upload --version-localization "LOC_ID" --path "./previews/" --device-type IPHONE_65
asc video-previews delete --id "PREVIEW_ID" --confirm
```

### Background Assets

```bash
# List background assets for an app
asc background-assets list --app "APP_ID"
asc background-assets list --app "APP_ID" --archived false

# Create and manage background assets
asc background-assets get --id "ASSET_ID"
asc background-assets create --app "APP_ID" --asset-pack-identifier "com.example.assets.pack1"
asc background-assets update --id "ASSET_ID" --archived true

# Manage versions
asc background-assets versions list --background-asset-id "ASSET_ID"
asc background-assets versions create --background-asset-id "ASSET_ID"

# Upload files
asc background-assets upload-files list --version-id "VERSION_ID"
asc background-assets upload-files create --version-id "VERSION_ID" --file "./asset.bin" --asset-type ASSET
```

### Routing Coverage

```bash
# Get routing app coverage for a version
asc routing-coverage get --version-id "VERSION_ID"

# Upload routing app coverage
asc routing-coverage create --version-id "VERSION_ID" --file "./routing.geojson"

# Get by ID
asc routing-coverage info --id "COVERAGE_ID"

# Delete routing coverage
asc routing-coverage delete --id "COVERAGE_ID" --confirm
```

### Notify

```bash
# Send a Slack notification
asc notify slack --webhook "https://hooks.slack.com/services/..." --message "Build deployed!"

# Send to a specific channel
asc notify slack --webhook "https://hooks.slack.com/services/..." --message "v1.0.0 live" --channel "#releases"
```

Notes:
- Set `ASC_SLACK_WEBHOOK` env var to avoid passing `--webhook` each time
- Webhook URL must target `hooks.slack.com` over HTTPS

### Apps & Builds

```bash
# List apps (useful for finding app IDs)
asc apps

# View the community Wall of Apps
asc apps wall

# Generate markdown wall output
asc apps wall --output markdown

# Filter and sort wall output
asc apps wall --include-platforms iOS,macOS --sort -name --limit 20

# Sort apps by name or bundle ID
asc apps --sort name
asc apps --sort -bundleId

# Fetch all apps (all pages)
asc apps --paginate

# List builds for an app
asc builds list --app "123456789"

# Sort builds by upload date (newest first)
asc builds list --app "123456789" --sort -uploadedDate

# Fetch all builds (all pages)
asc builds list --app "123456789" --paginate

# Build details
asc builds info --build "BUILD_ID"

# Expire a build (irreversible)
asc builds expire --build "BUILD_ID" --confirm

# Expire builds in bulk (use --dry-run to preview)
asc builds expire-all --app "123456789" --older-than 90d --dry-run
asc builds expire-all --app "123456789" --older-than 90d --confirm

# Upload a build
asc builds upload --app "123456789" --ipa "app.ipa"

# Upload a macOS build
asc builds upload --app "123456789" --pkg "app.pkg" --version "1.0.0" --build-number "123"

# Upload with concurrent chunk uploads
asc builds upload --app "123456789" --ipa "app.ipa" --concurrency 4

# Upload and verify checksums
asc builds upload --app "123456789" --ipa "app.ipa" --checksum

# Upload and wait for build processing
asc builds upload --app "123456789" --ipa "app.ipa" --wait

# Upload with "What to Test" notes
asc builds upload --app "123456789" --ipa "app.ipa" --test-notes "Test the new login flow" --locale "en-US" --wait

# Dry run (reserve upload operations only)
asc builds upload --app "123456789" --ipa "app.ipa" --dry-run

# Manage build uploads
asc builds uploads list --app "123456789"
asc builds uploads get --id "UPLOAD_ID"
asc builds uploads delete --id "UPLOAD_ID" --confirm
asc builds uploads files list --upload "UPLOAD_ID"

# Get the latest build for an app
asc builds latest --app "123456789"
asc builds latest --app "123456789" --version "1.0.0" --platform IOS

# Manage build test notes (What to Test)
asc builds test-notes list --build "BUILD_ID"
asc builds test-notes get --id "LOCALIZATION_ID"
asc builds test-notes create --build "BUILD_ID" --locale "en-US" --whats-new "Test the new login flow"
asc builds test-notes update --id "LOCALIZATION_ID" --whats-new "Updated test notes"
asc builds test-notes delete --id "LOCALIZATION_ID" --confirm

# Manage individual testers on a build
asc builds individual-testers list --build "BUILD_ID"
asc builds individual-testers add --build "BUILD_ID" --tester "TESTER_ID"
asc builds individual-testers remove --build "BUILD_ID" --tester "TESTER_ID"

# Add/remove beta groups from a build
asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
asc builds remove-groups --build "BUILD_ID" --group "GROUP_ID" --confirm

# Build relationships and related resources
asc builds app get --build "BUILD_ID"
asc builds pre-release-version get --build "BUILD_ID"
asc builds icons list --build "BUILD_ID"
asc builds beta-app-review-submission get --build "BUILD_ID"
asc builds build-beta-detail get --build "BUILD_ID"
asc builds relationships get --build "BUILD_ID" --type "app"

# Build metrics
asc builds metrics beta-usages --build "BUILD_ID"
```

### App Setup

```bash
# Set bundle ID and primary locale
asc app-setup info set --app "APP_ID" --primary-locale "en-US" --bundle-id "com.example.app"

# Set localized app info
asc app-setup info set --app "APP_ID" --locale "en-US" --name "My App" --subtitle "Great app"

# Set categories
asc app-setup categories set --app "APP_ID" --primary GAMES --secondary ENTERTAINMENT

# Set availability
asc app-setup availability set --app "APP_ID" --territory "USA,GBR" --available true --available-in-new-territories true

# Set pricing
asc app-setup pricing set --app "APP_ID" --price-point "PRICE_POINT_ID" --base-territory "USA"

# Upload localizations
asc app-setup localizations upload --version "VERSION_ID" --path "./localizations"
```

### Offer Codes (Subscriptions)

```bash
# List one-time use offer code batches for a subscription offer
asc offer-codes list --offer-code "OFFER_CODE_ID"

# Fetch all offer code batches (all pages)
asc offer-codes list --offer-code "OFFER_CODE_ID" --paginate

# Get an offer code by ID
asc offer-codes get --offer-code-id "OFFER_CODE_ID"

# Create an offer code
asc offer-codes create --subscription-id "SUB_ID" --name "Holiday" --customer-eligibilities "NEW,EXISTING" --offer-eligibility "ONCE" --duration "ONE_MONTH" --offer-mode "PAY_AS_YOU_GO" --number-of-periods 3 --prices "USA:PRICE_POINT_ID"

# Update an offer code
asc offer-codes update --offer-code-id "OFFER_CODE_ID" --active false

# Generate one-time use offer codes
asc offer-codes generate --offer-code "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01"

# Download one-time use offer codes to a file
asc offer-codes values --id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.txt"

# Manage custom (vanity) codes
asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID"
asc offer-codes custom-codes create --offer-code-id "OFFER_CODE_ID" --custom-code "HOLIDAY2026"
asc offer-codes custom-codes update --id "CUSTOM_CODE_ID" --active false

# List offer code prices
asc offer-codes prices list --offer-code-id "OFFER_CODE_ID"
```

### Categories

```bash
# List all App Store categories
asc categories list
asc categories list --output table

# Set primary and secondary categories for an app
asc categories set --app "123456789" --primary GAMES
asc categories set --app "123456789" --primary GAMES --secondary ENTERTAINMENT
```

### Versions

```bash
# List App Store versions
asc versions list --app "123456789"

# Fetch all versions (all pages)
asc versions list --app "123456789" --paginate

# Get version details
asc versions get --version-id "VERSION_ID"

# Create a new App Store version
asc versions create --app "123456789" --version "1.0.0"
asc versions create --app "123456789" --version "2.0.0" --platform IOS --release-type MANUAL

# Update a version
asc versions update --version-id "VERSION_ID" --version "1.0.1"

# Delete a version
asc versions delete --version-id "VERSION_ID" --confirm

# Attach a build to a version
asc versions attach-build --version-id "VERSION_ID" --build "BUILD_ID"

# Release a pending developer release version
asc versions release --version-id "VERSION_ID" --confirm

# Manage phased release
asc versions phased-release get --version-id "VERSION_ID"
asc versions phased-release create --version-id "VERSION_ID"
asc versions phased-release update --id "PHASED_ID" --state PAUSED
asc versions phased-release delete --id "PHASED_ID" --confirm

# Create a version promotion (create-only in API spec; treatment required)
asc versions promotions create --version-id "VERSION_ID" --treatment-id "TREATMENT_ID"
```

### App Info

```bash
# Get App Store metadata for the latest version
asc app-info get --app "123456789"

# Get metadata for a specific version
asc app-info get --app "123456789" --version "1.2.3" --platform IOS

# Include related resources
asc app-info get --app "123456789" --include "ageRatingDeclaration,territoryAgeRatings"

# Update metadata for a locale
asc app-info set --app "123456789" --locale "en-US" --whats-new "Bug fixes"
asc app-info set --app "123456789" --locale "en-US" --description "My app description" --keywords "app,tool" --support-url "https://example.com/support"
asc app-info set --app "123456789" --locale "en-US" --promotional-text "Now with dark mode!" --marketing-url "https://example.com"
```

### Pre-Release Versions

```bash
# List pre-release versions for an app
asc pre-release-versions list --app "123456789"

# Filter by platform or version
asc pre-release-versions list --app "123456789" --platform IOS
asc pre-release-versions list --app "123456789" --version "1.0.0"

# Fetch all pre-release versions (all pages)
asc pre-release-versions list --app "123456789" --paginate

# Get pre-release version details
asc pre-release-versions get --id "PRERELEASE_ID"
```

### Localizations

```bash
# List version localizations
asc localizations list --version "VERSION_ID"

# List app info localizations
asc localizations list --app "APP_ID" --type app-info

# Fetch all localizations (all pages)
asc localizations list --version "VERSION_ID" --paginate

# Download/upload localization files
asc localizations download --version "VERSION_ID" --path "./localizations"
asc localizations upload --version "VERSION_ID" --path "./localizations"
```

### Build Localizations

```bash
# List build release notes localizations
asc build-localizations list --build "BUILD_ID"

# Fetch all build localizations (all pages)
asc build-localizations list --build "BUILD_ID" --paginate

# Create/update/delete release notes
asc build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "Bug fixes"
asc build-localizations update --id "LOCALIZATION_ID" --whats-new "New features"
asc build-localizations delete --id "LOCALIZATION_ID" --confirm

# Fetch a localization by ID
asc build-localizations get --id "LOCALIZATION_ID"
```

### Migrate (Fastlane Compatibility)

Validate and migrate metadata between ASC's `.strings` format and Deliver-style directory layout.

```bash
# Validate metadata against App Store Connect character limits (offline)
asc migrate validate --fastlane-dir ./fastlane

# Import metadata, review info, and screenshots from Deliver-style layout
asc migrate import --app "123456789" --version-id "VERSION_ID" --fastlane-dir ./fastlane

# Import from project root using Deliverfile (app_identifier + app_version + platform)
asc migrate import --dry-run

# Export metadata from App Store Connect to fastlane format
asc migrate export --app "123456789" --version-id "VERSION_ID" --output-dir ./exported-metadata
```

**Character limits validated:**
| Field | Limit |
|-------|-------|
| Description | 4000 chars |
| Keywords | 100 chars |
| What's New | 4000 chars |
| Promotional Text | 170 chars |
| Name | 30 chars |
| Subtitle | 30 chars |

### Validate (Pre-Submission)

Run client-side checks before submission to catch metadata, screenshot, and age rating issues early.

```bash
# Validate release readiness for a version
asc validate --app "123456789" --version-id "VERSION_ID"

# Validate with platform-specific screenshot checks and table output
asc validate --app "123456789" --version-id "VERSION_ID" --platform IOS --output table

# Treat warnings as errors for CI gating
asc validate --app "123456789" --version-id "VERSION_ID" --strict
```

**Checks included:**
- Metadata length limits (description, keywords, release notes, promotional text, name, subtitle)
- Required field presence (localizations, required text fields)
- Screenshot size compatibility (per display type)
- Age rating completeness

### Submit

```bash
# Submit a build for review
asc submit create --app "123456789" --version "1.0.0" --build "BUILD_ID" --confirm

# Check submission status
asc submit status --id "SUBMISSION_ID"
asc submit status --version-id "VERSION_ID"

# Cancel a submission
asc submit cancel --id "SUBMISSION_ID" --confirm
asc submit cancel --version-id "VERSION_ID" --confirm
```

### Utilities

```bash
# Print version information
asc version
asc --version
```

### Output Formats

| Format | Flag | Use Case |
|--------|------|----------|
| JSON (minified) | default | Scripting, automation |
| Table | `--output table` | Terminal display |
| Markdown | `--output markdown` | Documentation |

Note: When using `--paginate`, the response `links` field is cleared to avoid confusion about additional pages.

### Authentication

```bash
# Check authentication status
asc auth status
asc auth status --verbose
asc auth status --validate

# Diagnose authentication issues (includes migration hints when fastlane files are detected)
asc auth doctor
asc auth doctor --output json --pretty
asc auth doctor --fix --confirm

# Logout
asc auth logout
asc auth logout --all
asc auth logout --name "MyApp"
```

## Design Philosophy

### Explicit Over Cryptic

```bash
# Good - self-documenting
asc reviews --app "MyApp" --stars 1

# Avoid - cryptic flags (hypothetical, not supported)
# asc reviews -a "MyApp" -s 1
```

### JSON-First Output

All commands output minified JSON by default for easy parsing:

```bash
asc feedback --app "123456789" | jq '.data[].attributes.comment'
```

JSON is minified (one line) by default. Use `--output table` or `--output markdown` for human-readable output.

### No Interactive Prompts

Everything is flag-based for automation:

```bash
# Non-interactive (good for CI/CD and scripts)
asc feedback --app "123456789"

# No prompts, no waiting
```

## Installation

### Homebrew (macOS)

```bash
# Add tap and install
brew tap rudrankriyam/tap
brew install rudrankriyam/tap/asc
```

### From Source

```bash
git clone https://github.com/rudrankriyam/App-Store-Connect-CLI.git
cd App-Store-Connect-CLI
make build
make install  # Installs to /usr/local/bin
```

## Documentation

- [ASC.md](ASC.md) - Command catalog (generate in your repo with `asc init`)
- [CLAUDE.md](CLAUDE.md) - Development guidelines for AI assistants
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [docs/openapi/README.md](docs/openapi/README.md) - Offline OpenAPI snapshot + update steps

## How to test in <10 minutes>

```bash
make tools   # installs gofumpt + golangci-lint (required for make format)
make format
make lint
make test
make build
./asc --help
```

## Security

- Credentials stored in the system keychain when available
- Local config fallback with restricted permissions
- Private key content never stored, only path reference
- Environment variables as fallback
- Report/artifact file helpers use atomic `O_NOFOLLOW` on Unix-like systems
- On platforms without a portable `O_NOFOLLOW`, helpers use best-effort pre/post `Lstat` checks to reject symlinks and detect path swaps (residual TOCTOU window remains)

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Author

[Rudrank Riyam](https://github.com/rudrankriyam)

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=rudrankriyam/App-Store-Connect-CLI&type=Date)](https://star-history.com/#rudrankriyam/App-Store-Connect-CLI&Date)

---

<p align="center">
  Primarily Built with Cursor and GPT-5.2 Codex Extra High
</p>

<p align="center">
  <sub>This project is an independent, unofficial tool and is not affiliated with, endorsed by, or sponsored by Apple Inc. App Store Connect, TestFlight, Xcode Cloud, and Apple are trademarks of Apple Inc., registered in the U.S. and other countries.</sub>
</p>

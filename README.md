# Unofficial App Store Connect CLI

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/badge/Homebrew-compatible-blue?style=for-the-badge" alt="Homebrew">
</p>

A **fast**, **lightweight**, and **AI-agent friendly** CLI for App Store Connect. Ship iOS apps with zero friction.

## Why ASC?

| Problem | Solution |
|---------|----------|
| Manual App Store Connect work | Automate everything from CLI |
| Slow, heavy tooling | Go binary, fast startup |
| Not AI-agent friendly | JSON output, explicit flags, clean exit codes |

## Table of Contents

- [Why ASC?](#why-asc)
- [Quick Start](#quick-start)
  - [Install](#install)
  - [Authenticate](#authenticate)
- [Commands](#commands)
  - [Agent Quickstart](#agent-quickstart)
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
  - [Game Center](#game-center)
  - [Apps & Builds](#apps--builds)
- [App Setup](#app-setup)
  - [Categories](#categories)
  - [Versions](#versions)
  - [App Info](#app-info)
  - [Pre-Release Versions](#pre-release-versions)
  - [Localizations](#localizations)
  - [Build Localizations](#build-localizations)
  - [Migrate (Fastlane Compatibility)](#migrate-fastlane-compatibility)
  - [Submit](#submit)
  - [Utilities](#utilities)
  - [Output Formats](#output-formats)
  - [Authentication](#authentication)
- [Design Philosophy](#design-philosophy)
  - [Explicit Over Cryptic](#explicit-over-cryptic)
  - [AI-Agent Friendly](#ai-agent-friendly)
  - [No Interactive Prompts](#no-interactive-prompts)
- [Installation](#installation)
- [Documentation](#documentation)
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
`ASC_BYPASS_KEYCHAIN` is set and environment credentials are fully provided, the
environment values take precedence over config.
Environment variable fallback:
- `ASC_KEY_ID`
- `ASC_ISSUER_ID`
- `ASC_PRIVATE_KEY_PATH`
- `ASC_PRIVATE_KEY` (raw key content; CLI writes a temp key file)
- `ASC_PRIVATE_KEY_B64` (base64 key content; CLI writes a temp key file)
- `ASC_CONFIG_PATH` (absolute path to config.json)
- `ASC_PROFILE`
- `ASC_BYPASS_KEYCHAIN` (ignore keychain and use config/env auth)
- `ASC_STRICT_AUTH` (fail when credentials resolve from multiple sources)

Use `--strict-auth` or `ASC_STRICT_AUTH=1` to fail when credentials are resolved from multiple sources.

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

## Commands

### Agent Quickstart

- JSON output is default for machine parsing; add `--pretty` when debugging.
- Use `--paginate` to automatically fetch all pages (recommended for AI agents).
- `--paginate` works on list commands including apps, builds list, app-tags list, app-tags territories, promo codes list, devices list, feedback, crashes, reviews, versions list, pre-release versions list, localizations list, build-localizations list, beta-groups list, beta-testers list, sandbox list, analytics requests/get, testflight apps list, game-center achievements/leaderboards/leaderboard-sets lists (including localizations/releases/members), and Xcode Cloud workflows/build-runs.
- Use `--limit` + `--next "<links.next>"` for manual pagination control.
- Sort with `--sort` (prefix `-` for descending):
  - Feedback/Crashes: `createdDate` / `-createdDate`
  - Reviews: `rating` / `-rating`, `createdDate` / `-createdDate`
  - Apps: `name` / `-name`, `bundleId` / `-bundleId`
  - Builds: `uploadedDate` / `-uploadedDate`

### TestFlight

```bash
# List beta feedback (JSON - best for AI agents)
asc feedback --app "123456789"

# Filter feedback by device model and OS version
asc feedback --app "123456789" --device-model "iPhone15,3" --os-version "17.2"

# Filter feedback by platform/build/tester
asc feedback --app "123456789" --app-platform IOS --device-platform IOS --build "BUILD_ID" --tester "TESTER_ID"

# Fetch all feedback pages automatically (AI agents)
asc feedback --app "123456789" --paginate

# Get crash reports (table format - for humans)
asc crashes --app "123456789" --output table

# Get crash reports (markdown - for docs)
asc crashes --app "123456789" --output markdown

# Limit results per page (pagination)
asc crashes --app "123456789" --limit 25

# Sort crashes by created date (newest first)
asc crashes --app "123456789" --sort -createdDate --limit 5

# Fetch all crash pages automatically (AI agents)
asc crashes --app "123456789" --paginate

# List TestFlight apps
asc testflight apps list

# Fetch a TestFlight app by ID
asc testflight apps get --app "APP_ID"

# Export TestFlight configuration to YAML
asc testflight sync pull --app "APP_ID" --output "./testflight.yaml"
```

### Beta Groups

```bash
# List beta groups for an app
asc beta-groups list --app "APP_ID"

# Fetch all beta groups (all pages)
asc beta-groups list --app "APP_ID" --paginate

# Create, fetch, update, delete
asc beta-groups create --app "APP_ID" --name "Beta Testers"
asc beta-groups get --id "GROUP_ID"
asc beta-groups update --id "GROUP_ID" --name "New Name"
asc beta-groups delete --id "GROUP_ID" --confirm

# Add/remove testers
asc beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID"
asc beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID"
```

### Beta Testers

```bash
# List beta testers
asc beta-testers list --app "APP_ID"

# Filter by build or group
asc beta-testers list --app "APP_ID" --build "BUILD_ID"
asc beta-testers list --app "APP_ID" --group "Beta"

# Fetch all beta testers (all pages)
asc beta-testers list --app "APP_ID" --paginate

# Get, add, remove, invite
asc beta-testers get --id "TESTER_ID"
asc beta-testers add --app "APP_ID" --email "tester@example.com" --group "Beta"
asc beta-testers remove --app "APP_ID" --email "tester@example.com"
asc beta-testers invite --app "APP_ID" --email "tester@example.com"

# Manage group membership
asc beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID"
asc beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID"
```

### Devices

```bash
# List devices
asc devices list

# Filter by platform/status/UDID
asc devices list --platform IOS --status ENABLED --udid "UDID1,UDID2"

# Fetch all devices (all pages)
asc devices list --paginate

# Get a device by ID
asc devices get --id "DEVICE_ID"

# Register a device
asc devices register --name "My iPhone" --udid "UDID" --platform IOS

# Update device name/status
asc devices update --id "DEVICE_ID" --name "New Name"
asc devices update --id "DEVICE_ID" --status DISABLED
```

### App Store

```bash
# List customer reviews (JSON - best for AI agents)
asc reviews --app "123456789"

# Filter by stars (table format - for humans)
asc reviews --app "123456789" --stars 1 --output table

# Filter by territory (markdown - for docs)
asc reviews --app "123456789" --territory US --output markdown

# Sort reviews by created date (newest first)
asc reviews --app "123456789" --sort -createdDate --limit 5

# Fetch all reviews pages automatically (AI agents)
asc reviews --app "123456789" --paginate

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
```

Notes:
- Workflows must have manual start conditions enabled to be triggered via API
- Use `--workflow` with `--app` for human-friendly workflow lookup by name
- Use `--workflow-id` and `--git-reference-id` for direct ID-based triggering
- Use `asc xcode-cloud workflows` and `asc xcode-cloud build-runs` to discover IDs
- When using `--wait`, the command polls until the build completes (or times out)
- Exit code is non-zero if the build fails, errors, or is canceled
- Use `ASC_TIMEOUT` env var or `--timeout` flag for long-running builds

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

### Apps & Builds

```bash
# List apps (useful for finding app IDs)
asc apps

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
asc builds expire --build "BUILD_ID"

# Expire builds in bulk (use --dry-run to preview)
asc builds expire-all --app "123456789" --older-than 90d --dry-run
asc builds expire-all --app "123456789" --older-than 90d --confirm

# Prepare a build upload
asc builds upload --app "123456789" --ipa "app.ipa"

Notes:
- Build upload currently prepares upload operations only (upload + commit is not yet automated).

# Add/remove beta groups from a build
asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
asc builds remove-groups --build "BUILD_ID" --group "GROUP_ID"
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

# Generate one-time use offer codes
asc offer-codes generate --offer-code "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01"

# Download one-time use offer codes to a file
asc offer-codes values --id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.txt"
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

# Update metadata for a locale
asc app-info set --app "123456789" --locale "en-US" --whats-new "Bug fixes"
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

Validate and migrate metadata between ASC's `.strings` format and Fastlane directory structure.

```bash
# Validate metadata against App Store Connect character limits (offline)
asc migrate validate --fastlane-dir ./metadata

# Import metadata from fastlane format to App Store Connect
asc migrate import --app "123456789" --fastlane-dir ./metadata

# Export metadata from App Store Connect to fastlane format
asc migrate export --app "123456789" --output ./exported-metadata
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
| JSON (minified) | default | AI agents, scripting |
| Table | `--output table` | Humans in terminal |
| Markdown | `--output markdown` | Humans, documentation |

Note: When using `--paginate`, the response `links` field is cleared to avoid confusion about additional pages.

### Authentication

```bash
# Check authentication status
asc auth status
asc auth status --verbose
asc auth status --validate

# Diagnose authentication issues
asc auth doctor
asc auth doctor --output json
asc auth doctor --fix --confirm

# Logout
asc auth logout
```

## Design Philosophy

### Explicit Over Cryptic

```bash
# Good - self-documenting
asc reviews --app "MyApp" --stars 1

# Avoid - cryptic flags (hypothetical, not supported)
# asc reviews -a "MyApp" -s 1
```

### AI-Agent Friendly

All commands output minified JSON by default for easy parsing by AI agents:

```bash
asc feedback --app "123456789" | jq '.data[].attributes.comment'
```

JSON output is minified (one line per response) by default. Use `--output table` or `--output markdown` for human-readable output.

### No Interactive Prompts

Everything is flag-based for automation:

```bash
# Non-interactive (good for CI/CD and AI)
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

- [CLAUDE.md](CLAUDE.md) - Development guidelines for AI assistants
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [docs/openapi/README.md](docs/openapi/README.md) - Offline OpenAPI snapshot + update steps

## Security

- Credentials stored in the system keychain when available
- Local config fallback with restricted permissions
- Private key content never stored, only path reference
- Environment variables as fallback

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

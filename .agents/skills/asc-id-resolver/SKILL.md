---
name: asc-id-resolver
description: Resolve App Store Connect IDs (apps, builds, app and digital-goods versions, groups, testers) from human-friendly names using asc. Use when commands require IDs.
---

# asc id resolver

Use this skill to map names to IDs needed by other commands.

## App ID
- By bundle ID or name:
  - `asc apps list --bundle-id "com.example.app"`
  - `asc apps list --name "My App"`
- Fetch everything:
  - `asc apps list --paginate`
- Set default:
  - `ASC_APP_ID=...`

## Build ID
- Latest build:
  - `asc builds info --app "APP_ID" --latest --version "1.2.3" --platform IOS`
- Recent builds:
  - `asc builds list --app "APP_ID" --sort -uploadedDate --limit 5`

## Version ID
- `asc versions list --app "APP_ID" --paginate`

## Digital-goods version IDs

API 4.4.1 version IDs are distinct from IAP product, subscription, and
subscription-group IDs:

Resolve the owning resources first:

- IAPs: `asc iap list --app "APP_ID" --paginate --output json`
- Subscription groups: `asc subscriptions groups list --app "APP_ID" --paginate --output json`
- Subscriptions: `asc subscriptions list --app "APP_ID" --paginate --output json`
  or `asc subscriptions list --group-id "GROUP_ID" --paginate --output json`

Then resolve their version IDs:

- IAP versions: `asc iap versions list --iap-id "IAP_ID" --paginate --output json`
- Subscription versions: `asc subscriptions versions list --subscription-id "SUB_ID" --paginate --output json`
- Subscription group versions: `asc subscriptions groups versions list --group-id "GROUP_ID" --paginate --output json`

Resolve child localization or image IDs from the corresponding version subtree:

- IAP localizations: `asc iap versions localizations list --version-id "VERSION_ID" --paginate --output json`
- IAP primary image: `asc iap versions image --version-id "VERSION_ID" --output json`
- IAP image collection: `asc iap versions images list --version-id "VERSION_ID" --paginate --output json`
- Subscription localizations: `asc subscriptions versions localizations list --version-id "VERSION_ID" --paginate --output json`
- Subscription primary image: `asc subscriptions versions images primary --version-id "VERSION_ID" --output json`
- Subscription image collection: `asc subscriptions versions images list --version-id "VERSION_ID" --paginate --output json`
- Subscription-group localizations: `asc subscriptions groups versions localizations list --version-id "VERSION_ID" --paginate --output json`

## TestFlight IDs
- Groups:
  - `asc testflight groups list --app "APP_ID" --paginate`
- Testers:
  - `asc testflight testers list --app "APP_ID" --paginate`

## Pre-release version IDs
- `asc testflight pre-release list --app "APP_ID" --platform IOS --paginate`

## Review submission IDs
- `asc review submissions list --app "APP_ID" --paginate`

## Output tips
- Output defaults are TTY-aware: table in a terminal and minified JSON in pipes
  or CI. An explicit `--output` wins.
- Use explicit `--output json` for automation and add `--pretty` only for
  human-readable JSON.
- For human viewing, use `--output table` or `--output markdown`.

## Guardrails
- Prefer `--paginate` on list commands to avoid missing IDs.
- Use `--sort` where available to make results deterministic.

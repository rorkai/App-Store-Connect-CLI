---
name: asc-metadata-sync
description: Sync, validate, and apply App Store metadata with the current asc canonical metadata workflow. Use when updating metadata, localizations, keywords, or migrating legacy fastlane metadata.
---

# asc metadata sync

Use this skill to keep App Store metadata in sync with App Store Connect. Prefer the canonical `asc metadata` workflow for app-info and version localization fields. Use the lower-level `asc localizations` and `asc migrate` commands only when the user specifically needs `.strings` files or legacy fastlane-format metadata.

## Current canonical workflow

### 1. Pull canonical metadata

```bash
asc metadata pull --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
```

If the app has multiple app-info records, resolve the app-info ID first and pass it explicitly:

```bash
asc apps info list --app "APP_ID" --output table
asc metadata pull --app "APP_ID" --app-info "APP_INFO_ID" --version "1.2.3" --platform IOS --dir "./metadata"
```

### 2. Edit local files

Canonical files are written under:

- `metadata/app-info/<locale>.json` for app-level fields: `name`, `subtitle`, `privacyPolicyUrl`, `privacyChoicesUrl`, `privacyPolicyText`
- `metadata/version/<version>/<locale>.json` for version fields: `description`, `keywords`, `marketingUrl`, `promotionalText`, `supportUrl`, `whatsNew`

Copyright is not a localization field. Manage it with:

```bash
asc versions update --version-id "VERSION_ID" --copyright "2026 Your Company"
```

### 3. Validate before upload

```bash
asc metadata validate --dir "./metadata" --output table
```

For subscription apps, include the extra Terms of Use / EULA heuristic:

```bash
asc metadata validate --dir "./metadata" --subscription-app --output table
```

### 4. Preview and apply

Run a dry run first:

```bash
asc metadata push --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --dry-run --output table
```

Apply after the plan looks correct:

```bash
asc metadata push --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
```

Use `asc metadata apply` when the user wants the apply-named command shape for the same canonical files:

```bash
asc metadata apply --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --dry-run
asc metadata apply --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
```

For a review-artifact workflow with explicit approval before mutation, use the metadata review commands introduced in `asc` 2.6.1:

```bash
asc metadata plan --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --review-dir ".asc/metadata/review"
asc metadata approve --review-dir ".asc/metadata/review" --all
asc metadata status --review-dir ".asc/metadata/review" --output table
asc metadata apply --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --review-dir ".asc/metadata/review" --confirm
```

Use `asc metadata approve --key "version:1.2.3:en-US:whatsNew"` or `--scope app-info,version` when the user wants selective approval artifacts before the guarded apply. Version-scoped keys include the App Store version string.

## Keyword-only workflow

Use this when only the version-localization `keywords` field should change:

```bash
asc metadata keywords diff --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
asc metadata keywords apply --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --confirm
```

For importing keyword research:

```bash
asc metadata keywords import --dir "./metadata" --version "1.2.3" --locale "en-US" --input "./keywords.csv"
asc metadata keywords sync --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --input "./keywords.csv"
```

## Quick field updates

For one-off version-localization edits, pass an explicit version selector. Use `--version-id` for deterministic updates when you already have it, or `--version` plus `--platform` when working from a version string.

```bash
asc apps info edit --app "APP_ID" --version-id "VERSION_ID" --locale "en-US" --whats-new "Bug fixes and improvements"
asc apps info edit --app "APP_ID" --version "1.2.3" --platform IOS --locale "en-US" --description "Your app description here"
asc apps info edit --app "APP_ID" --version "1.2.3" --platform IOS --locale "en-US" --keywords "keyword1,keyword2,keyword3"
asc apps info edit --app "APP_ID" --version "1.2.3" --platform IOS --locale "en-US" --support-url "https://support.example.com"
```

For app-info fields, prefer the post-create setup command:

```bash
asc app-setup info set --app "APP_ID" --primary-locale "en-US" --privacy-policy-url "https://example.com/privacy"
asc app-setup info set --app "APP_ID" --locale "en-US" --name "Your App Name" --subtitle "Your subtitle"
```

## Lower-level localization files

Use `.strings` files when the user specifically wants import/export files instead of canonical JSON:

```bash
asc localizations list --version "VERSION_ID" --output table
asc localizations download --version "VERSION_ID" --path "./localizations"
asc localizations upload --version "VERSION_ID" --path "./localizations" --dry-run
asc localizations upload --version "VERSION_ID" --path "./localizations"
```

For app-info localizations:

```bash
asc apps info list --app "APP_ID" --output table
asc localizations list --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --output table
asc localizations download --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations"
asc localizations upload --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations" --dry-run
asc localizations upload --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations"
```

## Legacy fastlane metadata

Use this only for existing fastlane-format trees:

```bash
asc migrate export --app "APP_ID" --version-id "VERSION_ID" --output-dir "./fastlane"
asc migrate validate --fastlane-dir "./fastlane"
asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir "./fastlane" --dry-run
asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir "./fastlane" --confirm
```

Deliverfile `metadata_path` and `screenshots_path` values take precedence and resolve relative to the Deliverfile. With `--fastlane-dir "./fastlane"`, use `metadata_path "./metadata"` rather than `"./fastlane/metadata"`; the latter resolves to `./fastlane/fastlane/metadata`. Fix stale values in the Deliverfile, or remove them to use the conventional `metadata/` and `screenshots/` directories.

Paths outside the selected Fastlane directory fail unless the operator explicitly trusts them with `--allow-external-metadata` or `--allow-external-screenshots`. Keep those flags off for untrusted imports.

Inspect the validation body as well as the process exit: `asc migrate validate` can return a report with `valid: false` and a nonzero `errorCount` while exiting 0. A confirmed import can print `status: "partial"` with completed stages and failure details while exiting nonzero, so non-empty stdout does not mean success.

## Character limits

| Field | Limit |
|-------|-------|
| Name | 30 |
| Subtitle | 30 |
| Keywords | 100 comma-separated characters |
| Description | 4000 |
| What's New | 4000 |
| Promotional Text | 170 |

## Agent behavior

- Start with `asc metadata pull` unless the user specifically asks for `.strings` or fastlane metadata.
- Always run `asc metadata validate` before remote writes.
- Preview remote changes with `--dry-run` when the command supports it.
- Use `asc metadata plan` plus `approve`/`status` when the user wants a durable review artifact before apply.
- For quick edits, always pass `--version-id` or `--version` plus `--platform`; do not rely on ambiguous latest-version behavior.
- Keep app-info fields and version fields separate.
- Use `--output table` for human verification and JSON for automation.

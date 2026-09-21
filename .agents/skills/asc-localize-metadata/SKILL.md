---
name: asc-localize-metadata
description: Automatically translate and sync App Store metadata (description, keywords, what's new, subtitle) to multiple languages using LLM translation and asc CLI. Use when asked to localize an app's App Store listing, translate app descriptions, or add new languages to App Store Connect.
---

# asc localize metadata

Use this skill to pull English (or any source locale) App Store metadata, translate it with LLM, and push translations back to App Store Connect — all automated.

## Command discovery and output conventions

- Always confirm flags with `--help` for the exact `asc` version:
  - `asc localizations --help`
  - `asc localizations download --help`
  - `asc localizations upload --help`
  - `asc apps info edit --help`
- Prefer explicit long flags (`--app`, `--version`, `--version-id`, `--type`, `--app-info`).
- Output defaults are TTY-aware: table in interactive terminals and JSON in CI or other non-interactive contexts. Use explicit `--output` when the format matters.
- Prefer deterministic ID-based operations. Do not "pick the first row" via `head -1` unless the user explicitly agrees.

## Preconditions
- Auth configured (`asc auth login` or `ASC_*` env vars)
- Know your app ID (`asc apps list` to find it)
- At least one locale (typically en-US) already has metadata in App Store Connect

## Supported Locales

App Store Connect locales for version and app-info localizations:
```
ar-SA, bn-BD, ca, cs, da, de-DE, el, en-AU, en-CA, en-GB,
en-US, es-ES, es-MX, fi, fr-CA, fr-FR, gu-IN, he, hi, hr,
hu, id, it, ja, kn-IN, ko, ml-IN, mr-IN, ms, nl-NL, no,
or-IN, pa-IN, pl, pt-BR, pt-PT, ro, ru, sk, sl-SI, sv,
ta-IN, te-IN, th, tr, uk, ur-PK, vi, zh-Hans, zh-Hant
```

## Two Types of Metadata

### Version Localizations (per-release)
Fields: `description`, `keywords`, `whatsNew`, `supportUrl`, `marketingUrl`, `promotionalText`

### App Info Localizations (app-level, persistent)
Fields: `name`, `subtitle`, `privacyPolicyUrl`, `privacyChoicesUrl`, `privacyPolicyText`

## Workflow

### Step 1: Resolve IDs

```bash
# Find app ID
asc apps list --output table

# Find latest version ID
asc versions list --app "APP_ID" --state READY_FOR_DISTRIBUTION --output table
# or for editable version:
asc versions list --app "APP_ID" --state PREPARE_FOR_SUBMISSION --output table

# Find app info ID (for app-level fields like name/subtitle)
asc apps info list --app "APP_ID" --output table
```

Notes:
- Version-localization fields (description, keywords, whatsNew, etc.) are per-version.
- App-info fields (name, subtitle, privacy URLs/text) are app-level and use `--type app-info`.
- If you only have names (app name, version string) and need IDs deterministically, use `asc-id-resolver`.

### Step 2: Download source locale

```bash
# Download version localizations to local .strings files
# (description, keywords, whatsNew, promotionalText, supportUrl, marketingUrl, ...)
asc localizations download --version "VERSION_ID" --path "./localizations"

# Download app-info localizations to local .strings files
# (name, subtitle, privacyPolicyUrl, privacyChoicesUrl, privacyPolicyText, ...)
asc localizations download --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations"
```

This creates files like `./localizations/en-US.strings` and `./app-info-localizations/en-US.strings`. If download is unavailable, read fields individually:

```bash
# List version localizations to see existing locales and their content
asc localizations list --version "VERSION_ID" --output table
```

### Step 3: Translate with LLM

For each target locale, translate the source text. Follow these rules:

#### Translation Guidelines
- **Tone & Register**: Always use formal, polite language. Use formal "you" forms where the language distinguishes them (Russian: «вы», German: «Sie», French: «vous», Spanish: «usted», Dutch: «u», Italian: «Lei», Portuguese: «você» formal, etc.). App Store descriptions are professional marketing copy — never use casual or informal register.
- **description**: Translate naturally, adapt tone to local market. Keep formatting (line breaks, bullet points, emoji). Stay within 4000 chars.
- **keywords**: Do NOT literally translate. Research what users in that locale would search for. Comma-separated, max 100 chars total. No duplicates, no app name (Apple adds it automatically).
- **whatsNew**: Translate release notes. Keep it concise. Max 4000 chars.
- **promotionalText**: Translate marketing hook. Max 170 chars. This can be updated without a new version.
- **subtitle**: Translate or adapt tagline. Max 30 chars — this is very tight, may need creative adaptation.
- **name**: Usually keep the original app name. Only translate if the user explicitly asks. Max 30 chars.

#### LLM Translation Prompt Template

For each target locale, use this approach:

```
Translate the following App Store metadata from {source_locale} to {target_locale}.

Rules:
- description: Natural, fluent translation. Preserve formatting (line breaks, bullets, emoji). Max 4000 chars.
- keywords: Do NOT literally translate. Choose keywords native speakers would search for in the App Store. Comma-separated, max 100 chars total. Do not include the app name.
- whatsNew: Translate release notes naturally. Max 4000 chars.
- promotionalText: Translate marketing tagline. Max 170 chars.
- subtitle: Adapt tagline creatively to fit 30 chars max.
- name: Keep the original app name unless explicitly requested to translate it. Max 30 chars.
- Use formal, polite language and formal "you" forms (Russian: вы, German: Sie, French: vous, Spanish: usted, Dutch: u, etc.). App Store copy is professional marketing — never use informal register.
- Respect cultural context. A playful tone in English may need adjustment for formal markets (e.g., ja, de-DE).

Source ({source_locale}):
description: """
{description}
"""

keywords: {keywords}

whatsNew: """
{whatsNew}
"""

promotionalText: {promotionalText}

name: {name}

subtitle: {subtitle}
```

### Step 4: Upload translations

#### Option A: Via .strings files (bulk)

Create a `.strings` file per locale in the appropriate directory.

Version localization example:

```
// nl-NL.strings
"description" = "Je app-beschrijving hier";
"keywords" = "wiskunde,kinderen,tafels,leren";
"whatsNew" = "Bugfixes en verbeteringen";
"promotionalText" = "Leer de tafels van vermenigvuldiging!";
```

Then upload version localizations:
```bash
asc localizations upload --version "VERSION_ID" --path "./localizations"
```

App-info localization example:

```
// nl-NL.strings
"subtitle" = "Leer tafels spelenderwijs";
```

Then upload app-info localizations:
```bash
asc localizations upload --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations"
```

#### Option B: Via individual commands (fine control)

```bash
# Version localization fields (fine control).
# Prefer passing the explicit version ID for determinism.
asc apps info edit --app "APP_ID" --version-id "VERSION_ID" --locale "nl-NL" \
  --description "Je beschrijving..." \
  --keywords "wiskunde,kinderen,tafels" \
  --whats-new "Bugfixes en verbeteringen"
```

For app-level fields:
```bash
# Subtitle/name (app-info localization) is managed via app-info localizations.
# Use the app-info localization .strings + upload flow; there is no app-infos localizations command.
#
# 1) Edit: ./app-info-localizations/nl-NL.strings
# "subtitle" = "Leer tafels spelenderwijs";
#
# 2) Upload:
asc localizations upload --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --path "./app-info-localizations"
```

### Step 5: Verify

```bash
# Check all locales are present
asc localizations list --version "VERSION_ID" --output table

# Check app info localizations
asc localizations list --app "APP_ID" --type app-info --app-info "APP_INFO_ID" --output table
```

## Character Limits (enforce before upload!)

| Field | Limit |
|-------|-------|
| Name | 30 |
| Subtitle | 30 |
| Keywords | 100 (comma-separated) |
| Description | 4000 |
| What's New | 4000 |
| Promotional Text | 170 |

**Always validate** translated text fits within limits before uploading. Truncated text looks unprofessional. If translation exceeds the limit, shorten it — do not truncate mid-sentence.

## Full Example: Add nl-NL and ru to Roxy Math

```bash
# 1) Resolve IDs deterministically (do not auto-pick the "first" row)
# If you only have names, use asc-id-resolver skill.
asc apps list --output table
APP_ID="APP_ID_HERE"

asc versions list --app "$APP_ID" --state PREPARE_FOR_SUBMISSION --output table
VERSION_ID="VERSION_ID_HERE"

asc apps info list --app "$APP_ID" --output table
APP_INFO_ID="APP_INFO_ID_HERE"

# 2) Download English source (or your chosen source locale)
asc localizations download --version "$VERSION_ID" --path "./localizations"
asc localizations download --app "$APP_ID" --type app-info --app-info "$APP_INFO_ID" --path "./app-info-localizations"

# 3) Read en-US.strings, translate to nl-NL and ru (LLM step)

# 4) Write nl-NL.strings and ru.strings to:
#    - ./localizations/ (version localization fields)
#    - ./app-info-localizations/ (subtitle/name/privacy fields)

# 5) Upload all
asc localizations upload --version "$VERSION_ID" --path "./localizations"
asc localizations upload --app "$APP_ID" --type app-info --app-info "$APP_INFO_ID" --path "./app-info-localizations"

# 6) Verify
asc localizations list --version "$VERSION_ID" --output table
asc localizations list --app "$APP_ID" --type app-info --app-info "$APP_INFO_ID" --output table
```

## Agent Behavior

1. **Always start by reading the source locale** — never translate from memory or assumptions.
2. **Check existing localizations first** — don't overwrite existing translations unless the user asks to update them.
3. **Version vs app-info is different** — version fields live under `--version "VERSION_ID"`; subtitle/name/privacy live under `--app ... --type app-info`.
4. **Prefer deterministic IDs** — do not select IDs via `head -1` unless explicitly requested; use `--output table` for selection or `asc-id-resolver`.
5. **Validate character limits** before uploading. Count characters for each field. If over limit, re-translate shorter.
6. **Keywords are special** — do not literally translate. Research locale-appropriate search terms. Think like a user searching the App Store in that language.
7. **Show the user translations before uploading** — present a summary table of all fields × locales for approval. Do not push without confirmation.
8. **Process one locale at a time** if translating many languages — easier to review and catch errors.
9. **If upload fails** for a locale, log the error, continue with other locales, report all failures at the end.
10. **For updates to existing localizations** — download current, show diff of what will change, get approval, then upload.

## Notes
- Version localizations are tied to a specific version. Create the version first if it doesn't exist.
- `promotionalText` can be updated anytime without a new version submission.
- `whatsNew` is only relevant for updates, not the first version.
- Use `asc-id-resolver` skill if you only have app/version names instead of IDs.
- Use `asc-metadata-sync` skill for non-translation metadata operations.
- For subscription/IAP display name localization, use `asc-subscription-localization` skill instead.

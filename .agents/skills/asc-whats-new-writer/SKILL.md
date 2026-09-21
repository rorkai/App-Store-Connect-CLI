---
name: asc-whats-new-writer
description: Generate engaging, localized App Store release notes (What's New) from git log, bullet points, or free text using canonical metadata under `./metadata`. Optionally pairs with promotional text updates.
---

# asc What's New Writer

Generate engaging, localized release notes from flexible input. Optionally pair with promotional text updates.

## Preconditions

- Metadata pulled locally into canonical files via `asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"`. OR: user provides keywords manually.
- Auth configured for upload (`asc auth login` or `ASC_*` env vars).
- The **primary locale** is `en-US` unless the user specifies otherwise.

## Before You Start

1. Read `references/release_notes_guidelines.md` for tone, structure, and examples.
2. Identify the **latest version directory** under `metadata/version/` (highest semver). Use this for all metadata reads.
3. Enumerate **existing locales** by listing the JSON files in that version directory.

## Phase 1: Gather Input

Accept one of three input modes (auto-detect):

### Git Log

Parse commits since the last tag:

```bash
# Find latest tag
git describe --tags --abbrev=0

# List commits since that tag
git log $(git describe --tags --abbrev=0)..HEAD --oneline --no-merges
```

Filter out noise: merge commits, dependency bumps, CI changes, formatting-only commits. Extract user-facing changes.

### Bullet Points

User provides rough bullets like:
- "improved search"
- "fixed crash on launch"
- "added sleep timer"

### Free Text

User describes changes conversationally:
> "We made search faster, fixed that annoying crash when you open the app, and added a sleep timer feature"

The skill extracts and structures the changes from the text.

### No Input Provided

Prompt the user: "What changed in this release? You can paste git log output, bullet points, or just describe the changes."

## Phase 2: Draft Notes (Primary Locale)

### Step 1: Classify Changes

Group changes into sections per the guidelines:
- **New** — new features or capabilities
- **Improved** — enhancements to existing features
- **Fixed** — bug fixes users would notice

Omit empty sections. If all changes are fixes, only show "Fixed."

### Step 2: Write Benefit-Focused Copy

Follow the tone rules from `references/release_notes_guidelines.md`:
- Describe user impact, not implementation details
- Use direct address ("you") and action verbs
- Be specific — mention concrete improvements

### Step 3: Front-Load the Hook

The first ~170 characters are the only visible part before "more." Lead with the single most impactful change in a complete, compelling sentence.

### Step 4: Echo Keywords for Conversion

1. Read `keywords` from `metadata/version/{latest}/{primary-locale}.json`
   - These canonical files are also what `asc metadata keywords ...` reads and writes.
2. If the field is empty or missing, skip this step
3. Identify keywords relevant to the changes being described
4. Weave them naturally into the notes — never force or stuff

### Step 5: Respect Character Limits

- Keep total length between 500-1500 characters in the primary locale
- This leaves room for localized expansions (some languages expand 30-40%)
- Hard limit: 4,000 characters

### Step 6: Optionally Draft Promotional Text

If the user wants it, draft a 170-char promotional text that:
- Summarizes the update's theme in one punchy line
- Can reference seasonal events
- Is updatable without a new submission

### Present Draft

Show the draft to the user with character count. Wait for approval before localizing.

## Phase 3: Localize

Translate the approved notes to all existing locales.

### Translation Rules

- Use formal register and formal "you" forms (Russian: вы, German: Sie, French: vous, Spanish: usted, Dutch: u, Italian: Lei)
- Adapt tone to local market — playful English may need adjustment for formal markets (ja, de-DE)
- Do NOT literally translate idioms — adapt them to local equivalents
- A playful tone in English may need to be more respectful or formal in other cultures

### Locale-Specific Keyword Echo

For each locale:
1. Read `keywords` from `metadata/version/{latest}/{locale}.json`
2. Echo locale-specific keywords naturally in the translated notes
3. If keywords field is empty, skip echo for that locale

### Validate

- All translations must be ≤ 4,000 characters
- Promotional text must be ≤ 170 characters per locale
- If a translation exceeds the limit, shorten it — never truncate mid-sentence

## Phase 4: Review & Upload

### Step 1: Present Summary

Show a table of all locales with their notes and character counts:

```
| Locale | What's New (first 80 chars...) | Chars | Promo Text | Chars |
|--------|-------------------------------|-------|------------|-------|
| en-US  | Search just got faster — ...   | 847   | New sleep… | 142   |
| ar-SA  | البحث أصبح أسرع — ...           | 923   | نوم جديد…  | 138   |
| ...    | ...                           | ...   | ...        | ...   |
```

### Step 2: Wait for Approval

Do not upload without user confirmation.

### Step 3: Upload

Upload via `asc` (verify exact syntax with `asc --help`):

```bash
# Individual locale direct update
asc apps info edit --app "APP_ID" --version-id "VERSION_ID" --locale "en-US" --whats-new "Your release notes here"

# Bulk canonical-metadata push after writing ./metadata/version/<version>/<locale>.json
asc metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata" --dry-run
asc metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata"
```

If promotional text was drafted, either include `--promotional-text "..."` in the direct update command or write `promotionalText` into the canonical JSON before `asc metadata push`.

### Step 4: Handle Failures

On partial upload failure:
- Report which locales succeeded and which failed
- Offer to retry failed locales

## Metadata File Paths

- **Keywords:** `metadata/version/{latest-version}/{locale}.json` → `keywords` field
- **Current What's New:** `metadata/version/{latest-version}/{locale}.json` → `whatsNew` field
- **Latest version:** highest semver directory under `metadata/version/`
- The canonical `./metadata` tree is what `asc metadata pull`, `asc metadata push`, and `asc metadata keywords ...` operate on.
- Follows the same metadata resolution conventions as `asc-aso-audit`

## Notes

- What's New is **not indexed** for App Store search — write for humans, not algorithms.
- Promotional text is the only metadata field updatable without a new submission.
- The 170-char visible window is the most important part of your release notes.
- Each app update triggers algorithm re-evaluation — the act of updating matters, even if the text doesn't affect ranking.
- Ideal update cadence: every 2-4 weeks.
- For full metadata translation (all fields), use `asc-localize-metadata` instead.
- For keyword research and optimization, use `asc-aso-audit` first.
- If the local keyword field is stale before drafting, refresh it with `asc metadata pull` or inspect planned keyword changes with `asc metadata keywords diff`.

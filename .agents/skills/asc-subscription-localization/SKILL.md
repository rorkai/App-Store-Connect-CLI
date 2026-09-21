---
name: asc-subscription-localization
description: Bulk-localize subscription, subscription-group, and in-app purchase display names across App Store locales using asc, including API 4.4.1 version-scoped v2 resources. Use when filling or updating subscription/IAP names and descriptions without App Store Connect UI work.
---

# asc subscription localization

Use this skill to bulk-create or bulk-update display names and, where supported,
descriptions for subscriptions, subscription groups, and in-app purchases across
all App Store Connect locales. This eliminates the tedious manual process of
clicking through each language in App Store Connect to set the same display
name.

## Preconditions
- Auth configured (`asc auth login` or `ASC_*` env vars).
- Know your app ID (`ASC_APP_ID` or `--app`).
- Subscription groups and subscriptions already exist.

## Choose the API scope first

API 4.4.1 adds discrete versions for IAPs, subscriptions, and subscription
groups. A version ID is different from its product, subscription, or group ID.

- Use `asc ... versions localizations ...` for all new localization work.
- Do not use the product- or group-scoped v1 localization commands. API 4.4.1
  deprecates those resources, and the CLI now emits migration warnings for
  their compatibility commands.
- Never pass a product, subscription, or group ID to a version-scoped command.

Resolve or create versions before localizing them:

```bash
asc iap versions list --iap-id "IAP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output table
asc subscriptions versions list --subscription-id "SUB_ID" --state PREPARE_FOR_SUBMISSION --paginate --output table
asc subscriptions groups versions list --group-id "GROUP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output table
```

Branch independently on each list result: zero matches means create, one means
reuse that version ID, and more than one means stop and require an explicit
version ID. Run each command below only for its zero-match branch:

```bash
# If and only if the IAP version list returned zero matches:
asc iap versions create --iap-id "IAP_ID" --output json
# If and only if the subscription version list returned zero matches:
asc subscriptions versions create --subscription-id "SUB_ID" --output json
# If and only if the group version list returned zero matches:
asc subscriptions groups versions create --group-id "GROUP_ID" --output json
```

None of the three version families has a version delete command. List and
reuse the single `PREPARE_FOR_SUBMISSION` version; create only for zero matches,
and stop for an explicit ID when multiple matches exist. Live parent deletion
did not cascade IAP or subscription versions, so do not assume parent deletion
cleans up versions created for testing.

## Supported App Store Locales

These are the locales supported by App Store Connect for subscription and IAP localizations:

```
ar-SA, ca, cs, da, de-DE, el, en-AU, en-CA, en-GB, en-US,
es-ES, es-MX, fi, fr-CA, fr-FR, he, hi, hr, hu, id, it,
ja, ko, ms, nl-NL, no, pl, pt-BR, pt-PT, ro, ru, sk,
sv, th, tr, uk, vi, zh-Hans, zh-Hant
```

## Workflow: Bulk-localize a subscription version (v2)

List existing localizations, create only missing locales, then verify:

```bash
asc subscriptions versions localizations list --version-id "VERSION_ID" --paginate --output table
asc subscriptions versions localizations create --version-id "VERSION_ID" --locale "LOCALE" --name "Display Name" --description "Description"
asc subscriptions versions localizations list --version-id "VERSION_ID" --paginate --output table
```

Updates distinguish omitted values, non-empty strings, and JSON `null`:

```bash
asc subscriptions versions localizations update --id "LOC_ID" --name "New Name" --description "Updated description"
```

Do not combine a value flag with its matching `--clear-name` or
`--clear-description` flag. The 4.4.1 schema permits JSON `null`, but Apple's
live service currently rejects both an empty `--description` and
`--clear-description` for subscription-version localizations because the
description must contain at least one character.

Creating a missing subscription-version localization therefore requires a
non-empty description. A display-name-only run may update the name of an
existing localization, but must not create a missing locale until the user
provides a non-empty locale-specific or shared fallback description.

## Workflow: Bulk-localize a subscription group version (v2)

```bash
asc subscriptions groups versions localizations list --version-id "VERSION_ID" --paginate --output table
asc subscriptions groups versions localizations create --version-id "VERSION_ID" --locale "LOCALE" --name "Group Display Name" --custom-app-name "My App"
asc subscriptions groups versions localizations update --id "LOC_ID" --name "Updated Group Display Name" --custom-app-name "My App"
asc subscriptions groups versions localizations list --version-id "VERSION_ID" --paginate --output table
```

Clearing metadata is a separate, explicit opt-in operation. After confirming
that the user wants to remove an existing custom app name, run:

```bash
asc subscriptions groups versions localizations update --id "LOC_ID" --clear-custom-app-name
```

Do not include `--clear-name` or `--clear-custom-app-name` in the standard bulk
localization workflow. Use either flag only when JSON `null` is deliberately
intended; omitting the flag leaves that attribute unchanged.

## Workflow: Bulk-localize an IAP version (v2)

```bash
asc iap versions localizations list --version-id "VERSION_ID" --paginate --output table
asc iap versions localizations create --version-id "VERSION_ID" --locale "LOCALE" --name "Display Name" --description "Description"
asc iap versions localizations update --localization-id "LOC_ID" --description "Updated description"
asc iap versions localizations list --version-id "VERSION_ID" --paginate --output table
```

Apple's live service also rejects empty descriptions and
`--clear-description` for IAP-version localizations even though the 4.4.1
schema permits JSON `null`. As with subscriptions, a display-name-only run may
update existing IAP localizations but must not create missing locales until a
non-empty description is available.

## Bulk-localize all subscription versions in an app

For a full app with multiple subscription groups and subscriptions:

```bash
# 1. List groups and resolve each group's unique mutable version.
asc subscriptions groups list --app "APP_ID" --paginate --output json
asc subscriptions groups versions list --group-id "GROUP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json

# 2. Localize each group version.
asc subscriptions groups versions localizations list --version-id "GROUP_VERSION_ID" --paginate --output json
asc subscriptions groups versions localizations create --version-id "GROUP_VERSION_ID" --locale "LOCALE" --name "Group Display Name"

# 3. List subscriptions and resolve each subscription's unique mutable version.
asc subscriptions list --group-id "GROUP_ID" --paginate --output json
asc subscriptions versions list --subscription-id "SUB_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json

# 4. Localize each subscription version.
asc subscriptions versions localizations list --version-id "SUBSCRIPTION_VERSION_ID" --paginate --output json
asc subscriptions versions localizations create --version-id "SUBSCRIPTION_VERSION_ID" --locale "LOCALE" --name "Display Name" --description "Description"
```

Apply the same zero/one/many rule to each version list: create a version for
zero matches, reuse the one match, and stop for an explicit ID when multiple
matches are returned.

## Agent Behavior

- Use only version-scoped v2 resources for new localization work.
- Keep version IDs separate from product, subscription, and group IDs.
- Always list existing localizations first to avoid duplicate creation errors.
- Create a missing locale, update the resolved localization ID when existing
  values differ, and do nothing when the existing values already match.
- When the user provides a single display name, use it for all locales (same name everywhere).
- When the user provides translated names per locale, use the locale-specific name for each.
- For subscription- and IAP-version localizations, require a non-empty
  `--description` on every create. If the user supplies only display names,
  update existing localizations by resolved ID, skip creates for missing
  locales, and ask for locale-specific descriptions or one non-empty fallback
  description before creating them.
- On updates to existing subscription- or IAP-version localizations, omit
  `--description` unless the user supplied a new non-empty value; never infer an
  empty value or clear it.
- Subscription-group-version localizations do not have a description field.
  Create or update their names normally, with `--custom-app-name` only when the
  user supplied that value.
- Use `--output table` for verification steps so the user can visually confirm.
- Use explicit `--output json` for intermediate automation steps; output
  defaults are TTY-aware.
- After bulk writes, always run the list command to verify completeness.
- For apps with many subscriptions, process them sequentially per group to keep output readable.
- If a create or update call fails for a locale, log the locale and error, then
  continue with the remaining locales. After the batch completes, report all
  failures together so the user can address them.

## Notes
- Subscription display names are what users see on the subscription management sheet and in purchase dialogs.
- Creating a localization for a locale that already exists will fail; list
  first and update the resolved ID when a change is needed.
- There is no bulk API; each locale requires a separate create call.
- Use `--paginate` on list commands to ensure all existing localizations are returned.
- Use the `asc-id-resolver` skill if you only have app names instead of IDs.

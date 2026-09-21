---
name: asc-ppp-pricing
description: Set territory-specific pricing for subscriptions and in-app purchases using current asc setup, pricing summary, price import, and price schedule commands. Use when adjusting prices by country or implementing localized PPP strategies.
---

# PPP pricing (per-territory pricing)

Use this skill to create or update localized pricing across territories based on purchasing power parity (PPP) or your own regional pricing strategy.

Prefer the current high-level flows:
- `asc subscriptions setup` and `asc iap setup` for parent creation and pricing,
  followed by version-scoped metadata commands
- `asc subscriptions pricing ...` for subscription pricing changes
- `asc iap pricing summary` and `asc iap pricing schedules ...` for IAP pricing changes

## Preconditions
- Ensure credentials are set (`asc auth login` or `ASC_*` env vars).
- Prefer `ASC_APP_ID` or pass `--app` explicitly.
- Decide your base territory (usually `USA`) and baseline price.
- Use `asc pricing territories list --paginate` if you need supported territory IDs.

## Subscription PPP workflow

### New subscription: bootstrap the parent and price with `setup`
Use `setup` to create the group and subscription, upload the App Review
screenshot, materialize the complete equalized price matrix, and set sale
availability. Create the API 4.4.1 group and subscription versions afterward;
the localization flags on `setup` use deprecated v1 resources and must not be
used for new workflows.

```bash
asc subscriptions setup \
  --app "APP_ID" \
  --group-reference-name "Pro" \
  --reference-name "Pro Monthly" \
  --product-id "com.example.pro.monthly" \
  --subscription-period ONE_MONTH \
  --review-screenshot "./review.png" \
  --price "9.99" \
  --price-territory "USA" \
  --territories "USA,CAN,GBR" \
  --no-verify \
  --output json
```

Capture `.groupId` and `.subscriptionId` from the setup JSON, then add
version-scoped metadata. `--no-verify` is intentional here: without deprecated
v1 localizations, the parent can remain `MISSING_METADATA` until the v2 steps
finish.

```bash
asc subscriptions groups versions list --group-id "GROUP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json
# If and only if the list has zero matches:
asc subscriptions groups versions create --group-id "GROUP_ID" --output json
# For one match, reuse .data[0].id. For more than one, stop and require an explicit GROUP_VERSION_ID.
asc subscriptions groups versions localizations list --version-id "GROUP_VERSION_ID" --paginate --output json
# If and only if en-US is missing:
asc subscriptions groups versions localizations create --version-id "GROUP_VERSION_ID" --locale "en-US" --name "Pro"
# Otherwise, if and only if the resolved en-US name differs:
asc subscriptions groups versions localizations update --id "GROUP_LOC_ID" --name "Pro"
# Otherwise, do nothing.

asc subscriptions versions list --subscription-id "SUB_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json
# If and only if the list has zero matches:
asc subscriptions versions create --subscription-id "SUB_ID" --output json
# For one match, reuse .data[0].id. For more than one, stop and require an explicit SUBSCRIPTION_VERSION_ID.
asc subscriptions versions localizations list --version-id "SUBSCRIPTION_VERSION_ID" --paginate --output json
# If and only if en-US is missing:
asc subscriptions versions localizations create --version-id "SUBSCRIPTION_VERSION_ID" --locale "en-US" --name "Pro Monthly" --description "Unlock everything"
# Otherwise, if and only if the resolved en-US values differ:
asc subscriptions versions localizations update --id "SUBSCRIPTION_LOC_ID" --name "Pro Monthly" --description "Unlock everything"
# Otherwise, do nothing.
asc subscriptions groups versions localizations list --version-id "GROUP_VERSION_ID" --paginate --output table
asc subscriptions versions localizations list --version-id "SUBSCRIPTION_VERSION_ID" --paginate --output table
asc validate subscriptions --app "APP_ID" --output table
```

For each version list, reuse its single `PREPARE_FOR_SUBMISSION` result. Create
only when the result is empty; if more than one result is returned, stop and
require an explicit version ID instead of creating another non-deletable
version. Each localization create/update pair is also conditional: create for
a missing locale, update the resolved localization only when values differ,
and do nothing when it already matches.

Notes:
- `setup` materializes Apple's complete equalized price matrix from the selected
  base price. This split workflow defers final verification until the v2
  localizations exist.
- Outside this split v2 bootstrap, omit `--no-verify` so setup performs its
  normal readback verification.
- Use `--tier` or `--price-point-id` instead of `--price` when your workflow is tier-driven.
- If an existing subscription remains `MISSING_METADATA` with the same selected base price, re-run the setup inputs with `--repair` to atomically rebuild and re-save the matrix.

### Inspect current subscription pricing before changes
Use the summary view first when you want a compact current-state snapshot.

```bash
asc subscriptions pricing summary --subscription-id "SUB_ID" --territory "USA"
asc subscriptions pricing summary --subscription-id "SUB_ID" --territory "IND"
asc subscriptions pricing prices list --subscription-id "SUB_ID" --paginate
```

Use `summary` for quick before/after spot checks and `prices list` when you need raw price records.

### Derive one subscription's localized prices from another
Use `derive` when a target subscription should stay near a fixed multiple of a
source subscription in every territory, such as yearly pricing near 10 times
monthly pricing. Apple ladders scale unevenly across territories, so preview
the selected target points and achieved multiples before applying them.

```bash
asc subscriptions pricing derive \
  --source-subscription-id "MONTHLY_SUB_ID" \
  --target-subscription-id "YEARLY_SUB_ID" \
  --multiplier "10" \
  --round nearest \
  --dry-run \
  --output table
```

Choose how a desired price resolves when Apple does not offer it:

- `exact` fails unless the calculated amount exists on the target ladder.
- `nearest` chooses the closest amount; an exact tie chooses the lower one.
- `up` chooses the smallest available amount at or above the calculation.
- `down` chooses the largest available amount at or below the calculation.

The confirmed command fetches current prices and builds a fresh plan; it does
not reuse the preceding dry-run result. When the applied values must match the
reviewed values, rerun `--dry-run` immediately before confirming, then apply:

```bash
asc subscriptions pricing derive \
  --source-subscription-id "MONTHLY_SUB_ID" \
  --target-subscription-id "YEARLY_SUB_ID" \
  --multiplier "10" \
  --round nearest \
  --confirm \
  --output table
```

The source and target must be distinct subscriptions with existing standard
`UPFRONT` prices. The operation is a one-time snapshot, not a persistent link.
It fails closed before mutation when any territory cannot resolve, skips target
prices that already match, and verifies applied prices by reading them back.
Use `--territory "SWE"` for a focused preview or staged one-territory update;
omit it to derive every current source territory.
Approved or live targets are scheduled for tomorrow by default when no
`--start-date` is supplied because `--auto-start-date` defaults to true. Pass
`--auto-start-date=false` to apply immediately, or use an explicit date when
coordinating a rollout.
The command does not change subscription sale availability.

### Preferred bulk PPP update: import a CSV with dry run
For broad PPP rollouts, prefer the subscription pricing import command instead of manually adding territory prices one by one.

Example CSV:

```csv
territory,price,start_date,preserved
IND,2.99,2026-04-01,false
BRA,4.99,2026-04-01,false
MEX,4.99,2026-04-01,false
DEU,8.99,2026-04-01,false
```

Dry-run first:

```bash
asc subscriptions pricing prices import \
  --subscription-id "SUB_ID" \
  --input "./ppp-prices.csv" \
  --dry-run \
  --output table
```

Apply for real:

```bash
asc subscriptions pricing prices import \
  --subscription-id "SUB_ID" \
  --input "./ppp-prices.csv" \
  --confirm \
  --output table
```

Notes:
- `--dry-run` validates rows and resolves price points without creating prices.
- `--continue-on-error=false` gives you a fail-fast mode.
- CSV required columns: `territory`, `price`
- CSV optional columns: `currency_code`, `start_date`, `preserved`, `preserve_current_price`, `price_point_id`
- When `price_point_id` is omitted, the CLI resolves the matching price point for the row's territory and price automatically.
- Territory inputs in import can be 3-letter IDs, 2-letter codes, or common territory names that map cleanly.

### One-off subscription territory changes
For a small number of manual overrides, use the canonical `set` command.

```bash
asc subscriptions pricing prices set --subscription-id "SUB_ID" --price "2.99" --territory "IND"
asc subscriptions pricing prices set --subscription-id "SUB_ID" --tier 5 --territory "BRA"
asc subscriptions pricing prices set --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID" --territory "DEU"
```

Notes:
- Add `--start-date "YYYY-MM-DD"` to schedule a future change.
- Add `--preserved` when you want to preserve the current price relationship.
- The command handles both initial pricing and later price changes.

### Discover raw price points only when you need them
Use price-point lookup and equalizations when you want to inspect Apple's localized ladder directly or pin exact price point IDs.

```bash
asc subscriptions pricing price-points list --subscription-id "SUB_ID" --territory "USA" --paginate --price "9.99"
asc subscriptions pricing price-points equalizations --price-point-id "PRICE_POINT_ID" --paginate
asc subscriptions pricing price-points adjusted-equalizations --price-point-id "PRICE_POINT_ID" --upfront-price-point-id "UPFRONT_PRICE_POINT_ID" --plan-type MONTHLY --subscription-id "SUB_ID" --paginate
```

Use `equalizations` for Apple's standard localized ladder. Use
`adjusted-equalizations` when you need the API 4.4.1 subscription-specific
adjustments. For a fresh adjusted-equalizations request, pass both
`--upfront-price-point-id` and `--plan-type`; `--subscription-id` and
`--territory` are optional filters. Treat `--next` as an opaque continuation
URL. On a resumed `equalizations` or `adjusted-equalizations` request, pass only
`--next` without the original owner `--price-point-id`, filters, sparse fields,
includes, or limit; the continuation URL already carries that query state.
`--paginate` and explicit output flags may still be used.

### Verify after apply
Re-run the summary and raw list views after changes.

```bash
asc subscriptions pricing summary --subscription-id "SUB_ID" --territory "IND"
asc subscriptions pricing summary --subscription-id "SUB_ID" --territory "BRA"
asc subscriptions pricing prices list --subscription-id "SUB_ID" --paginate
```

After the version metadata exists, you may rerun `asc subscriptions setup` with
verification enabled to recheck the parent, pricing, screenshot, and
availability state.

### Subscription availability
The underlying subscription-availability resource is deprecated in App Store Connect API 4.4. Keep this command family only for compatibility when ordinary upfront territory availability still needs it; Apple does not provide a one-for-one replacement for that case. For Monthly with 12-Month Commitment, use `asc subscriptions pricing monthly-commitment enable|disable|list` instead.

```bash
asc subscriptions pricing availability edit --subscription-id "SUB_ID" --territories "USA,CAN,IND,BRA"
asc subscriptions pricing availability view --subscription-id "SUB_ID"
```

## IAP PPP workflow

### New IAP: bootstrap the parent and price with `setup`
Use `setup` to create the product and initial price schedule. Its localization
flags use the deprecated v1 resource, so add localization through an API 4.4.1
IAP version after setup.

```bash
asc iap setup \
  --app "APP_ID" \
  --type NON_CONSUMABLE \
  --reference-name "Pro Lifetime" \
  --product-id "com.example.pro.lifetime" \
  --price "9.99" \
  --base-territory "USA" \
  --output json
```

Capture `.iapId` from the setup JSON, then create the version and metadata:

```bash
asc iap versions list --iap-id "IAP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json
# If and only if the list has zero matches:
asc iap versions create --iap-id "IAP_ID" --output json
# For one match, reuse .data[0].id. For more than one, stop and require an explicit IAP_VERSION_ID.
asc iap versions localizations list --version-id "IAP_VERSION_ID" --paginate --output json
# If and only if en-US is missing:
asc iap versions localizations create --version-id "IAP_VERSION_ID" --locale "en-US" --name "Pro Lifetime" --description "Unlock everything forever"
# Otherwise, if and only if the resolved en-US values differ:
asc iap versions localizations update --localization-id "IAP_LOC_ID" --name "Pro Lifetime" --description "Unlock everything forever"
# Otherwise, do nothing.
```

Reuse the single `PREPARE_FOR_SUBMISSION` version. Create only when the list is
empty, and stop for an explicit version ID if multiple matches are returned.
Create the localization only when `en-US` is absent, update its resolved ID only
when values differ, and otherwise do nothing.

Notes:
- `setup` verifies the created IAP and price schedule by default; verify the
  version localization with its version-scoped list command.
- Use `--start-date` for scheduled pricing.
- Use `--tier` or `--price-point-id` when you want deterministic tier- or ID-based setup.

### Inspect current IAP pricing before changes
Use `asc iap pricing summary` as the main current-state summary for PPP work.

```bash
asc iap pricing summary --iap-id "IAP_ID" --territory "USA"
asc iap pricing summary --iap-id "IAP_ID" --territory "IND"
```

This returns the base territory, current price, estimated proceeds, and scheduled changes for the requested territory.

### Discover candidate IAP price points
Use price-point lookup when you want to inspect or pin exact price point IDs.

```bash
asc iap pricing price-points list --iap-id "IAP_ID" --territory "USA" --paginate --price "9.99"
asc iap pricing price-points equalizations --id "PRICE_POINT_ID"
```

### Create or update an IAP price schedule
For manual PPP updates, create a price schedule directly.

```bash
asc iap pricing schedules create --iap-id "IAP_ID" --base-territory "USA" --price "4.99" --start-date "2026-04-01"
asc iap pricing schedules create --iap-id "IAP_ID" --base-territory "USA" --tier 5 --start-date "2026-04-01"
asc iap pricing schedules create --iap-id "IAP_ID" --base-territory "USA" --prices "PRICE_POINT_ID:2026-04-01"
```

Use these when you are intentionally creating or replacing schedule entries. For deeper inspection:

```bash
asc iap pricing schedules view --iap-id "IAP_ID"
asc iap pricing schedules manual-prices --schedule-id "SCHEDULE_ID" --paginate
asc iap pricing schedules automatic-prices --schedule-id "SCHEDULE_ID" --paginate
```

### Verify after apply
Use the summary command again after scheduling or applying pricing changes.

```bash
asc iap pricing summary --iap-id "IAP_ID" --territory "USA"
asc iap pricing summary --iap-id "IAP_ID" --territory "IND"
```

For future-dated schedules, expect scheduled changes rather than an immediately updated current price.

## Common PPP strategy patterns

### Base territory first
- Pick one baseline territory, usually `USA`.
- Set the baseline price there first.
- Derive lower or higher territory targets from that baseline.

### Tiered regional pricing
- High-income markets stay close to baseline.
- Mid-income markets get moderate discounts.
- Lower-income markets get stronger PPP adjustments.

### Spreadsheet-driven rollout
- Build the target territory list in a CSV.
- Dry-run the import.
- Fix any resolution failures.
- Apply the import.
- Re-run summary checks for the most important territories.

## Notes
- Prefer canonical commands in docs and automation: `asc subscriptions pricing ...`
- `asc subscriptions pricing ...` is the supported subscription pricing family; do not use the removed `asc subscriptions prices ...` path.
- Prefer canonical IAP commands in docs and automation: `asc iap pricing ...`
- `asc subscriptions pricing prices import --dry-run` is the safest subscription batch PPP path today.
- `asc subscriptions setup` and `asc iap setup` already provide built-in post-create verification.
- There is not yet a single first-class before/after PPP diff command; use the current summary commands before and after apply.
- Price changes may take time to propagate in App Store Connect and storefronts.

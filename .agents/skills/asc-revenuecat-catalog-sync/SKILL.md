---
name: asc-revenuecat-catalog-sync
description: Reconcile App Store Connect subscriptions and in-app purchases with RevenueCat products, entitlements, offerings, and packages using asc and RevenueCat MCP. Use when setting up or syncing subscription catalogs across ASC and RevenueCat.
---

# asc RevenueCat catalog sync

Use this skill to keep App Store Connect (ASC) and RevenueCat aligned, including creating missing ASC items and mapping them to RevenueCat resources.

## When to use
- You want to bootstrap RevenueCat from an existing ASC catalog.
- You want to create missing ASC subscriptions/IAPs, then map them into RevenueCat.
- You need a drift audit before release.
- You want deterministic product mapping based on identifiers.

## Preconditions
- `asc` authentication is configured (`asc auth login` or `ASC_*` env vars).
- RevenueCat MCP server is configured and authenticated.
- In Cursor and VS Code, OAuth auth is available for RevenueCat MCP. API key auth is also supported.
- You know:
  - ASC app ID (`APP_ID`)
  - RevenueCat `project_id`
  - target RevenueCat app type (`app_store` or `mac_app_store`) and bundle ID for create flows
- Use a write-enabled RevenueCat API v2 key when applying changes.

## Safety defaults
- Start in **audit mode** (read-only).
- Require explicit confirmation before writes.
- Never delete resources in this workflow.
- Continue on per-item failures and report all failures at the end.

## Canonical identifiers
- Primary cross-system key: ASC `productId` == RevenueCat `store_identifier`.
- Keep `productId` stable once products are live.
- Do not use display names as unique identifiers.

## Scope boundary
- RevenueCat MCP configures RevenueCat resources; it does not create App Store Connect products directly.
- Use `asc` commands to create missing ASC subscription groups, subscriptions, and IAPs before RevenueCat mapping.

## Modes

### 1) Audit mode (default)
1. Read ASC source catalog.
2. Read RevenueCat target catalog.
3. Build a diff with actions:
   - missing in ASC
   - missing in RevenueCat
   - mapping conflicts (identifier/type/app mismatch)
4. Present the plan. Audit-only requests finish with findings; apply requests require approval covering the complete current diff. Reuse prior approval if the refreshed diff is unchanged.

### 2) Apply mode (explicit)
Execute approved actions in this order:
1. Ensure ASC groups/subscriptions/IAP exist.
2. Ensure RevenueCat app/products exist.
3. Ensure entitlements and product attachments.
4. Ensure offerings/packages and package attachments.
5. Verify and print a final reconciliation summary.

## Step-by-step workflow

### Step A - Read current ASC catalog

```bash
asc subscriptions groups list --app "APP_ID" --paginate --output json
asc iap list --app "APP_ID" --paginate --output json
# for each subscription group:
asc subscriptions list --group-id "GROUP_ID" --paginate --output json
```

### Step B - Read current RevenueCat catalog (MCP)

Use these MCP tools (with `project_id` and pagination where applicable):
- `mcp_RC_get_project`
- `mcp_RC_list_apps`
- `mcp_RC_list_products`
- `mcp_RC_list_entitlements`
- `mcp_RC_list_offerings`
- `mcp_RC_list_packages`

### Step C - Build mapping plan

Map ASC product types to RevenueCat product types:
- ASC subscription -> RevenueCat `subscription`
- ASC IAP `CONSUMABLE` -> RevenueCat `consumable`
- ASC IAP `NON_CONSUMABLE` -> RevenueCat `non_consumable`
- ASC IAP `NON_RENEWING_SUBSCRIPTION` -> RevenueCat `non_renewing_subscription`

Suggested entitlement policy:
- subscriptions: one entitlement per subscription group (or explicit map provided by user)
- non-consumable IAP: one entitlement per product
- consumable IAP: no entitlement by default unless user asks

### Step D - Ensure missing ASC items (if requested)

Resolve every parent and version before writing. Match groups by exact reference
name and products by `productId`; never treat a display name as identity. Reuse
the canonical ID when the resource already exists, and run a create command
only when the fully paginated read proves it is missing. A RevenueCat product
mapping is not proof that the corresponding ASC subscription is review-ready.

```bash
# Resolve GROUP_ID by exact referenceName.
asc subscriptions groups list --app "APP_ID" --paginate --output json
# If and only if the fully paginated list has zero exact matches:
asc subscriptions groups create --app "APP_ID" --reference-name "Premium" --output json
# For one match, reuse its ID. For more than one, stop and require an explicit GROUP_ID.

# Resolve SUB_ID by exact productId within GROUP_ID. Run setup only for a missing
# parent or an explicitly approved reconciliation of that same product ID.
asc subscriptions list --group-id "GROUP_ID" --paginate --output json
# If and only if the fully paginated list has zero exact matches, run setup:
asc subscriptions setup \
  --app "APP_ID" \
  --group-id "GROUP_ID" \
  --reference-name "Monthly" \
  --product-id "com.example.premium.monthly" \
  --subscription-period ONE_MONTH \
  --review-screenshot "./review.png" \
  --price "3.99" \
  --price-territory "USA" \
  --territories "USA" \
  --no-verify \
  --output json
# For one match, reuse its ID. For more than one, stop and require an explicit SUB_ID.
# Re-run setup for an existing SUB_ID only for an explicitly approved reconciliation.

# Resolve the unique mutable group version for this review lifecycle.
asc subscriptions groups versions list --group-id "GROUP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json
# If and only if the list has zero matches:
asc subscriptions groups versions create --group-id "GROUP_ID" --output json
# For one match, reuse .data[0].id. For more than one, stop and require an explicit GROUP_VERSION_ID.

# Resolve the en-US localization on GROUP_VERSION_ID. Create it only when
# missing; update the resolved localization ID when its values differ.
asc subscriptions groups versions localizations list --version-id "GROUP_VERSION_ID" --paginate --output json
asc subscriptions groups versions localizations create --version-id "GROUP_VERSION_ID" --locale "en-US" --name "Premium" --output json
asc subscriptions groups versions localizations update --id "GROUP_LOC_ID" --name "Premium"

# Resolve the unique mutable subscription version for this review lifecycle.
asc subscriptions versions list --subscription-id "SUB_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json
# If and only if the list has zero matches:
asc subscriptions versions create --subscription-id "SUB_ID" --output json
# For one match, reuse .data[0].id. For more than one, stop and require an explicit SUBSCRIPTION_VERSION_ID.
asc subscriptions versions localizations list --version-id "SUBSCRIPTION_VERSION_ID" --paginate --output json
asc subscriptions versions localizations create --version-id "SUBSCRIPTION_VERSION_ID" --locale "en-US" --name "Premium Monthly" --description "Unlock all premium features." --output json
asc subscriptions versions localizations update --id "SUBSCRIPTION_LOC_ID" --name "Premium Monthly" --description "Unlock all premium features."

# Read back the exact versions selected above, then run the final strict validator.
asc subscriptions groups versions localizations list --version-id "GROUP_VERSION_ID" --paginate --output table
asc subscriptions versions localizations list --version-id "SUBSCRIPTION_VERSION_ID" --paginate --output table
asc validate subscriptions --app "APP_ID" --strict --output table

# Resolve IAP_ID by exact productId.
asc iap list --app "APP_ID" --paginate --output json
# If and only if the fully paginated list has zero exact matches:
asc iap create \
  --app "APP_ID" \
  --type NON_CONSUMABLE \
  --ref-name "Lifetime" \
  --product-id "com.example.lifetime" \
  --output json
# For one match, reuse its ID. For more than one, stop and require an explicit IAP_ID.

# Resolve the unique mutable IAP version for this review lifecycle.
asc iap versions list --iap-id "IAP_ID" --state PREPARE_FOR_SUBMISSION --paginate --output json
# If and only if the list has zero matches:
asc iap versions create --iap-id "IAP_ID" --output json
# For one match, reuse .data[0].id. For more than one, stop and require an explicit IAP_VERSION_ID.
asc iap versions localizations list --version-id "IAP_VERSION_ID" --paginate --output json
asc iap versions localizations create --version-id "IAP_VERSION_ID" --locale "en-US" --name "Lifetime" --description "Unlock all premium features." --output json
asc iap versions localizations update --localization-id "IAP_LOC_ID" --name "Lifetime" --description "Unlock all premium features."
asc iap versions localizations list --version-id "IAP_VERSION_ID" --paginate --output table
```

Each adjacent create/update pair above is conditional, not a sequence to run
blindly: create when the list has no match, update when the resolved match has
different values, and do nothing when it already matches. Capture `.data.id`
from a create response or `.data[].id` from the preceding list as the canonical
ID used by later commands. For each version list, zero
`PREPARE_FOR_SUBMISSION` matches means create, one means reuse that ID, and more
than one means stop and require the user to choose an explicit version ID. Do
not create a version to resolve ambiguity: parent deletion did not reliably
cascade IAP or subscription versions in live testing.

`subscriptions setup` finishes the parent, App Review screenshot delivery,
complete App Store price matrix, and sale availability. The version-scoped
commands finish the group and subscription localizations that RevenueCat
cannot. Keep sale availability scoped to the requested territories; pricing
still needs Apple's complete equalized territory matrix. `--no-verify` is
intentional in this split workflow because final verification must wait until
the version metadata exists; the explicit readbacks and validator are the final
gate.

If an existing API-created subscription remains `MISSING_METADATA` even though
the selected base price is unchanged, re-run the same setup inputs with
`--repair`. Repair atomically rebuilds and re-saves the complete equalized price
matrix; it is not a duplicate single-price POST.

For every resolved ASC subscription, require this final gate before creating or
attaching its RevenueCat product. Run it after the final ASC reconciliation even
when the subscription and its selected version were reused without any ASC
write:

```bash
mkdir -p "./audit"
asc validate subscriptions --app "APP_ID" --strict --output json --pretty \
  > "./audit/subscriptions-validation.json"
```

For every resolved ASC IAP, require the IAP gate before creating or attaching
its RevenueCat product. Run it after the final ASC reconciliation even when the
IAP and its selected version were reused without any ASC write:

```bash
mkdir -p "./audit"
asc validate iap --app "APP_ID" --strict --output json --pretty \
  > "./audit/iap-validation.json"
```

Both commands are strict mapping gates, not post-write smoke tests. Do not map
any resolved or reused subscription while validation reports warnings, errors,
`MISSING_METADATA`, a review screenshot that is not `COMPLETE`, or incomplete
or unverified pricing coverage. Do not map any resolved or reused IAP while its
validator reports warnings or errors. A zero-write audit or apply run must still
execute the applicable gate and require a zero exit status before RevenueCat
product creation or attachment. Direct redirection preserves each validator's
exit status. Retain `./audit/subscriptions-validation.json` and
`./audit/iap-validation.json` as final audit evidence.

### Step E - Ensure RevenueCat app and products

Use MCP:
- create app if missing: `mcp_RC_create_app`
- create products: `mcp_RC_create_product`
  - `store_identifier` = ASC `productId`
  - `app_id` = RevenueCat app ID
  - `type` from mapping above

### Step F - Ensure entitlements and attachments

Use MCP:
- list/create entitlements: `mcp_RC_list_entitlements`, `mcp_RC_create_entitlement`
- attach products: `mcp_RC_attach_products_to_entitlement`
- verify attachments: `mcp_RC_get_products_from_entitlement`

### Step G - Ensure offerings and packages (optional)

Use MCP:
- list/create/update offerings:
  - `mcp_RC_list_offerings`
  - `mcp_RC_create_offering`
  - `mcp_RC_update_offering` (`is_current=true` only if requested)
- list/create packages:
  - `mcp_RC_list_packages`
  - `mcp_RC_create_package`
- attach products to packages:
  - `mcp_RC_attach_products_to_package` with `eligibility_criteria: "all"`

Recommended package keys:
- `ONE_WEEK` -> `$rc_weekly`
- `ONE_MONTH` -> `$rc_monthly`
- `TWO_MONTHS` -> `$rc_two_month`
- `THREE_MONTHS` -> `$rc_three_month`
- `SIX_MONTHS` -> `$rc_six_month`
- `ONE_YEAR` -> `$rc_annual`
- lifetime IAP -> `$rc_lifetime`
- custom -> `$rc_custom_<name>`

## Expected output format

Return a final summary with:
- ASC created counts (groups/subscriptions/IAP)
- RevenueCat created counts (apps/products/entitlements/offerings/packages)
- attachment counts (entitlement-products, package-products)
- skipped existing items
- failed items with actionable errors

Example:

```text
ASC: created groups=1 subscriptions=2 iap=1, skipped=14, failed=0
RC: created apps=0 products=3 entitlements=2 offerings=1 packages=2, skipped=27, failed=1
Attachments: entitlement_products=3 package_products=2
Failures:
- com.example.premium.annual: duplicate store_identifier exists on another RC app
```

## Agent behavior
- Always run audit first, even in apply mode.
- Reuse approval covering the complete current diff for its create/update operations. Ask again only when approval is absent or additional or materially changed actions fall outside it.
- Match by `store_identifier` first.
- Use full pagination (`--paginate` for ASC, `starting_after` for RevenueCat tools).
- Continue processing after per-item failures and report all failures together.
- Never auto-delete ASC or RevenueCat resources in this skill.
- For every resolved subscription, including a reused no-write match, run `asc validate subscriptions --app "APP_ID" --strict --output json --pretty`, save its JSON, and block RevenueCat creation or attachment unless the gate exits zero with no missing metadata or incomplete pricing.
- For every resolved IAP, including a reused no-write match, run `asc validate iap --app "APP_ID" --strict --output json --pretty`, save its JSON, and block RevenueCat creation or attachment unless the gate exits zero.

## Common pitfalls
- Wrong RevenueCat `project_id` or app ID.
- Creating RC products under the wrong platform app.
- Accidentally assigning consumables to entitlements.
- Skipping the post-create ASC re-read step.
- Mapping a RevenueCat product before ASC setup finishes localization, review screenshot delivery, the complete price matrix, and availability.
- Missing offering/package verification after product creation.

## Additional resources
- Workflow examples: [examples.md](examples.md)
- Source references: [references.md](references.md)

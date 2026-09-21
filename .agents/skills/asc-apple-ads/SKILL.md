---
name: asc-apple-ads
description: Use when managing Apple Ads with asc, including OAuth profiles, ad-account discovery, Platform API v1 campaigns and targeting, reports, assets, recommendations, guarded mutations, raw requests, and Campaign Management API v5 migration.
---

# asc Apple Ads

Run Apple Ads work through `asc ads`. Apple Ads credentials are separate from App Store Connect credentials; `asc auth login` does not configure Ads.

## Pick the API first

- Direct `asc ads <resource> ...` commands use Apple Ads Platform API v1 and an ad account ID.
- Deprecated Campaign Management API v5 commands live under `asc ads v5 ...` and use an organization ID. Apple retires v5 on January 26, 2027.
- Never substitute an org ID for an ad account ID. The CLI keeps them separate.
- Run the exact leaf command with `--help` before building a request file. Platform v1 payloads and response envelopes differ from v5; the CLI does not translate them.
- For non-interactive pipelines, pass `--file -` to read a JSON request body from stdin; the CLI rejects it when stdin is a terminal.
- Resource, report, upload, and raw commands emit lossless JSON. Use `jq` for projections instead of asking for table or markdown output.

## Authenticate and pin the account

Store both contexts when a profile must support v1 and legacy v5:

```bash
asc ads auth login \
  --name "Marketing" \
  --client-id "$ASC_ADS_CLIENT_ID" \
  --team-id "$ASC_ADS_TEAM_ID" \
  --key-id "$ASC_ADS_KEY_ID" \
  --private-key "$ASC_ADS_PRIVATE_KEY_PATH" \
  --ad-account "987654" \
  --org "123456" \
  --network
```

For CI, set Ads-specific variables and bypass the host keychain:

```bash
export ASC_ADS_CLIENT_ID="SEARCHADS_CLIENT_ID"
export ASC_ADS_TEAM_ID="SEARCHADS_TEAM_ID"
export ASC_ADS_KEY_ID="KEY_ID"
export ASC_ADS_PRIVATE_KEY_PATH="$HOME/.asc/apple-ads-private-key.pem"
export ASC_ADS_AD_ACCOUNT_ID="987654"
export ASC_ADS_BYPASS_KEYCHAIN=1
```

`ASC_ADS_PRIVATE_KEY` and `ASC_ADS_PRIVATE_KEY_B64` also work. If another trusted process minted a short-lived token, set `ASC_ADS_ACCESS_TOKEN`; scoped v1 calls still need an ad account ID.

Check auth without printing a token:

```bash
asc ads auth status --validate --output json
asc ads auth discover --ads-profile "Marketing" --output json
asc ads auth doctor --output json
```

Discovery calls Platform v1 `GET /v1/me` and `GET /v1/acls`. Compare each ACL's ad-account ID, name, organization ID, and roles; never select the first result automatically. Print the chosen account before any mutation, then pass both `--ads-profile "Marketing"` and `--ad-account "987654"` when more than one profile or account is available.

For named profiles, the profile's `ad_account_id` and `org_id` stand alone; they do not inherit context from another profile or root config. V1 context precedence is `--ad-account`, `ASC_ADS_AD_ACCOUNT_ID`, the selected profile, then profile-less root config. Legacy v5 uses the matching `--org` and `ASC_ADS_ORG_ID` chain.

## Start read-only

Identity and ACL calls need no ad account context:

```bash
asc ads me view --ads-profile "Marketing" --output json
asc ads acls list --ads-profile "Marketing" --output json
asc ads orgs view --ads-profile "Marketing" --org-id "123456" --output json
```

Then prove the selected account with a small app search:

```bash
asc ads apps search \
  --ads-profile "Marketing" \
  --ad-account "987654" \
  --query "Example" \
  --limit 1 \
  --output json
```

App search requires at least one of `--query`, `--cpids`, or `--return-owned-apps`. Storefronts use comma-separated ISO alpha-2 codes. Add `--paginate` only when every search result is needed.

Use each resource's `find` command for inventory. Most v1 queries put filters, sorting, and pagination in a JSON object. A subordinate-resource filter looks like this:

```json
{
  "filters": [
    {"field": "campaignId", "operator": "EQUALS", "value": ["campaign-id"]}
  ],
  "pagination": {"offset": 0, "pageSize": 100, "fetchTotalCount": true}
}
```

```bash
asc ads campaigns find --ads-profile "Marketing" --ad-account "987654" --output json
asc ads ad-groups find --ads-profile "Marketing" --ad-account "987654" --file query.json --output json
asc ads ads find --ads-profile "Marketing" --ad-account "987654" --file query.json --output json
```

Omitting `--file` from `campaigns find` requests the default first page. To control or exhaust the result set, use `pagination.offset`, `pageSize`, and `fetchTotalCount` in a query file, read the response pagination, and advance the offset until complete. This command has no `--paginate` flag. Platform filters use the singular `value`; do not copy v5 `Selector` fields such as `conditions` or plural `values`, which current `asc` rejects before auth.

The direct v1 tree also covers ad accounts and advertiser resources; app eligibility, locales, product pages, and rejection reasons; brands, business categories, locations, location groups, creatives, and assets; geographic targeting and shared budgets; reports for apps and brands; insights, suggestions, recommendations, and change history. Discover the exact leaf instead of falling back to raw HTTP:

```bash
asc ads change-history --help
asc ads suggestions --help
asc ads rejection-reasons --help
asc ads reports brands --help
```

Keyword queries need a selector file. Targeting keywords require an `id`, `adGroupId`, or `campaignId` filter. Negative keywords require `id` or `adGroupId`; campaign-level negative keywords combine `campaignId` with an `adGroupId` filter whose operator is `IS_NULL`.

```bash
asc ads targeting-keywords find --ads-profile "Marketing" --ad-account "987654" --file keyword-query.json --output json
asc ads negative-keywords find --ads-profile "Marketing" --ad-account "987654" --file negative-keyword-query.json --output json
```

## Reports and optimization

V1 reports require an endpoint-specific body. Dates live under `timeRange`, page controls use `offset` and `pageSize`, and campaign or ad-group IDs belong in `filters`:

```json
{
  "pagination": {"offset": 0, "pageSize": 20},
  "filters": [
    {"field": "campaignId", "operator": "EQUALS", "value": ["campaign-id"]}
  ],
  "groupBy": ["countryOrRegion"],
  "timeRange": {
    "start": "2026-08-01",
    "end": "2026-08-14",
    "timeZone": "ORTZ",
    "granularity": "DAILY"
  }
}
```

```bash
asc ads reports apps campaigns \
  --ads-profile "Marketing" \
  --ad-account "987654" \
  --file report.json \
  --output json
```

Report commands do not accept `--paginate`; change pagination in the body. Inspect the leaf help because report entities accept different `groupBy` and option values.

Recommendations and suggestions also use endpoint-specific bodies. Applying or dismissing recommendations can change spend and requires `--confirm`:

```bash
asc ads recommendations daily-budgets find --ads-profile "Marketing" --ad-account "987654" --file query.json
asc ads recommendations daily-budgets apply --ads-profile "Marketing" --ad-account "987654" --file recommendations.json --confirm
```

## Guard mutations

Do not mutate until the user has approved the ad account, resource type, target IDs, and reviewed payload. Keep request JSON in files; never invent fields from a related v5 schema.

Campaign creation may start spending. `CampaignCreate` requires `adAccountId`, `billingEvent`, `dailyBudget`, `name`, `promotedObjectId`, `promotedObjectType`, and `targeting`. The CLI sends the file unchanged: `--ad-account` selects the request context but does not inject `adAccountId` into the JSON. Start from this paused shape, replace every placeholder with values read from the selected account, and recheck the current Apple v1 schema for any account-specific requirements:

```json
{
  "name": "ASC agent validation 2026-08-15T00:00:00Z",
  "status": "PAUSED",
  "adAccountId": 987654,
  "promotedObjectType": "APPSTORE_APP",
  "promotedObjectId": "123456789",
  "billingEvent": "TAPS",
  "dailyBudget": {"value": {"amount": "1", "currency": "USD"}},
  "startTime": "2030-01-01T00:00:00.000",
  "endTime": "2030-01-02T00:00:00.000",
  "targeting": {"countryOrRegion": {"include": ["US"]}},
  "bidStrategy": {"bidStrategyType": "MANUAL_CPT", "bidStrategyGoal": "TAP"}
}
```

A payload with explicit top-level `"status":"PAUSED"` can run without `--confirm`; an omitted or non-paused status requires it.

```bash
asc ads campaigns create --ads-profile "Marketing" --ad-account "987654" --file paused-campaign.json
asc ads campaigns pause --ads-profile "Marketing" --ad-account "987654" --campaign "campaign-id"
asc ads campaigns resume --ads-profile "Marketing" --ad-account "987654" --campaign "campaign-id" --confirm
```

Campaign updates need `--confirm` when they can change budget, targeting, bids, delivery, dates, or status. A name-only update or a name plus `PAUSED` status does not. Deletes, bulk creates or updates, recommendation apply or dismiss calls, budget-order writes, and other operationally risky mutations require confirmation before auth or network access.

Ad-group creation and keyword bulk writes are examples of always-confirmed delivery or targeting changes. V1 bulk keyword files use wrapper objects such as `KeywordCreateBulkRequest`, not the v5 raw-array shape:

```bash
asc ads ad-groups create --ads-profile "Marketing" --ad-account "987654" --file ad-group.json --confirm
asc ads targeting-keywords create-bulk --ads-profile "Marketing" --ad-account "987654" --file keywords.json --confirm
asc ads targeting-keywords delete --ads-profile "Marketing" --ad-account "987654" --keyword "keyword-id" --confirm
```

Shared budgets use the `budget-orders` command group. Create, update, and delete are context-free but require confirmation; view and find accept optional ad-account context.

```bash
asc ads budget-orders create --ads-profile "Marketing" --file shared-budget.json --confirm
asc ads budget-orders update --ads-profile "Marketing" --budget-order "budget-id" --file update.json --confirm
asc ads budget-orders delete --ads-profile "Marketing" --budget-order "budget-id" --confirm
```

Ad-account creation also requires `--confirm` because its account family cannot change and Apple provides no delete endpoint. An ad-account update containing `delegations` requires confirmation because it replaces the complete list.

Use the dedicated multipart command for brand assets. Poll until Apple finishes processing:

```bash
asc ads assets upload --ads-profile "Marketing" --file ./brand.png --brand "BRAND_ID" --ad-account "987654"
asc ads assets view --ads-profile "Marketing" --asset "ASSET_UUID" --ad-account "987654"
```

Only use an asset when `eligibility.status` is `ELIGIBLE`; for `LIMITED`, inspect `allowedGroups`. Do not attach `PENDING` or `INELIGIBLE` assets.

## Raw requests

Use first-class commands for routine work. Raw v1 requests accept only `v1/...` paths or `https://api.ads.apple.com/v1/...` URLs:

```bash
asc ads api request \
  --method POST \
  --path v1/campaigns/query \
  --ads-profile "Marketing" \
  --ad-account "987654" \
  --file query.json \
  --output json
```

Unknown mutations fail closed, and risky known mutations require `--confirm`. The raw command rejects multipart asset upload; use `asc ads assets upload`.

Keep legacy calls explicit:

```bash
asc ads v5 api request \
  --method POST \
  --path v5/campaigns/find \
  --ads-profile "Marketing" \
  --org "123456" \
  --file selector.json \
  --output json
```

## Migrate v5 one command at a time

Keep existing v5 payloads under `asc ads v5` until each script has a reviewed v1 body and response parser. Common moves:

| Deprecated v5 | Platform API v1 |
| --- | --- |
| `asc ads v5 campaigns list` | `asc ads campaigns find` |
| `asc ads v5 apps localized-details` | `asc ads apps locales find` |
| `asc ads v5 product-pages list` | `asc ads product-pages find` |
| `asc ads v5 reports campaigns` | `asc ads reports apps campaigns` |
| `asc ads v5 campaigns pause` / `resume` | `asc ads campaigns pause` / `resume` |
| v5 campaign or ad-group negative keywords | `asc ads negative-keywords ...` with scope in the body |

Seven v5 leaves have no one-command v1 replacement: product-page countries, product-page devices, targeting-keyword bulk delete, both negative-keyword bulk deletes, and impression-share report list and view. Do not pretend that `geo search`, `insights impression-share`, or single-resource deletes preserve those contracts.

## Finish live tests cleanly

- Start with ACL discovery and a one-result app search.
- Use a unique timestamped name and explicit `PAUSED` status for disposable campaign tests.
- Save every created ID from JSON output.
- Pause spend-bearing resources before checking anything else.
- Reread a test campaign with `asc ads campaigns view --ads-profile "Marketing" --ad-account "987654" --campaign "campaign-id" --output json`.
- Delete only test-created campaigns with `asc ads campaigns delete --ads-profile "Marketing" --ad-account "987654" --campaign "campaign-id" --confirm`; do not delete a pre-existing parent.
- Run the same `campaigns view` again. Treat Apple's not-found response as cleanup proof; report any campaign that still exists or could not be removed.

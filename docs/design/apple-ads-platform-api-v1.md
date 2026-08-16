# Apple Ads Platform API v1

## Goal

Add the complete Apple Ads Platform API v1 surface as the default Ads resource tree. Preserve the behavior of Campaign Management API v5 commands under an explicit deprecated namespace until Apple retires v5 on January 26, 2027.

## Command placement

Platform API v1 commands live directly under `asc ads`. Existing Campaign Management API v5 commands move to `asc ads v5`.

Examples:

```bash
asc ads acls list --output json
asc ads campaigns find --ad-account "AD_ACCOUNT_ID" --file query.json
asc ads assets upload --ad-account "AD_ACCOUNT_ID" --file creative.png --brand "BRAND_ID"
asc ads api request --method GET --path v1/me
```

This makes the long-lived API the shortest path while keeping the incompatible v5 request and response schemas explicit under a versioned legacy tree.

## API contract

- Base URL: `https://api.ads.apple.com/v1/`.
- OAuth token endpoint, client-secret JWT, and `searchadsorg` scope remain unchanged.
- Ad-account-scoped requests send `X-AP-Context: adAccountId=<id>;`.
- `GET /me`, `GET /acls`, `GET /orgs/{id}`, `GET /advertiser-resources`, and `POST /ad-accounts` omit `X-AP-Context`.
- The official clients omit context for shared-budget create, update, and delete, and make it optional for shared-budget view and query. The CLI follows that client contract because Apple's generated DocC parameter table conflicts with every official client and request example for these five operations.
- JSON endpoints accept a request file and preserve Apple's response envelope on stdout.
- Asset upload uses multipart form data with a rootfs-controlled local file.
- DELETE and state-changing recommendation apply or dismiss commands require `--confirm`.
- Comma-separated CLI values for query-string arrays are encoded as repeated wire keys.
- Errors and diagnostics go to stderr; usage failures exit with code 2.

## Authentication and configuration

Add an optional ad account ID beside the existing legacy organization ID:

- flag: `--ad-account`
- environment: `ASC_ADS_AD_ACCOUNT_ID`
- config/profile field: `ad_account_id`

Resolution keeps the existing profile and strict-auth rules. A scoped command resolves the explicit flag first, then the environment, then the selected profile. A named profile never inherits root `ads.org_id` or `ads.ad_account_id`; store both values on the profile or pass the corresponding flags. Profile-less access-token or environment authentication can still use matching root context.

## Endpoint coverage

Apple documents 99 v1 endpoints in 24 collections. The completed 4.4.0 stack is
split by dependency and operator workflow:

1. Foundation, account management, app search, and app eligibility.
2. Campaigns, ad groups, geo targeting, keywords, negative keywords, ads, product pages, bulk operations, and budget orders.
3. Apple Maps brands, business categories, locations, location groups, creatives, and assets.
4. App and brand reports, insights, recommendations, suggestions, and change history.
5. Legacy v5 deprecation warnings and migration guidance.

The cumulative 4.4.0 stack implements all 99 operations. Its foundation layer registers 13 operations, the campaign layer adds 41, Maps and assets add 21, and reports and optimization add 24. The endpoint specs drive command registration. A separate checked-in contract fixture records method, path, parameters, SDK body optionality, response type, context requirement, confirmation, command path, and Apple source URL for all 99 operations. The final cumulative layer compares the implementation with that fixture and asserts exact count and uniqueness; earlier layers assert their implemented subsets.

### Reports and optimization

Reporting requests keep Apple's pagination and selector fields in the JSON payload. The CLI does not expose `--paginate` for these commands because query-string pagination cannot safely advance the reporting response. Successful result and pagination envelopes are printed unchanged; API errors continue through the CLI's structured stderr formatter.

```bash
# App and business-brand reports
asc ads reports apps campaigns --ad-account "AD_ACCOUNT_ID" --file report.json --output json
asc ads reports brands search-terms --ad-account "AD_ACCOUNT_ID" --file report.json --output json

# Read-only insights, recommendations, suggestions, and audit queries
asc ads insights impression-share find --ad-account "AD_ACCOUNT_ID" --file query.json --output json
asc ads recommendations daily-budgets find --ad-account "AD_ACCOUNT_ID" --file query.json --output json
asc ads suggestions keywords find --ad-account "AD_ACCOUNT_ID" --file query.json --output json
asc ads change-history find --ad-account "AD_ACCOUNT_ID" --file query.json --output json
asc ads change-history view --ad-account "AD_ACCOUNT_ID" --detail-id "Campaign.444555666.txn_abc123def456" --limit 100 --paginate --output json
```

Recommendation apply and dismiss operations accept an array payload and require explicit confirmation before the CLI reads the payload or resolves credentials:

```bash
asc ads recommendations target-cpas apply --ad-account "AD_ACCOUNT_ID" --file recommendations.json --confirm --output json
asc ads recommendations daily-budgets dismiss --ad-account "AD_ACCOUNT_ID" --file recommendations.json --confirm --output json
```

## Compatibility and deprecation

The v1 host, context identifier, paths, payloads, response envelopes, pagination, reporting, and creative model are incompatible with v5. Because Apple Ads is an auxiliary surface in this App Store Connect-focused CLI, 4.4.0 takes the breaking command-tree change now: direct resource paths use v1 and v5 moves under `asc ads v5`.

Every runnable v5 leaf remains available but gains:

- a `DEPRECATED:` direct-help prefix;
- one stderr warning per invocation;
- a v1 replacement command or an explicit statement when no one-command replacement exists;
- migration guidance in the Apple Ads command documentation.

No v5 operation is removed in 4.4.0; only its command prefix changes. The
intermediate nested prototype is removed before merge and does not
become a compatibility alias.

### Migration contract

Teach Platform API v1 first in user-facing examples. `--org` remains the v5
organization context; `--ad-account` is the separate v1 ad-account context.
V1 IDs, payloads, query objects, report requests, and response envelopes are
not converted from v5 shapes by the CLI. V1 report pagination stays in the
request body, and the legacy `--paginate` behavior does not apply to v1
reports. The v5 `asc ads v5 reports preset` helper, campaign pause/resume
workflows, and v5 raw request command remain runnable compatibility paths with
warnings; the raw v5 command continues to send v5 paths and is never silently
rewritten.

All named v1 query commands reject legacy v5 selector members before resolving
credentials or sending a request. The shared migration guard covers the
standard `QueryRequest` plus reporting, insights, recommendations, policy,
and audit query bodies. It reports the direct member replacements:
`conditions` to `filters`, `values` to `value`, `orderBy` to `sorting`,
`sortOrder` to `order`, and `pagination.limit` to `pagination.pageSize`.
It also catches the renamed operator and sort values (`STARTSWITH` and
`ENDSWITH` to `STARTS_WITH` and `ENDS_WITH`; `ASCENDING` and `DESCENDING`
to `ASC` and `DESC`).

The schema-aware part of the guard handles members that do not have one
universal replacement. Only `AppsReportingRequest` and
`BrandsReportingRequest` accept top-level `fields`; other query bodies reject
the legacy projection member. Reporting dates, time zone, and granularity move
under `timeRange`, while grand totals and empty app-metric rows move to
`options.includeRows`. `returnRowTotals` has no v1 request field, and brand
reports do not support `EMPTY_METRICS`. The v5 custom impression-share
`name` and relative `dateRange` members are also rejected with their specific
v1 migration paths.

Search Term Popularity has one runtime-only exception. Apple's Platform API
documentation and generated clients model its sort member as `order`, but the
live endpoint rejects that property and accepts `sortOrder` instead. The CLI
therefore accepts `sorting[].sortOrder` with the v1 values `ASC` and `DESC`
only for `SearchTermPopularityQueryRequest`; all other v1 query schemas keep
the documented `sorting[].order` contract. The starter payload and local
migration errors expose this endpoint-specific spelling before authentication.

Alternatives considered were silently rewriting `order` on the wire and
removing sorting from the starter payload. Rewriting would make the sent body
differ from the operator's file, while omission would leave explicit sorting
unusable. An endpoint-specific validated spelling is transparent and keeps the
exception from weakening the other v1 migration guards.

Explicit `null` legacy members are rejected because the Platform API rejects
the property itself. The CLI does not rewrite payloads automatically: value
cardinality and accepted fields vary by endpoint, so silent conversion could
change query semantics.

Platform v1 unifies campaign and ad-group negative keywords under
`negative-keywords`, but it has no bulk-delete endpoint. These seven v5 leaves
have no one-command v1 replacement in 4.4.0: `product-pages countries list`,
`product-pages devices list`, `targeting-keywords delete-bulk`,
`campaign-negative-keywords delete-bulk`, `ad-group-negative-keywords
delete-bulk`, and `impression-share-reports list` and `view`. Documentation
must not present geo search, impression-share insights, or single-keyword
delete as drop-in replacements for those contracts.

## Tests

RED-GREEN coverage includes:

- exact endpoint count, unique names and command paths, context metadata, path expansion, query encoding, body kind, and confirmation metadata;
- CLI registration, flags, required values, invalid values, unexpected arguments, stdout/stderr, and exit code 2;
- HTTP method, host, path, headers, repeated query parameters, JSON body, response preservation, and API errors;
- ad-account resolution across flag, environment, config, and named profiles with strict-auth isolation;
- raw v1 URL guardrails and context-free endpoint exceptions;
- multipart upload fields, content type, rootfs reads, file failures, and API failures;
- exact legacy warning text and direct-help migration paths;
- pre-auth rejection of legacy v5 selector members across every v1 query body
  type, while preserving valid v1 payloads and raw API pass-through behavior;
- generated command docs and built-binary smoke tests.

The local repository gate is `make format`, `make check-docs`, `make lint`, and `ASC_BYPASS_KEYCHAIN=1 make test`.

## Handoff contract

Before any commit or push, run the full local repository gate above and keep
the endpoint fixture, generated command docs, and migration tests synchronized.
Live-account behavior remains the principal unverified risk: an operator with
real Apple Ads credentials must validate read-only Platform calls first, then
explicitly authorize any mutation testing. If mutation testing is necessary,
use only disposable app `6759231657`, clean up every temporary resource after
the test, and never mutate a non-disposable app without explicit approval.
Never place those credentials in the repository or test fixtures.

## Alternatives considered

An `--api-version` flag would mix incompatible leaves and payload schemas in one help surface. Keeping v1 permanently under `platform` would make the future default API more verbose forever. The selected breaking tree gives v1 the idiomatic direct paths and moves the retiring surface under the exact `v5` version label.

For query migration, automatic v5-to-v1 payload conversion was rejected
because `value` may be scalar or array depending on the schema and operator,
and report bodies require a larger structural rewrite. Leaving validation to
Apple was also rejected: it spends a live request on a deterministic local
error and produces one-field-at-a-time 400 responses instead of a complete
migration hint.

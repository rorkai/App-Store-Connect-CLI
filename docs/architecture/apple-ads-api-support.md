# Apple Ads Campaign Management API v5 Support (Historical Scope)

Status: Removed in 6.0.0 after Apple retired Campaign Management API v5 on
January 26, 2027; retained as a historical reference only. See
`docs/release-notes-apple-ads-v5-removal.md`.
Research date: May 31, 2026
Original target API: Apple Ads Campaign Management API 5.5
Historical command root: `asc ads`
Release target: 4.4.0
Current API addition: Apple Ads Platform API v1
Preferred API in CLI 4.4.0: Apple Ads Platform API v1 under direct `asc ads`

## Historical goal

This note records the existing Campaign Management API v5 surface and the
4.4.0 addition of the Apple Ads Platform API v1 surface. Direct `asc ads`
resource commands use v1. The incompatible, retiring v5 surface moves under
`asc ads v5`; the intermediate nested prototype is removed before
merge. Users should not need a raw HTTP client for supported workflows.

The v1 and v5 request contracts remain separate because they use different
hosts, contexts, payloads, and response envelopes. Existing v5 commands move
under the explicit `v5` group instead of being silently retargeted.

The original implementation preserved the local CLI style:

- `ffcli` commands with `shared.DefaultUsageFunc`
- explicit long flags
- TTY-aware output defaults through `shared.BindOutputFlags`
- non-interactive behavior only
- `--confirm` for deletes and bulk deletes
- `--paginate` for multi-page list/search commands
- JSON output that preserves Apple response envelopes for agents
- no new third-party dependencies

## Historical v5 sources

Apple sources used for the May 2026 v5 implementation:

- Apple Ads root: https://developer.apple.com/documentation/apple_ads
- Platform API v1: https://developer.apple.com/documentation/apple-ads-platform-api
- OAuth: https://developer.apple.com/documentation/apple_ads/implementing-oauth-for-the-apple-search-ads-api
- Calling the API: https://developer.apple.com/documentation/apple_ads/calling-the-apple-search-ads-api
- API functionality: https://developer.apple.com/documentation/apple_ads/using-apple-search-ads-api-functionality
- API 5 changelog: https://developer.apple.com/documentation/apple_ads/apple-search-ads-campaign-management-api-5

At the original research date, API 5 was Apple's current Campaign Management
API and API 5.5 had been released in February 2026. Apple later made Platform
API v1 available. CLI 4.4.0 implements that API under direct `asc ads` resource
paths and keeps the original v5 commands under `asc ads v5` as warning-producing compatibility paths until
Apple's January 26, 2027 retirement date.

At the research date, deprecated `Creative Sets` were not included as commands
because Apple's v5 documentation marked the collection as deprecated and
exposed no active v5 endpoint under that page. The
`includeDeletedCreativeSetAssets` query parameter on
`GET /v5/creatives/{creativeId}` was included.

AdServices Attribution API is out of scope. It is not part of the Apple Ads
Campaign Management API command surface and has different caller requirements.

## Platform API v1 in 4.4.0

The cumulative 4.4.0 implementation stack is intended to add all 99 documented
operations under direct `asc ads` resource paths: this foundation layer registers
13 operations, the campaign layer adds 41, Maps and assets add 21, and reports
and optimization add 24. At each intermediate PR, only the operations registered
by that layer are available; the cumulative stack is what provides the complete
surface.
Ad-account context is carried in
`X-AP-Context: adAccountId=<ad-account-id>;` for account-scoped requests. The
`--ad-account` flag, `ASC_ADS_AD_ACCOUNT_ID`, and the selected profile's
`ad_account_id` are independent of the legacy v5 `--org`/`org_id` context.

The `/v1/ad-accounts` collection is method-dependent:

| Operation | Context behavior |
| --- | --- |
| `POST /v1/ad-accounts` | Creates an account without `X-AP-Context`; the account context does not exist until the response supplies its ID. |
| `GET /v1/ad-accounts/{id}` | Requires `X-AP-Context: adAccountId=<id>;`; the header account must match the path ID. |
| `PUT /v1/ad-accounts/{id}` | Requires `X-AP-Context: adAccountId=<id>;`; the header account must match the path ID. |

Authentication commands use the Platform API v1 transport:

| Command | Validation/discovery request |
| --- | --- |
| `asc ads auth login --network` | OAuth token exchange, then Platform API v1 `GET /v1/me`. |
| `asc ads auth status --validate` | For each stored credential, OAuth token exchange, then Platform API v1 `GET /v1/me`. |
| `asc ads auth discover` | Platform API v1 `GET /v1/me` and `GET /v1/acls`, returning user and ad-account access. |

All three commands avoid an ad-account context for these requests. The token
exchange remains `POST https://appleid.apple.com/auth/oauth2/token` with the
`searchadsorg` scope; a supplied access token skips that exchange. Platform API
v1 users should call `asc ads me view` and `asc ads acls list`
when they need the v1 user or ACL resources.

## Historical v5 command placement

The original implementation added this command root; in 4.4.0 the historical
v5 leaves move beneath an explicit version group:

```text
asc ads v5 <subcommand> [flags]
```

Common endpoint flags:

- Every Apple Ads endpoint leaf command accepts `--ads-profile NAME`.
- Every org-scoped endpoint leaf command accepts `--org ORG_ID`.
- `asc ads v5 me view`, `asc ads v5 acls list`, and `asc ads auth ...` do not require
  `--org`.
- Examples in the endpoint matrix omit `--org` and `--ads-profile` unless those
  flags are materially relevant to the endpoint. The implementation still adds
  them to the command.
- Resource groups with a natural list endpoint execute list by default:
  `asc ads v5 campaigns`, `asc ads v5 budget-orders`, `asc ads v5 ad-groups`,
  `asc ads v5 creatives`, and `asc ads v5 impression-share-reports`.

These rules describe the deprecated v5 tree. Platform v1 commands live under
direct `asc ads`, use `--ad-account` instead of `--org`, and follow the v1
payload and response contracts documented in
`docs/design/apple-ads-platform-api-v1.md`.

Root help placement:

- Add `ads` to `cmd/root_usage.go` under `ANALYTICS & FINANCE COMMANDS` after
  `performance`. This is acquisition/marketing rather than App Store Connect
  resource management, but this is the closest existing command group.
- Register `ads.AdsCommand()` in `internal/cli/registry/registry.go`.
- Add `ads` to registry/root help tests.

Package layout:

```text
internal/appleads/
  auth.go
  auth_store.go
  client.go
  client_test.go
  endpoints.go
  endpoints_test.go
  errors.go
  pagination.go

internal/cli/ads/
  api.go
  root.go
  auth.go
  endpoints.go
  endpoints_test.go
  resolve.go
```

Use `internal/appleads`, not `internal/asc`, because the base URL, auth flow,
error envelope, and pagination model are different from App Store Connect.

Do not implement this as 73 bespoke command functions and 73 bespoke client
methods. Add one `EndpointSpec` table in `internal/appleads/endpoints.go` and
generic command/client builders that consume it.

`EndpointSpec` must include:

```go
type EndpointSpec struct {
	Name             string
	Method           string
	Path             string
	CommandPath      []string
	BodyKind         shared.JSONPayloadKind
	BodyType         string
	ResponseType     string
	RequiresOrg      bool
	RequiresConfirm  bool
	PathParams       []ParamSpec
	QueryParams      []ParamSpec
	SupportsPaginate bool
	DefaultListAlias bool
}

type ParamSpec struct {
	Name      string
	Flag      string
	Type      ParamType
	Required  bool
	Max       int
	Allowed   []string
}
```

Acceptance check: adding a newly documented Apple Ads endpoint requires one
spec row plus docs/tests, not a new hand-written request method.

## Authentication Contract

Apple Ads OAuth is separate from App Store Connect JWT auth.

OAuth facts from Apple:

- Token URL: `https://appleid.apple.com/auth/oauth2/token`
- Grant type: `client_credentials`
- Scope: `searchadsorg`
- Client secret: ES256 JWT
- JWT header: `alg=ES256`, `kid=<key-id>`
- JWT claims:
  - `iss=<team-id>`
  - `iat=<issued-at>`
  - `exp=<expires-at>`
  - `aud=https://appleid.apple.com`
  - `sub=<client-id>`
- Token response contains `access_token`, `token_type=Bearer`,
  `expires_in=3600`, and `scope=searchadsorg`.
- Generate the client secret on demand for each token refresh with a 10-minute
  lifetime and 30-second refresh skew. Do not persist the client secret.
- Send the token request as `application/x-www-form-urlencoded` form values:
  `grant_type`, `client_id`, `client_secret`, and `scope`.

Implement this command group:

```text
asc ads auth login --name NAME --client-id CLIENT_ID --team-id TEAM_ID --key-id KEY_ID --private-key PATH [--org ORG_ID] [--ad-account AD_ACCOUNT_ID] [--network] [--skip-validation] [--bypass-keychain] [--local]
asc ads auth status [--verbose] [--validate] [--output table|json]
asc ads auth discover [--ads-profile NAME] [--org ORG_ID] [--ad-account AD_ACCOUNT_ID] [--output table|json]
asc ads auth switch --name NAME
asc ads auth token --confirm [--output text|json]
asc ads auth doctor [--output text|json]
asc ads auth logout [--all --confirm | --name NAME]
```

Mirror the existing `asc auth` behavior:

- `--name`, `--key-id`, and `--private-key` use the same meaning and validation
  style as `asc auth login`.
- `--client-id` is Apple Ads OAuth `client_id`.
- `--team-id` is the JWT issuer (`iss`).
- `--org` stores a default Apple Ads org ID for API calls.
- `--ad-account` stores a default Platform API v1 ad-account ID and, on
  `auth discover`, marks that account as active in the output. It is separate
  from `--org`.
- `--private-key` accepts the EC P-256 PEM Apple documents for Ads. Reuse the
  existing private-key parsing helpers because they already support ES256 keys.
- `--network` requests an access token and calls Platform API v1 `GET /v1/me`.
- `--skip-validation` skips JWT and network validation.
- `--network` and `--skip-validation` are mutually exclusive.
- `--local` requires keychain bypass, exactly like `asc auth login`.
- Keychain is preferred; config fallback is allowed when bypassing keychain.
- `auth status` supports `--verbose` and `--validate`, matching `asc auth status`.
- `auth status --validate` validates each stored credential through Platform
  API v1 `GET /v1/me` and reports failures after rendering the status output.
- `auth discover` calls Platform API v1 `/v1/me` and `/v1/acls` to show the
  active Ads user and available ad accounts without printing access tokens.
  Each ACL row includes its honest `ad_account_id`; `--ad-account` controls
  which row is marked active.
- Platform API v1 user and ACL discovery is also available directly through
  `asc ads me view` and `asc ads acls list`.
- `auth logout` supports `--all` and `--name`. It requires one of those flags
  so bare `asc ads auth logout` does not clear every stored Ads profile, and
  `--all` requires `--confirm`.

Add config fields without changing existing App Store Connect credentials:

```go
type AdsCredential struct {
	Name           string `json:"name"`
	ClientID       string `json:"client_id"`
	TeamID         string `json:"team_id"`
	KeyID          string `json:"key_id"`
	PrivateKeyPath string `json:"private_key_path"`
	OrgID          string `json:"org_id,omitempty"`
	AdAccountID    string `json:"ad_account_id,omitempty"`
}

type AdsKeychainMetadata struct {
	Name       string `json:"name"`
	ClientID   string `json:"client_id"`
	TeamID     string `json:"team_id"`
	KeyID      string `json:"key_id"`
	OrgID      string `json:"org_id,omitempty"`
	AdAccountID string `json:"ad_account_id,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

type AdsConfig struct {
	DefaultKeyName   string                `json:"default_key_name,omitempty"`
	Keys             []AdsCredential       `json:"keys,omitempty"`
	KeychainMetadata []AdsKeychainMetadata `json:"keychain_metadata,omitempty"`
	OrgID            string                `json:"org_id,omitempty"`
	AdAccountID      string                `json:"ad_account_id,omitempty"`
}

type Config struct {
	// existing fields...
	Ads AdsConfig `json:"ads,omitempty"`
}
```

Do not reuse the existing App Store Connect keychain item prefixes. Ads storage
uses:

```text
asc:ads-credential:<name>
asc:ads-metadata:<name>
```

Reuse only private-key parsing/validation helpers from `internal/auth`.

Environment variables:

```text
ASC_ADS_ACCESS_TOKEN
ASC_ADS_CLIENT_ID
ASC_ADS_TEAM_ID
ASC_ADS_KEY_ID
ASC_ADS_PRIVATE_KEY_PATH
ASC_ADS_PRIVATE_KEY
ASC_ADS_PRIVATE_KEY_B64
ASC_ADS_ORG_ID
ASC_ADS_AD_ACCOUNT_ID
ASC_ADS_PROFILE
ASC_ADS_STRICT_AUTH
ASC_ADS_BYPASS_KEYCHAIN
```

Resolution order:

1. Explicit `--ads-profile` if present.
2. `ASC_ADS_PROFILE` if present.
3. `ASC_ADS_ACCESS_TOKEN` if no profile is selected; it bypasses token exchange but still needs
   `--org`, `ASC_ADS_ORG_ID`, or stored profile org for org-scoped calls.
4. Complete Ads env credential tuple.
5. Default Ads keychain/config profile.

If `ASC_ADS_STRICT_AUTH` is true, fail when Ads credentials are split across
multiple sources, matching the existing strict-auth principle.

If a profile is selected by `--ads-profile` or `ASC_ADS_PROFILE`, ignore
`ASC_ADS_ACCESS_TOKEN` unless `ASC_ADS_STRICT_AUTH` is true; in strict mode,
selected profile plus access token is a mixed-source error.

Org ID resolution is independent from token resolution:

1. `--org`
2. `ASC_ADS_ORG_ID`
3. selected Ads profile `org_id`
4. `ads.org_id` in config only when authentication is profile-less

Persist the org ID both on the selected credential and in `ads.org_id` when Ads
login receives an org ID. This lets
`ASC_ADS_ACCESS_TOKEN` users reuse a configured default org without storing Ads
private key material in the active environment. A named profile never inherits
either root Ads context when its own profile omits the value. Switching or
removing the default clears both root context values. Profile-less access-token
or environment authentication can still use the standalone root values.

Platform API v1 ad-account resolution is independent from org resolution:

1. `--ad-account`
2. `ASC_ADS_AD_ACCOUNT_ID`
3. selected Ads profile `ad_account_id`
4. `ads.ad_account_id` in config when authentication does not select a named profile

Persist an ad-account ID supplied to Ads login alongside the selected profile
and root config. A named profile never inherits the root ad-account ID from
another profile.

## Campaign Management API v5 HTTP Client Contract

This section applies only to the deprecated Campaign Management API v5
commands under `asc ads v5`. Platform API v1 has a separate transport contract
below.

Base URL:

```text
https://api.searchads.apple.com/api/
```

Headers:

```text
Authorization: Bearer <access_token>
Accept: application/json
Content-Type: application/json
X-AP-Context: orgId=<org-id>
```

Do not send `X-AP-Context` for these two endpoints:

- `GET /v5/acls`
- `GET /v5/me`

For all other endpoints, resolve org ID from `--org`, `ASC_ADS_ORG_ID`, the
selected Ads profile, or `ads.org_id` in config. If it is still empty, fail
with:

```text
Error: --org is required (or set ASC_ADS_ORG_ID or an Ads profile org_id)
```

Use `shared.ContextWithTimeout` for all Apple Ads API requests so `ASC_TIMEOUT`
continues to apply.

Use the existing retry/backoff approach for GET and HEAD requests and for 429
responses. Redact `Authorization`, `client_secret`, private keys, and access
tokens in `ASC_DEBUG=api` logging.

Apple Ads error envelope:

```json
{
  "error": {
    "errors": [
      {
        "field": "name",
        "message": "message",
        "messageCode": "CODE"
      }
    ]
  }
}
```

Add `appleads.APIError` with HTTP status, field, message, and messageCode. If
the response is not this envelope, sanitize and return the raw detail the same
way `internal/asc` does.

## Platform API v1 HTTP Client Contract

Base URL:

```text
https://api.ads.apple.com/v1/
```

Platform API v1 uses the same bearer authorization and JSON content headers as
v5. Account-scoped requests add:

```text
X-AP-Context: adAccountId=<ad-account-id>;
```

The ad-account ID is resolved independently from the legacy organization ID.
`--ad-account`, `ASC_ADS_AD_ACCOUNT_ID`, the selected profile's
`ad_account_id`, and root `ads.ad_account_id` are the supported sources. The
CLI rejects control characters and semicolons before authentication or network
work so an ID cannot inject additional context fields.

The following requests are context-free: `GET /v1/me`, `GET /v1/acls`,
`GET /v1/orgs/{id}`, `GET /v1/advertiser-resources`, and
`POST /v1/ad-accounts`. For `GET` and `PUT /v1/ad-accounts/{id}`, the context
account must match the path ID. Other
endpoint context requirements are declared by the v1 endpoint metadata.

Platform API v1 retains the shared timeout, retry, rate-limit, pagination, and
sanitized error handling behavior of the client, while selecting the v1 base
URL and endpoint-specific response envelope.

## Payload Contract

Every Apple Ads endpoint with an HTTP body uses `--file`.

Reason: Apple Ads request objects are large, nested, API-specific JSON payloads.
Using `--file` is already established in this repo for complex JSON payloads
such as Xcode Cloud workflows. It also preserves full API support without
mapping every Apple Ads nested object into unstable CLI flags.

Rules:

- The CLI sends the file content as the HTTP body without wrapping it.
- Object endpoints require a JSON object file.
- Array endpoints require a JSON array file.
- `(CustomProductPageCreative | DefaultProductPageCreative)` accepts a JSON
  object file.
- `[int64]` delete-bulk endpoints accept a JSON array of numbers.
- Do not implement `--file -` in this PR.

Extend `internal/cli/shared/json_payload.go`:

```go
type JSONPayloadKind string

const (
	JSONPayloadObject JSONPayloadKind = "object"
	JSONPayloadArray  JSONPayloadKind = "array"
	JSONPayloadAny    JSONPayloadKind = "any"
)

func ReadJSONFilePayloadKind(path string, kind JSONPayloadKind) (json.RawMessage, error)
```

Keep `ReadJSONFilePayload(path)` as the object-only compatibility wrapper.

Required errors:

```text
payload path must be a file
payload file is empty
invalid JSON: ...
payload must be a JSON object
payload must be a JSON array
```

## Pagination Contract

Apple Ads uses offset pagination, not App Store Connect `links.next`.

List/search commands with `limit`/`offset` support:

```text
--limit INT
--offset INT
--paginate
```

Do not add `--next` to Apple Ads commands. Validate:

- `--limit` must be `1..1000` except `GET v5/custom-reports`, where it must be
  `1..50`.
- `--offset` must be `>=0`.
- `--paginate` starts at `--offset` when provided.
- `--paginate` uses page size `--limit` when provided, otherwise `1000`
  except `GET v5/custom-reports`, where the default page size is `50`.
- Continue until `pagination.startIndex + pagination.itemsPerPage >= pagination.totalResults` or a page returns no data.

Do not add `--paginate` to `find` or report endpoints that carry pagination
inside a `Selector` or `ReportingRequest` JSON body. Those endpoints keep
pagination fully manual in the payload file. The CLI must not mutate payload
files or rewrite body JSON to advance selector pagination in this PR.

The generic paginated response shape must preserve the Apple envelope:

```json
{
  "data": [],
  "pagination": {
    "itemsPerPage": 1000,
    "startIndex": 0,
    "totalResults": 0
  }
}
```

## Output Contract

All endpoint commands bind:

```go
output := shared.BindOutputFlags(fs)
```

Return Apple Ads response JSON exactly as Apple returns it, including `data`,
`pagination`, and `error` envelopes. JSON is the canonical agent output.

Use the existing output-registry fallback for Apple Ads raw response types:
`table` and `markdown` print the same JSON envelope as `json` until a dedicated
renderer is added in a later PR. Do not add custom Apple Ads table/markdown
renderers in this PR.

Represent successful Apple Ads responses as `json.RawMessage` or a dedicated
raw envelope type that is not registered with the output registry.

## Historical v5 endpoint-to-command matrix

This matrix records the 100% v5 coverage implemented from Apple's May 2026
documentation. Every row remains a runnable deprecated compatibility command;
it is not the preferred Platform v1 command inventory.

| CLI command | HTTP endpoint | Body | Notes |
| --- | --- | --- | --- |
| `asc ads v5 acls list` | `GET v5/acls` | none | No `--org` header. |
| `asc ads v5 me view` | `GET v5/me` | none | No `--org` header. |
| `asc ads v5 apps search --query QUERY [--limit N --offset N --paginate --return-owned-apps]` | `GET v5/search/apps` | none | `query` is required. |
| `asc ads v5 apps view --adam-id ADAM_ID` | `GET v5/apps/{adamId}` | none | `adamId` is int64. |
| `asc ads v5 apps localized-details --adam-id ADAM_ID` | `GET v5/apps/{adamId}/locale-details` | none |  |
| `asc ads v5 apps eligibility find --adam-id ADAM_ID --file selector.json` | `POST v5/apps/{adamId}/eligibilities/find` | `Selector` object |  |
| `asc ads v5 apps assets find --adam-id ADAM_ID --file selector.json` | `POST v5/apps/{adamId}/assets/find` | `Selector` object |  |
| `asc ads v5 product-pages list --adam-id ADAM_ID [--name NAME --states STATES]` | `GET v5/apps/{adamId}/product-pages` | none | Forward `states` as raw query string. |
| `asc ads v5 product-pages view --adam-id ADAM_ID --product-page PRODUCT_PAGE_ID` | `GET v5/apps/{adamId}/product-pages/{productPageId}` | none |  |
| `asc ads v5 product-pages locales list --adam-id ADAM_ID --product-page PRODUCT_PAGE_ID [--device-classes VALUE --language-codes VALUE --languages VALUE --expand]` | `GET v5/apps/{adamId}/product-pages/{productPageId}/locale-details` | none | Forward query strings exactly. |
| `asc ads v5 product-pages countries list [--countries-or-regions VALUE]` | `GET v5/countries-or-regions` | none |  |
| `asc ads v5 product-pages devices list` | `GET v5/creativeappmappings/devices` | none |  |
| `asc ads v5 budget-orders list [--limit N --offset N --paginate]` | `GET v5/budgetorders` | none |  |
| `asc ads v5 budget-orders create --file budget-order-create.json --confirm` | `POST v5/budgetorders` | `BudgetOrderCreate` object | Require `--confirm`. |
| `asc ads v5 budget-orders view --budget-order BUDGET_ORDER_ID` | `GET v5/budgetorders/{boId}` | none |  |
| `asc ads v5 budget-orders update --budget-order BUDGET_ORDER_ID --file budget-order-update.json --confirm` | `PUT v5/budgetorders/{boId}` | `BudgetOrderUpdate` object | Require `--confirm`. |
| `asc ads v5 campaigns list [--limit N --offset N --paginate]` | `GET v5/campaigns` | none | `asc ads v5 campaigns` aliases list. |
| `asc ads v5 campaigns find --file selector.json` | `POST v5/campaigns/find` | `Selector` object |  |
| `asc ads v5 campaigns view --campaign CAMPAIGN_ID` | `GET v5/campaigns/{campaignId}` | none |  |
| `asc ads v5 campaigns create --file campaign.json --confirm` | `POST v5/campaigns` | `Campaign` object | Require `--confirm`. |
| `asc ads v5 campaigns update --campaign CAMPAIGN_ID --file campaign-update.json --confirm` | `PUT v5/campaigns/{campaignId}` | `UpdateCampaignRequest` object | Require `--confirm`; campaign update uses Apple's campaign envelope. |
| `asc ads v5 campaigns delete --campaign CAMPAIGN_ID --confirm` | `DELETE v5/campaigns/{campaignId}` | none | Require `--confirm`. |
| `asc ads v5 ad-groups list --campaign CAMPAIGN_ID [--limit N --offset N --paginate]` | `GET v5/campaigns/{campaignId}/adgroups` | none | `asc ads v5 ad-groups` aliases list. |
| `asc ads v5 ad-groups find --campaign CAMPAIGN_ID --file selector.json` | `POST v5/campaigns/{campaignId}/adgroups/find` | `Selector` object |  |
| `asc ads v5 ad-groups find-org --file selector.json` | `POST v5/adgroups/find` | `Selector` object | Org-level find. |
| `asc ads v5 ad-groups view --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID` | `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}` | none |  |
| `asc ads v5 ad-groups create --campaign CAMPAIGN_ID --file ad-group.json --confirm` | `POST v5/campaigns/{campaignId}/adgroups` | `AdGroup` object | Require `--confirm`. |
| `asc ads v5 ad-groups update --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file ad-group-update.json --confirm` | `PUT v5/campaigns/{campaignId}/adgroups/{adgroupId}` | `AdGroupUpdate` object | Require `--confirm`. |
| `asc ads v5 ad-groups delete --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --confirm` | `DELETE v5/campaigns/{campaignId}/adgroups/{adgroupId}` | none | Require `--confirm`. |
| `asc ads v5 ads list --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID` | `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads` | none |  |
| `asc ads v5 ads find --campaign CAMPAIGN_ID --file selector.json` | `POST v5/campaigns/{campaignId}/ads/find` | `Selector` object | Campaign-level find. |
| `asc ads v5 ads find-org --file selector.json` | `POST v5/ads/find` | `Selector` object | Org-level find. |
| `asc ads v5 ads view --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --ad AD_ID` | `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads/{adId}` | none |  |
| `asc ads v5 ads create --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file ad-create.json --confirm` | `POST v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads` | `AdCreate` object | Require `--confirm`. |
| `asc ads v5 ads update --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --ad AD_ID --file ad-update.json --confirm` | `PUT v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads/{adId}` | `AdUpdate` object | Require `--confirm`. |
| `asc ads v5 ads delete --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --ad AD_ID --confirm` | `DELETE v5/campaigns/{campaignId}/adgroups/{adgroupId}/ads/{adId}` | none | Require `--confirm`. |
| `asc ads v5 targeting-keywords list --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID [--limit N --offset N --paginate]` | `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords` | none |  |
| `asc ads v5 targeting-keywords find --campaign CAMPAIGN_ID --file selector.json` | `POST v5/campaigns/{campaignId}/adgroups/targetingkeywords/find` | `Selector` object | Campaign-level find across ad groups. |
| `asc ads v5 targeting-keywords view --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --keyword KEYWORD_ID` | `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/{keywordId}` | none |  |
| `asc ads v5 targeting-keywords create-bulk --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file keywords.json --confirm` | `POST v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/bulk` | `[Keyword]` array | Require `--confirm`. |
| `asc ads v5 targeting-keywords update-bulk --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file keywords-update.json --confirm` | `PUT v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/bulk` | `[KeywordUpdateRequest]` array | Require `--confirm`. |
| `asc ads v5 targeting-keywords delete --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --keyword KEYWORD_ID --confirm` | `DELETE v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/{keywordId}` | none | Require `--confirm`. |
| `asc ads v5 targeting-keywords delete-bulk --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file keyword-ids.json --confirm` | `POST v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords/delete/bulk` | `[int64]` array | Require `--confirm`. |
| `asc ads v5 campaign-negative-keywords list --campaign CAMPAIGN_ID [--limit N --offset N --paginate]` | `GET v5/campaigns/{campaignId}/negativekeywords` | none |  |
| `asc ads v5 campaign-negative-keywords find --campaign CAMPAIGN_ID --file selector.json` | `POST v5/campaigns/{campaignId}/negativekeywords/find` | `Selector` object |  |
| `asc ads v5 campaign-negative-keywords view --campaign CAMPAIGN_ID --negative-keyword KEYWORD_ID` | `GET v5/campaigns/{campaignId}/negativekeywords/{keywordId}` | none |  |
| `asc ads v5 campaign-negative-keywords create-bulk --campaign CAMPAIGN_ID --file negative-keywords.json --confirm` | `POST v5/campaigns/{campaignId}/negativekeywords/bulk` | `[NegativeKeyword]` array | Require `--confirm`. |
| `asc ads v5 campaign-negative-keywords update-bulk --campaign CAMPAIGN_ID --file negative-keywords.json --confirm` | `PUT v5/campaigns/{campaignId}/negativekeywords/bulk` | `[NegativeKeyword]` array | Require `--confirm`. |
| `asc ads v5 campaign-negative-keywords delete-bulk --campaign CAMPAIGN_ID --file keyword-ids.json --confirm` | `POST v5/campaigns/{campaignId}/negativekeywords/delete/bulk` | `[int64]` array | Require `--confirm`. |
| `asc ads v5 ad-group-negative-keywords list --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID [--limit N --offset N --paginate]` | `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords` | none |  |
| `asc ads v5 ad-group-negative-keywords find --campaign CAMPAIGN_ID --file selector.json` | `POST v5/campaigns/{campaignId}/adgroups/negativekeywords/find` | `Selector` object | Campaign-level find across ad groups. |
| `asc ads v5 ad-group-negative-keywords view --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --negative-keyword KEYWORD_ID` | `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords/{keywordId}` | none |  |
| `asc ads v5 ad-group-negative-keywords create-bulk --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file negative-keywords.json --confirm` | `POST v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords/bulk` | `[NegativeKeyword]` array | Require `--confirm`. |
| `asc ads v5 ad-group-negative-keywords update-bulk --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file negative-keywords.json --confirm` | `PUT v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords/bulk` | `[NegativeKeyword]` array | Require `--confirm`. |
| `asc ads v5 ad-group-negative-keywords delete-bulk --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file keyword-ids.json --confirm` | `POST v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords/delete/bulk` | `[int64]` array | Require `--confirm`. |
| `asc ads v5 geo search [--country-code CC --entity ENTITY --query QUERY --limit N --offset N --paginate]` | `GET v5/search/geo` | none | API default query is `*:*`. |
| `asc ads v5 geo resolve --file geo-requests.json [--limit N --offset N --paginate]` | `POST v5/search/geo` | `[GeoRequest]` array |  |
| `asc ads v5 creatives list [--limit N --offset N --paginate]` | `GET v5/creatives` | none |  |
| `asc ads v5 creatives find --file selector.json` | `POST v5/creatives/find` | `Selector` object |  |
| `asc ads v5 creatives view --creative CREATIVE_ID [--include-deleted-creative-set-assets]` | `GET v5/creatives/{creativeId}` | none | Include deprecated creative set assets query. |
| `asc ads v5 creatives create --file creative.json` | `POST v5/creatives` | `CustomProductPageCreative` or `DefaultProductPageCreative` object |  |
| `asc ads v5 rejection-reasons find --file selector.json` | `POST v5/product-page-reasons/find` | `Selector` object |  |
| `asc ads v5 rejection-reasons view --reason PRODUCT_PAGE_REASON_ID` | `GET v5/product-page-reasons/{productPageReasonId}` | none |  |
| `asc ads v5 reports campaigns --file reporting-request.json` | `POST v5/reports/campaigns` | `ReportingRequest` object |  |
| `asc ads v5 reports ad-groups --campaign CAMPAIGN_ID --file reporting-request.json` | `POST v5/reports/campaigns/{campaignId}/adgroups` | `ReportingRequest` object |  |
| `asc ads v5 reports keywords --campaign CAMPAIGN_ID --file reporting-request.json` | `POST v5/reports/campaigns/{campaignId}/keywords` | `ReportingRequest` object |  |
| `asc ads v5 reports search-terms --campaign CAMPAIGN_ID --file reporting-request.json` | `POST v5/reports/campaigns/{campaignId}/searchterms` | `ReportingRequest` object |  |
| `asc ads v5 reports ads --campaign CAMPAIGN_ID --file reporting-request.json` | `POST v5/reports/campaigns/{campaignId}/ads` | `ReportingRequest` object |  |
| `asc ads v5 reports ad-group-keywords --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file reporting-request.json` | `POST v5/reports/campaigns/{campaignId}/adgroups/{adgroupId}/keywords` | `ReportingRequest` object |  |
| `asc ads v5 reports ad-group-search-terms --campaign CAMPAIGN_ID --ad-group AD_GROUP_ID --file reporting-request.json` | `POST v5/reports/campaigns/{campaignId}/adgroups/{adgroupId}/searchterms` | `ReportingRequest` object |  |
| `asc ads v5 impression-share-reports list [--field FIELD --sort-order ORDER --limit N --offset N --paginate]` | `GET v5/custom-reports` | none |  |
| `asc ads v5 impression-share-reports create --file custom-report-request.json` | `POST v5/custom-reports` | `CustomReportRequest` object |  |
| `asc ads v5 impression-share-reports view --report REPORT_ID` | `GET v5/custom-reports/{reportId}` | none |  |

The `EndpointSpec` query parameter metadata matched Apple's v5 documentation at
the research date for every row. Required query parameters were:

| HTTP endpoint | Query flags |
| --- | --- |
| `GET v5/search/apps` | `--query` required, `--limit`, `--offset`, `--return-owned-apps` |
| `GET v5/search/geo` | `--country-code`, `--entity`, `--query`, `--limit`, `--offset` |
| `POST v5/search/geo` | `--limit`, `--offset` |
| `GET v5/campaigns` | `--limit`, `--offset` |
| `GET v5/budgetorders` | `--limit`, `--offset` |
| `GET v5/campaigns/{campaignId}/adgroups` | `--limit`, `--offset` |
| `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}/targetingkeywords` | `--limit`, `--offset` |
| `GET v5/campaigns/{campaignId}/adgroups/{adgroupId}/negativekeywords` | `--limit`, `--offset` |
| `GET v5/campaigns/{campaignId}/negativekeywords` | `--limit`, `--offset` |
| `GET v5/creatives` | `--limit`, `--offset` |
| `GET v5/custom-reports` | `--field`, `--sort-order`, `--limit`, `--offset` |
| `GET v5/apps/{adamId}/product-pages` | `--name`, `--states` |
| `GET v5/apps/{adamId}/product-pages/{productPageId}/locale-details` | `--device-classes`, `--expand`, `--language-codes`, `--languages` |
| `GET v5/countries-or-regions` | `--countries-or-regions` |
| `GET v5/creatives/{creativeId}` | `--include-deleted-creative-set-assets` |

Apple's general partial-fetch `fields` query parameter was not exposed in the
original v5 implementation because those endpoint pages did not list
endpoint-specific `fields[...]` query parameters. Add it only if the
compatibility implementation extracts a documented endpoint-specific query
parameter from Apple docs.

Add this debug/forward-compatibility command after the named endpoints:

```text
asc ads v5 api request --method METHOD --path v5/... [--file payload.json] [--org ORG_ID] [--confirm]
```

Rules:

- `--path` accepts only relative `v5/...` paths or full URLs under
  `https://api.searchads.apple.com/api/`.
- Full URLs for other hosts are rejected.
- `DELETE` requires `--confirm`.
- This command is not a substitute for any named endpoint above; all rows above
  must still exist.

## Request Body Reference

The following body types are accepted through `--file` and must be validated
only for JSON shape, not for every semantic field. Semantic validation belongs
to Apple Ads.

| Body type | JSON shape | Required top-level fields from Apple docs |
| --- | --- | --- |
| `Selector` | object | none; common fields are `conditions`, `fields`, `orderBy`, `pagination` |
| `Condition` | object | none; common fields are `field`, `operator`, `values`, `ignoreCase` |
| `BudgetOrderCreate` | object | none documented as top-level required |
| `BudgetOrderUpdate` | object | none documented as top-level required |
| `Campaign` | object | `adamId`, `adChannelType`, `billingEvent`, `countriesOrRegions`, `dailyBudgetAmount`, `name`, `supplySources` |
| `UpdateCampaignRequest` | object | none documented as top-level required; payload uses `campaign` envelope |
| `AdGroup` | object | `defaultBidAmount`, `name`, `pricingModel`, `startTime` |
| `AdGroupUpdate` | object | none documented as top-level required |
| `Keyword` | object array item | `bidAmount`, `matchType`, `text` |
| `KeywordUpdateRequest` | object array item | `matchType`, `text` |
| `NegativeKeyword` | object array item | `matchType`, `text` |
| `AdCreate` | object | `creativeId`, `name`, `status` |
| `AdUpdate` | object | none documented as top-level required |
| `CustomProductPageCreative` | object | `productPageId`, `adamId`, `name`, `type` |
| `DefaultProductPageCreative` | object | `adamId`, `name`, `type` |
| `ReportingRequest` | object | `startTime`, `endTime`, `selector` |
| `CustomReportRequest` | object | `name` |
| `GeoRequest` | object array item | `entity`, `id` |
| `[int64]` | array | integer IDs |

Payload examples:

```json
{
  "conditions": [
    {
      "field": "campaignId",
      "operator": "EQUALS",
      "values": ["1234567890"]
    }
  ],
  "pagination": {
    "limit": 1000,
    "offset": 0
  }
}
```

```json
{
  "adamId": 123456789,
  "adChannelType": "SEARCH",
  "billingEvent": "TAPS",
  "countriesOrRegions": ["US"],
  "dailyBudgetAmount": {
    "amount": "25",
    "currency": "USD"
  },
  "name": "US Search Campaign",
  "status": "ENABLED",
  "supplySources": ["APPSTORE_SEARCH_RESULTS"]
}
```

```json
[
  {
    "text": "example keyword",
    "matchType": "BROAD",
    "bidAmount": {
      "amount": "1.25",
      "currency": "USD"
    },
    "status": "ACTIVE"
  }
]
```

```json
{
  "startTime": "2026-05-01",
  "endTime": "2026-05-31",
  "granularity": "DAILY",
  "selector": {
    "pagination": {
      "limit": 1000,
      "offset": 0
    }
  },
  "timeZone": "UTC"
}
```

## Implementation Sequence

The PR implemented the work in this order:

1. Added `internal/appleads` auth, token, client, error, and pagination tests.
2. Extended `shared.ReadJSONFilePayload` with array/object/any shape support
   while preserving the existing object-only wrapper.
3. Added `internal/cli/ads` root, auth commands, and registry/root help wiring.
4. Added read-only endpoints first: `me`, `acls`, app search/details, product
   pages, countries, devices, list/view commands.
5. Added request-body commands with `--file` and shape validation.
6. Added deletes and bulk deletes with `--confirm`.
7. Added reports and impression-share reports.
8. Added `ads api request`.
9. Added command docs and generated command docs.

Do not add ergonomic create/update flags in the first PR. They would duplicate
large Apple request schemas and weaken the guarantee that every API field can be
used by agents immediately. Ergonomic wrappers can be a later additive PR.

## Test Plan

Use TDD for implementation. Start with failing tests.

Required unit tests:

- Apple Ads client secret JWT includes `alg=ES256`, `kid`, `iss`, `iat`, `exp`,
  `aud=https://appleid.apple.com`, and `sub=<client-id>`.
- Token request uses `POST https://appleid.apple.com/auth/oauth2/token` with
  `grant_type=client_credentials`, `client_id`, `client_secret`, and
  `scope=searchadsorg`.
- Access token cache refreshes before expiry.
- `ASC_ADS_ACCESS_TOKEN` bypasses token exchange.
- `X-AP-Context` is present for org-scoped endpoints and absent for `me`/`acls`.
- Missing org fails before auth resolution.
- Apple Ads error envelopes parse into `appleads.APIError`.
- Offset pagination aggregates `data` and stops at `totalResults`.
- JSON payload helper accepts object, array, and any modes; rejects wrong shapes.

Required CLI tests under `internal/cli/cmdtest`:

- Every command in the matrix is registered and has help output.
- Every leaf command validates missing required flags with exit code `2` and a
  concrete stderr message.
- Every command with `--file` rejects missing file, empty file, invalid JSON,
  and wrong object/array shape.
- Every delete and bulk delete rejects missing `--confirm` with exit code `2`.
- Every list/search command rejects invalid `--limit`, invalid `--offset`, and
  invalid boolean flags.
- Query parameters are encoded exactly as documented.
- Body payloads are forwarded byte-for-byte except for JSON validation.
- `--output json` returns parseable JSON for representative success responses.
- `--pretty` is accepted only for JSON.
- `--paginate` makes multiple requests with increasing `offset`.
- `ads api request` rejects non-Apple Ads hosts and requires `--confirm` for
  `DELETE`.

Required client tests:

- One table-driven test row for every `EndpointSpec` row that asserts method,
  path, path variables, query variables, body shape, and response passthrough.
- One generated inventory test that compares each `EndpointSpec` path, method,
  body kind, and query parameter names against the Apple docs snapshot used in
  the test fixture.
- 401, 403, 404, 429, and 500 response handling.
- Token endpoint errors do not print secrets.

Black-box verification:

```bash
go build -o /tmp/asc .
ASC_BYPASS_KEYCHAIN=1 /tmp/asc ads --help
ASC_BYPASS_KEYCHAIN=1 /tmp/asc ads v5 campaigns list --org 123 --output json
ASC_BYPASS_KEYCHAIN=1 /tmp/asc ads v5 campaigns delete --campaign 1 --org 123
ASC_BYPASS_KEYCHAIN=1 /tmp/asc ads v5 campaigns delete --campaign 1 --org 123 --confirm
```

Repository checks before PR:

```bash
make format
make check-command-docs
make lint
ASC_BYPASS_KEYCHAIN=1 make test
```

Live Apple Ads smoke tests are not required for PR completion when Ads
credentials are unavailable. When Ads credentials are configured locally, run
only these read-only smoke tests:

```bash
ASC_BYPASS_KEYCHAIN=1 asc ads v5 me view --output json
ASC_BYPASS_KEYCHAIN=1 asc ads v5 acls list --output json
ASC_BYPASS_KEYCHAIN=1 asc ads v5 campaigns list --org "$ASC_ADS_ORG_ID" --limit 1 --output json
```

Do not create spend-bearing Apple Ads campaigns in live smoke tests unless the
user explicitly approves the exact org and payload.

## Documentation Updates

Updated:

- `commands/ads.mdx`
- `authentication.mdx` with Apple Ads auth subsection
- `configuration/environment-variables.mdx` with `ASC_ADS_*` variables
- `docs.json` navigation for the new command page
- `docs/COMMANDS.md` via `make generate-command-docs`
- `README.md` feature list

Docs teach `--file` payloads with copy-pasteable JSON examples and state that
Apple Ads credentials are separate from App Store Connect API keys.

### Docs placement

`commands/ads.mdx` is the user guide. Keep it focused on auth, org context,
payload files, pagination rules, endpoint groups, and raw requests.

`configuration/environment-variables.mdx` owns the full `ASC_ADS_*` reference
and credential precedence.

This architecture note owns endpoint coverage, auth internals, generated
command strategy, test requirements, and live-smoke limits.

## Definition of Done

- All 73 current v5 endpoints in the matrix have named commands.
- `asc ads v5 api request` exists for debugging and newly added Apple fields.
- Apple Ads OAuth login/status/token/doctor/logout are implemented.
- All org-scoped commands require/respect `--org`.
- All body endpoints use `--file` and support the correct JSON shape.
- Deletes and bulk deletes require `--confirm`.
- Offset pagination works with `--paginate`.
- JSON output preserves Apple response envelopes.
- Generated command docs are updated.
- `make format`, `make check-command-docs`, `make lint`, and
  `ASC_BYPASS_KEYCHAIN=1 make test` pass.

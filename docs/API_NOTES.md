# API Notes

Quirks and tips for specific App Store Connect API endpoints.

## Analytics & Sales Reports

- Date formats vary by frequency:
  - DAILY/WEEKLY: `YYYY-MM-DD`
  - MONTHLY: `YYYY-MM`
  - YEARLY: `YYYY`
- Vendor number comes from Sales and Trends → Reports URL (`vendorNumber=...`)
- Use `--paginate` with `asc analytics get --date` to avoid missing instances on later pages
- Long analytics runs may require raising `ASC_TIMEOUT`

## Finance Reports

Finance reports use Apple fiscal months (`YYYY-MM`), not calendar months.

**API Report Types (mapping to App Store Connect UI):**

| API `--report-type` | UI Option                               | `--region` Code(s)      |
|---------------------|-----------------------------------------|-------------------------|
| `FINANCIAL`         | All Countries or Regions (Single File)  | `ZZ` (consolidated)     |
| `FINANCIAL`         | All Countries or Regions (Multiple Files) | `US`, `EU`, `JP`, etc. |
| `FINANCE_DETAIL`    | All Countries or Regions (Detailed)     | `Z1` (required)         |
| Not available       | Transaction Tax (Single File)           | N/A                     |

**Important:**
- `FINANCE_DETAIL` reports require region code `Z1` (the only valid region for detailed reports)
- Transaction Tax reports are NOT available via API; download manually from App Store Connect
- Region codes reference: https://developer.apple.com/help/app-store-connect/reference/financial-report-regions-and-currencies/
- Use `asc finance regions` to see all available region codes

## Sandbox Testers

- Required fields: email, first/last name, password + confirm, secret question/answer, birth date, territory
- Password must include uppercase, lowercase, and a number (8+ chars)
- Territory uses 3-letter App Store territory codes (e.g., `USA`, `JPN`)
- List/get use the v2 API; create/delete use v1 endpoints (may be unavailable on some accounts)
- Update/clear-history use the v2 API

## Game Center

- Most Game Center endpoints require a Game Center detail ID, resolved via `/v1/apps/{id}/gameCenterDetail`.
- If Game Center is not enabled for the app, the detail lookup returns 404.
- Releases are required to make achievements/leaderboards/leaderboard-sets live (create a release after creating the resource).
- Image uploads follow a three-step flow: reserve upload slot → upload file → commit upload (using upload operations).
- The `challengesMinimumPlatformVersions` relationship on `gameCenterDetails` uses `appStoreVersions` linkages (live API rejects `gameCenterAppVersions` for this relationship).
- The relationship endpoint is replace-only (PATCH); GET relationship requests are rejected with "does not allow 'GET_RELATIONSHIP'... Allowed operation is: REPLACE".
- Setting `challengesMinimumPlatformVersions` requires a live App Store version; non-live versions fail with `ENTITY_ERROR.RELATIONSHIP.INVALID.MIN_CHALLENGES_VERSION_MUST_BE_LIVE` ("must be live to be set as a minimum challenges version.").
- Concrete example (set members): `asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "lb-weekly,lb-alltime"`

## Authentication & Rate Limiting

- JWTs issued for App Store Connect are valid for 10 minutes (handled internally).
- Automatic retries apply only to GET/HEAD requests on 429/503 responses; POST/PATCH/DELETE are not retried.
- Retry-After headers are honored when present; configure retry settings via `ASC_MAX_RETRIES`, `ASC_BASE_DELAY`, `ASC_MAX_DELAY`, `ASC_RETRY_LOG`.
- Some endpoints return 403 when the API key role lacks permission (e.g., finance reports, reviews).

## Devices

- No DELETE endpoint; devices can only be enabled/disabled via PATCH.
- Registration requires a UDID (iOS) or Hardware UUID (macOS).
- Device management UI lives in the Apple Developer portal, not App Store Connect.
- Device reset is limited to once per membership year; disabling does not free slots.

## Pass Type IDs

- Live API rejects `include=passTypeId` and `fields[passTypeIds]` on `/v1/passTypeIds/{id}/certificates` despite the OpenAPI spec allowing them.
- The CLI does not expose those parameters for `pass-type-ids certificates list` to avoid API errors.

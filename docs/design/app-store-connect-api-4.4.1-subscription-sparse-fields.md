# App Store Connect API 4.4.1 subscription sparse fields

## Placement and command shape

This slice extends the existing subscription resource, offer, pricing, review,
and promoted-purchase commands. It does not add a command noun. The one
previously client-only relationship read is exposed through the established
scoped view command:

```text
asc subscriptions promoted-purchases view --subscription-id "SUBSCRIPTION_ID"
```

Related-resource sparse fields use the existing CLI naming convention:

- `--subscription-fields` maps to `fields[subscriptions]`.
- `--group-fields` maps to `fields[subscriptionGroups]`.
- `--price-point-fields` maps to `fields[subscriptionPricePoints]`.
- `--in-app-purchase-fields` maps to `fields[inAppPurchases]`.
- `--fields` remains the primary-resource sparse-field flag on
  `subscriptions pricing price-points view`.

Before this change, the affected commands had no way to request the new sparse
field values. List commands accepted `--next` but would silently discard any
new sparse-field options if they were sent through an opaque next URL.

## OpenAPI contract and implementation ledger

Apple's 4.4.1 schema changes only the following parameter enums for this slice.
All operations return their existing JSON:API response types.

| Done | Method and path | 4.4.1 sparse-field addition | CLI command |
| --- | --- | --- | --- |
| [x] | `GET /v1/subscriptionAppStoreReviewScreenshots/{id}` | `fields[subscriptions]=versions` | `subscriptions review screenshots view` |
| [x] | `GET /v1/subscriptionGroupLocalizations/{id}` | `fields[subscriptionGroups]=versions` | `subscriptions groups localizations view` |
| [x] | `GET /v1/subscriptionImages/{id}` | `fields[subscriptions]=versions` | `subscriptions images view` |
| [x] | `GET /v1/subscriptionLocalizations/{id}` | `fields[subscriptions]=versions` | `subscriptions localizations view` |
| [x] | `GET /v1/subscriptionOfferCodes/{id}` | `fields[subscriptions]=versions` | `subscriptions offers offer-codes view` |
| [x] | `GET /v1/subscriptionPricePoints/{id}` | `fields[subscriptionPricePoints]=adjustedEqualizations` | `subscriptions pricing price-points view` |
| [x] | `GET /v1/subscriptionPromotionalOffers/{id}` | `fields[subscriptions]=versions` | `subscriptions offers promotional view` |
| [x] | `GET /v1/subscriptionGroups/{id}/subscriptionGroupLocalizations` | `fields[subscriptionGroups]=versions` | `subscriptions groups localizations list` |
| [x] | `GET /v1/subscriptionOfferCodes/{id}/prices` | `fields[subscriptionPricePoints]=adjustedEqualizations` | `subscriptions offers offer-codes prices` |
| [x] | `GET /v1/subscriptionPromotionalOffers/{id}/prices` | `fields[subscriptionPricePoints]=adjustedEqualizations` | `subscriptions offers promotional prices` |
| [x] | `GET /v1/subscriptions/{id}/appStoreReviewScreenshot` | `fields[subscriptions]=versions` | `subscriptions review app-store-screenshot view` |
| [x] | `GET /v1/subscriptions/{id}/images` | `fields[subscriptions]=versions` | `subscriptions images list` |
| [x] | `GET /v1/subscriptions/{id}/introductoryOffers` | `fields[subscriptions]=versions`; `fields[subscriptionPricePoints]=adjustedEqualizations` | `subscriptions offers introductory list` |
| [x] | `GET /v1/subscriptions/{id}/offerCodes` | `fields[subscriptions]=versions` | `subscriptions offers offer-codes list` |
| [ ] | `GET /v1/subscriptions/{id}/promotedPurchase` | `fields[subscriptions]=versions`; `fields[inAppPurchases]=versions` | `subscriptions promoted-purchases view --subscription-id` |
| [x] | `GET /v1/subscriptions/{id}/promotionalOffers` | `fields[subscriptions]=versions` | `subscriptions offers promotional list` |
| [x] | `GET /v1/subscriptions/{id}/subscriptionLocalizations` | `fields[subscriptions]=versions` | `subscriptions localizations list` |

The CLI validates every sparse-field value against the exact enum for its
resource type before client creation. Related sparse fields automatically add
their required include relationship: `subscription`, `subscriptionGroup`,
`subscriptionPricePoint`, or `inAppPurchaseV2`. On paginated commands, an
explicit sparse-field flag conflicts with `--next`; the opaque URL remains the
only query source.

`subscriptions promoted-purchases view` accepts exactly one of its existing
`--promoted-purchase-id` selector or the new `--subscription-id` selector. The
existing direct-resource invocation remains unchanged; relationship sparse
fields are valid only with `--subscription-id`.

## Output, compatibility, and failure behavior

Output remains the existing TTY-aware JSON/table/Markdown behavior. Successful
reads exit zero. Invalid sparse fields, explicit empty values, missing IDs, and
`--next` conflicts are usage errors (exit code 2) written to stderr before auth
or HTTP. Existing invocations and response models are unchanged. Deprecated v1
localization and image commands remain available with their existing warnings.

## Verification plan

1. Establish RED with client HTTP tests for all 17 paths and CLI tests for each
   new flag family, auto-includes, invalid enums, explicit empties, and
   `--next` conflicts.
2. Implement endpoint-specific option types and query builders; do not use a
   universal query map.
3. Run focused `internal/asc`, `internal/cli/subscriptions`, and `cmdtest`
   packages, then build a binary and inspect the changed help and exit behavior.
4. Regenerate `docs/COMMANDS.md`; run format, docs, lint, and the full test gate.
5. Perform only a safe read-only App Store Connect smoke if credentials and a
   suitable existing resource are available. No live mutation is required.

## Alternatives considered

- A raw `map[string]string` query escape hatch would be smaller, but it would
  accept unsupported parameters and silently drift from endpoint-specific
  OpenAPI truth.
- Reusing one universal functional-option type across all 17 operations would
  allow invalid cross-endpoint combinations at compile time. Endpoint-specific
  option types preserve the API contract while sharing only private builders.

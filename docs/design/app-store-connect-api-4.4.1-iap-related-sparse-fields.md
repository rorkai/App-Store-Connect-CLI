# App Store Connect API 4.4.1 IAP related sparse fields

## Placement and command shape

This compatibility slice extends the existing IAP subresource and scoped
promoted-purchase commands. It does not add a command group or registry entry.

The public additions are endpoint-specific sparse-field flags:

- `asc iap review-screenshots view ... --iap-fields versions`
- `asc iap content view ... --iap-fields versions`
- `asc iap images list|view ... --iap-fields versions`
- `asc iap localizations list ... --iap-fields versions`
- `asc iap promoted-purchases list|view ... --iap-fields versions --subscription-fields versions`
- `asc subscriptions promoted-purchases list|view ... --iap-fields versions --subscription-fields versions`

The IAP-scoped promoted-purchase view also accepts
`--iap-id IAP_ID` as an alternative to `--promoted-purchase-id PROMO_ID` and
uses `/v2/inAppPurchases/{id}/promotedPurchase`. Exactly one selector is
required. The shared scoped-command hook is owner-agnostic so the subscription
slice can add its symmetric relationship selector without duplicating command
logic.

Both promoted-purchase command scopes expose both related-product sparse-field
flags because their shared API operations permit both fieldsets. The deprecated
product-scoped image and localization commands remain available and retain
their existing warnings and migration guidance.

## Exact API contracts

The 4.4.1 OpenAPI snapshot adds `versions` to `fields[inAppPurchases]` on these
GET operations:

- `/v1/inAppPurchaseAppStoreReviewScreenshots/{id}`
- `/v1/inAppPurchaseContents/{id}`
- `/v1/inAppPurchaseImages/{id}`
- `/v1/inAppPurchaseLocalizations/{id}`
- `/v1/promotedPurchases/{id}`
- `/v1/apps/{id}/promotedPurchases`
- `/v2/inAppPurchases/{id}/appStoreReviewScreenshot`
- `/v2/inAppPurchases/{id}/content`
- `/v2/inAppPurchases/{id}/images`
- `/v2/inAppPurchases/{id}/inAppPurchaseLocalizations`
- `/v2/inAppPurchases/{id}/promotedPurchase`

The three promoted-purchase operations also add `versions` to
`fields[subscriptions]`. Sparse fields are useful only when their related
resource is included, so the client builders automatically add the exact
relationship: `inAppPurchaseV2` for screenshots, content, localizations, and
promoted purchases; `inAppPurchase` for images; and `subscription` for
subscription fields. Callers may also request the endpoint's exact include
set explicitly through typed options.

Responses remain the existing JSON:API response types, including `included`,
`links`, and `meta`. The commands continue to write data to stdout and errors
to stderr with the existing output defaults.

## Validation and compatibility

CLI sparse-field values are checked against the endpoint's exact OpenAPI enum
before authentication, ID resolution, or HTTP. List commands reject combining
`--next` with sparse-field flags because the opaque next URL already owns its
query. Invalid selections and next conflicts are usage errors. Deprecated
commands are extended in place rather than removed.

## RED-GREEN and verification

RED coverage will assert exact query strings for all eleven client methods,
including automatic includes and both promoted-purchase fieldsets. CLI tests
will cover successful flags, invalid enums, opaque-next conflicts, and the
deprecated warning surface. The changed help will regenerate
`docs/COMMANDS.md`.

After focused tests, validation is `make format`, `make check-docs`,
`make lint`, and `ASC_BYPASS_KEYCHAIN=1 make test`, plus built-binary help and
error-stream checks. A read-only live smoke is optional; no mutation is needed
to validate query construction. The public IAP command page documents the
long-form flags and owner selector against the built help.

## Alternatives

A universal arbitrary-query map would be smaller but would accept unsupported
keys and values on unrelated endpoints. Extending only the top-level IAP reads
would also miss the exact 4.4.1 propagation change. Dedicated typed option
families keep the allowed query surface aligned with each endpoint while
preserving existing method and command compatibility.

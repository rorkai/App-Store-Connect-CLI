# StoreKit server actions

Extend the existing `storekit` command, preserving all read commands and separate
In-App Purchase credentials. Baseline help exposes transaction view/history and
subscription status, but none of the actions below.

## API and command contract

These endpoints belong to App Store Server API, not App Store Connect API; they
are absent from `docs/openapi/latest.json`. Contracts were checked against
Apple's endpoint documentation and official Node server library on 2026-10-23.
All four endpoints are available in production and sandbox.

| Command under `asc storekit` | Method and path |
| --- | --- |
| `app-transactions view --transaction-id ID` | GET `/inApps/v1/transactions/appTransactions/{anyTransactionId}` |
| `transactions set-account-token --original-transaction-id ID --app-account-token UUID --confirm` | PUT `/inApps/v1/transactions/{originalTransactionId}/appAccountToken` |
| `transactions send-consumption --transaction-id ID --customer-consented true --delivery-status DELIVERED --sample-content-provided false --confirm` | PUT `/inApps/v2/transactions/consumption/{transactionId}` |
| `transactions finish --transaction-id ID --confirm` | POST `/inApps/v1/transactions/{transactionId}/finish` |

Every invocation also selects `--environment production|sandbox` (or the
existing environment variable). Reads preserve Apple's JSON envelope and support
table/Markdown. App transaction reads reuse the existing optional, explicitly
unverified `--decode`. The signed app transaction uses Apple's
`originalApplicationVersion` claim for the original-version column. Empty-response mutations print registered camelCase
receipts. Diagnostics go to stderr; invalid input
returns usage exit 2, API/auth failures exit 1. Mutations respect read-only mode
and are sent once; the client does not automatically retry them.

## Request validation and lifecycle

- Token updates require a UUID and replace an existing association. They do not
  rewrite past subscription renewals or support FAMILY_SHARED transactions.
- Consumption uses the current v2 body: required `customerConsented`,
  `deliveryStatus`, `sampleContentProvided`; optional `consumptionPercentage`
  (integer milliunits, 0–100000) and `refundPreference`. Require explicit true
  consent and explicit sample boolean. For non-delivery, a supplied percentage
  must be zero; omit it for auto-renewable subscriptions. Apple validates
  product type, which is not supplied to this endpoint. Do not infer consent or fetch a
  transaction before sending. Send only after CONSUMPTION_REQUEST, within 12h.
- Finish only after content is delivered; no server finish is needed if the app
  already calls StoreKit's `finish()`.

## Verification

Establish RED with CLI request-wiring and input-validation tests, then implement
and reach GREEN. Use httptest for actual method/path/body/response contracts,
including empty-response receipts, read-only
refusal, and no output on failure. Reuse existing authentication, HTTP error,
timeout, and cancellation coverage. Verify table/Markdown and built-binary
exit/stream behavior. Run generated command docs, full repository gates, a
separate read-only subagent review, and the mandatory local Codex review.

No stable flags are removed and no new dependency is needed. Individual flags
keep requests reviewable and avoid introducing raw JSON file parsing. Live
mutations are outside this implementation's verification scope.

## References

- [Apple Server API library](https://github.com/apple/app-store-server-library-node/blob/main/index.ts)
- [App transaction info](https://developer.apple.com/documentation/appstoreserverapi/get-app-transaction-info)
- [Account token update](https://developer.apple.com/documentation/appstoreserverapi/set-app-account-token)
- [Consumption request](https://developer.apple.com/documentation/appstoreserverapi/consumptionrequest)
- [Finish transaction](https://developer.apple.com/documentation/appstoreserverapi/finish-transaction)

- [App transaction claims](https://github.com/apple/app-store-server-library-node/blob/b71d9c70aa4a4a5efea258d530f50733a7916af3/models/AppTransaction.ts)

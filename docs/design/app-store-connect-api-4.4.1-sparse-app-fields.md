# App Store Connect API 4.4.1 sparse app fields

## Placement and command shape

This slice covers eight existing GET operations whose sparse-field enums gained
new values in App Store Connect API 4.4.1. It extends the existing typed clients
and the closest existing commands; it does not add a generic query escape hatch.

- `asc apps list` and `asc apps view` accept
  `--app-info-fields kidsAgeBand`, `--iap-fields versions`, and
  `--subscription-group-fields versions`, automatically including the matching
  relationship.
- `asc apps info list` and the app-info form of `asc apps info view` accept
  `--fields kidsAgeBand`; both accept
  `--age-rating-fields socialMedia,socialMediaAgeRestricted`, automatically
  including `ageRatingDeclaration`.
- `asc age-rating view` accepts
  `--fields socialMedia,socialMediaAgeRestricted`.
- `asc localizations list --type app-info` accepts
  `--app-info-fields kidsAgeBand`, automatically including `appInfo`.
- `asc xcode-cloud products app` accepts `--iap-fields versions` and
  `--subscription-group-fields versions`, automatically including the matching
  relationship.
- The app-info-localization detail operation has typed client options only;
  the collection operation is exposed by `asc localizations list --type app-info`.

Apple marks `kidsAgeBand` deprecated even though API 4.4.1 adds it to these
sparse-field enums. The CLI supports the published selector for compatibility,
labels it deprecated, and guides new workflows to `asc age-rating view` and its
current age-rating fields.

All flags use the exact OpenAPI field names and comma-separated long-form CLI
syntax. JSON remains the default non-TTY output and errors remain on stderr.

## Endpoint contract

| Method and path | New 4.4.1 query value |
| --- | --- |
| `GET /v1/appInfoLocalizations/{id}` | `fields[appInfos]=kidsAgeBand` |
| `GET /v1/appInfos/{id}` | `fields[appInfos]=kidsAgeBand`; `fields[ageRatingDeclarations]=socialMedia,socialMediaAgeRestricted` |
| `GET /v1/apps` | `fields[appInfos]=kidsAgeBand`; `fields[inAppPurchases]=versions`; `fields[subscriptionGroups]=versions` |
| `GET /v1/apps/{id}` | `fields[appInfos]=kidsAgeBand`; `fields[inAppPurchases]=versions`; `fields[subscriptionGroups]=versions` |
| `GET /v1/appInfos/{id}/ageRatingDeclaration` | `fields[ageRatingDeclarations]=socialMedia,socialMediaAgeRestricted` |
| `GET /v1/appInfos/{id}/appInfoLocalizations` | `fields[appInfos]=kidsAgeBand` |
| `GET /v1/apps/{id}/appInfos` | `fields[appInfos]=kidsAgeBand`; `fields[ageRatingDeclarations]=socialMedia,socialMediaAgeRestricted` |
| `GET /v1/ciProducts/{id}/app` | `fields[appInfos]=kidsAgeBand`; `fields[inAppPurchases]=versions`; `fields[subscriptionGroups]=versions` |

Relationship sparse fields require the corresponding `include` value:
`appInfo`, `ageRatingDeclaration`, `inAppPurchases`, or `subscriptionGroups`.
Commands add it deterministically and preserve explicit compatible includes.

## Validation and compatibility

The new flags are additive. They validate against the exact new enum members
before authentication or HTTP, and explicitly provided empty or whitespace-only
values are rejected. `--app-info-fields` is rejected on non-app-info
localization types rather than silently ignored. On paginated app lists,
app-info localization reads, and the app-info command's paginated localization
mode, `--next` cannot be combined with a sparse-field flag, even when that
flag's explicit value is empty, because the continuation URL owns the full
query. Existing invocations and output shapes are unchanged.

The typed client keeps operation-specific query structs and functional options;
there is intentionally no universal query map. Invalid or unsupported values
cannot be smuggled through the public CLI.

## Verification

RED coverage establishes missing query propagation, strict CLI enums, automatic
includes, and `--next` conflicts. GREEN coverage asserts exact paths and query
keys with `httptest`, CLI behavior before auth, and representative command HTTP
requests. The slice is then checked with focused packages, generated command
docs, the repository gates, and a read-only live smoke test.

Alternatives rejected:

- A raw `--query` map would expose unsupported values and violate typed command
  validation.
- Separate commands for the new fields would duplicate existing resource
  taxonomy without adding capability.

## Live evidence (July 17, 2026)

Read-only checks used disposable app `6759231657`:

- `fields[inAppPurchases]=versions` and
  `fields[subscriptionGroups]=versions` with their automatic includes returned
  `200` from `GET /v1/apps/{id}`.
- `fields[ageRatingDeclarations]=socialMedia,socialMediaAgeRestricted` returned
  `200` both from the direct age-rating operation and when included from app
  info detail and collection operations.
- Apple production returned `400` (`'kidsAgeBand' is not a valid field name`)
  for the exact official 4.4.1 `fields[appInfos]=kidsAgeBand` query on app info
  detail and collection operations. The implementation intentionally retains
  the official OpenAPI contract; this is recorded as an upstream rollout lag,
  not hidden by client-side fallback.
- The disposable app has no Xcode Cloud product, so the product-to-app variant
  remains covered by exact HTTP and CLI transport tests rather than a live call.

No App Store Connect resource was created, changed, or deleted by these checks.

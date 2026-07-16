# IAP version pagination contract

## Placement and command shape

The App Store Connect 4.4.1 IAP version commands remain under the existing
`asc iap versions` taxonomy. This change hardens the three collection lists and
the three plural raw-linkage commands, plus the parent IAP list extended by the
4.4.1 version fields:

- `asc iap list`
- `asc iap versions list`
- `asc iap versions images list`
- `asc iap versions localizations list`
- `asc iap versions links versions`
- `asc iap versions links images`
- `asc iap versions links localizations`

The collection lists already expose `--paginate`. The plural linkage commands
gain the same flag, with the invocation shape
`asc iap versions links <relationship> --<owner>-id "ID" --paginate`.

## API contract

The commands issue `GET` requests to these OpenAPI operations:

- `/v2/inAppPurchases/{id}/versions`
- `/v1/apps/{id}/inAppPurchasesV2`
- `/v1/inAppPurchaseVersions/{id}/images`
- `/v1/inAppPurchaseVersions/{id}/localizations`
- `/v2/inAppPurchases/{id}/relationships/versions`
- `/v1/inAppPurchaseVersions/{id}/relationships/images`
- `/v1/inAppPurchaseVersions/{id}/relationships/localizations`

The direct collection endpoints return their typed collection responses. The
relationship endpoints return `LinkagesResponse`, whose `data` array contains
resource identifiers. Every response may contain an opaque `links.next` URL.
When `--next` is present, that URL replaces the complete owner-scoped request
path and query string. Therefore owner flags and explicitly supplied `--limit`
(including `--limit 0`) cannot be combined with `--next`; accepting them would
silently ignore user input.

`--paginate` fetches the first owner-scoped page and follows each opaque
`links.next` URL, aggregating every page before applying the normal output
renderer. Explicit `--output` and `--pretty` behavior remains unchanged. Data
is written to stdout; validation diagnostics are written to stderr and return
the CLI usage exit code before authentication or HTTP work begins.

## Compatibility and failure modes

This is additive for valid linkage invocations and makes previously ambiguous
invalid combinations fail explicitly. Existing one-page linkage output remains
unchanged when `--paginate` is absent. A malformed next URL, an explicitly
supplied owner plus `--next`, or an explicit `--limit` plus `--next` is a usage
error. Pagination API errors retain page context from `asc.PaginateAll`.

## Verification

RED coverage uses a poison client factory to prove the parent list and all six
version collection/linkage owner-plus-next combinations fail before
authentication. It also covers parent and linkage limit-plus-next combinations,
including explicitly empty owners and `--limit 0`, so validation follows flag
presence rather than values. HTTP-backed CLI tests exercise two-page
aggregation for each plural linkage command, including the exact first owner
path and opaque second URL. Command documentation is regenerated because
linkage help gains `--paginate`. Focused command tests precede the full format,
documentation, lint, test, and build gates.

An alternative was to accept owner flags with `--next` and try to compare the
owner against the URL. Opaque URLs are not a stable source of owner identity,
so that would add parsing without making the request less ambiguous. Another
alternative was to leave linkage pagination to repeated manual `--next` calls;
that would keep these commands inconsistent with the neighboring collection
and subscription relationship commands.

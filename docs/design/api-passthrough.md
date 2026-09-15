# `asc api`: authenticated raw request passthrough

Agent wrappers built on the App Store Connect API rely on a raw request
executor. `asc` already exposes `auth token` and `schema`, but no request
runner, so agents shell out to `curl` for the operations the CLI does not wrap
yet, including the remaining `relationships/` linkage reads. `asc api` closes
that gap with one command that reuses the CLI's authentication, retries, rate
limiting, timeouts, debug logging, and error rendering.

## Placement

`api` is a top-level leaf command registered in
`internal/cli/registry/registry.go` and listed under `UTILITY COMMANDS` in root
help, next to `schema`. The implementation lives in `internal/cli/api`.

## Invocation

```text
asc api <METHOD> <PATH> [flags]
```

- `METHOD` is `GET`, `POST`, `PATCH`, or `DELETE` (case-insensitive). Apple's
  OpenAPI definition declares no other methods.
- `PATH` is a versioned API path such as `/v1/apps/{id}/builds` or a full
  `https://api.appstoreconnect.apple.com/...` URL. Any other host is a usage
  error. A query string in `PATH` is merged with `--query`.
- `--query key=value` is repeatable and adds one query parameter per use. Keys
  can repeat. Query parameters are only accepted for `GET`; the OpenAPI
  definition declares none for mutating methods and the client rejects them.
- `--body JSON` sends an inline JSON object. `--body @path` reads the object
  from a file (`@-` reads standard input). `--body-file PATH` is the explicit
  file form and is mutually exclusive with `--body`. A body is accepted for
  `POST`, `PATCH`, and `DELETE` (relationship removals carry a body) and is a
  usage error for `GET`.
- `--paginate` follows `links.next` for `GET` collection responses and prints
  one aggregated envelope.
- `--confirm` is required for every non-`GET` request. There are no prompts.
- `--allow-unknown-path` skips the embedded schema index check.
- `--output json` (the only format) and `--pretty` control rendering, bound
  through `shared.BindOutputFlagsWithAllowed` exactly as `asc ads`,
  `asc workflow`, and `asc web xcode-cloud workflow-options` bind it. The
  default is always `json` rather than the TTY-aware table default: the command
  accepts any operation in the index, so there is no fixed column set to render,
  and the issue contract requires `GET` to print the envelope as-is. An
  explicit `--output table` is rejected with exit 2 rather than silently
  ignored.

Flags may appear before or after the positional arguments, matching
`asc schema`. `asc api` is registered as a positional-payload command so a
trailing `--paginate GET` is never rewritten into `--paginate=GET`.

## Validation order

1. Positional count, method, and path shape (usage errors, exit 2).
2. Flag combinations: `--body` with `--body-file`, body on `GET`, `--paginate`
   or `--query` on a non-`GET`, missing `--confirm` on a non-`GET`.
3. Control characters in `PATH`. The client rejects these too, but only after
   credentials load, so the command rejects them first as a usage error.
4. Non-JSON operations. The shared client negotiates
   `Accept: application/json` and this command prints a JSON envelope, so an
   operation whose 2xx responses declare no plain `application/json`
   representation is a usage error naming the command that handles it. Seven
   operations qualify in `docs/openapi/latest.json`: the `application/a-gzip`
   reports (`/v1/salesReports` -> `asc analytics sales`, `/v1/financeReports`
   -> `asc finance reports`), the `text/csv` one-time-use code values
   (`/v1/subscriptionOfferCodeOneTimeUseCodes/{id}/values` ->
   `asc subscriptions offers offer-codes values`,
   `/v1/inAppPurchaseOfferCodeOneTimeUseCodes/{id}/values` ->
   `asc iap offer-codes one-time-codes values`), and the vendor JSON metrics
   and logs (`/v1/apps/{id}/perfPowerMetrics` and
   `/v1/builds/{id}/perfPowerMetrics` -> `asc performance metrics`,
   `/v1/diagnosticSignatures/{id}/logs` ->
   `asc performance diagnostics view`), which need an explicit vendor `Accept`
   header the shared retrying request path does not carry. The check keys off
   the matched schema template, so it applies under `--allow-unknown-path` too:
   the limitation is the response representation, not a gap in the index.
   `TestNonJSONOperationsMatchOpenAPISnapshot` re-derives this set from the
   snapshot, so a snapshot refresh that adds or removes such an operation fails
   the build instead of silently degrading output. Every other `text/csv`
   content type in the snapshot sits on an endpoint that also serves
   `application/json`, so those pass through unchanged.
5. Schema index lookup. The request path is matched segment-by-segment against
   the embedded `asc schema` index, where `{id}` segments match any single
   segment. An unknown method/path pair is a usage error whose single-line
   message lists up to five nearest known operations, unless
   `--allow-unknown-path` is set. Candidates rank by: same path under another
   method first, then longest shared leading segment run, then the requested
   method, then the smallest edit distance between the first differing
   segments, then the smallest length difference.
6. Body parsing (`--body`, `--body-file`).
7. Client creation. Nothing above touches credentials or the network.

## Output

- `GET` responses print Apple's JSON envelope unmodified. `--pretty` re-indents
  the same document. A non-empty body that is not valid JSON is an error on
  both paths, since `json` is the only output format.
- A malformed `links.next` (a non-string value) fails the paginated read.
  An absent key and an explicit `null` are both the terminal case.
- Mutation responses print Apple's response body unmodified. An empty body
  (HTTP 204, for example from `DELETE`) prints nothing and exits 0.
- `--paginate` requires a `data` array. A to-one linkage or a `null` data
  member fails with `ErrRawPaginationNotCollection` instead of being coerced
  into an empty collection. Pages are merged like
  `asc.PaginateAll`: `data` is concatenated, `included` is concatenated with
  duplicate `type`/`id` pairs removed, `links` and `meta` come from the first
  page with `links.next` removed. Every `links.next` URL must stay on
  `api.appstoreconnect.apple.com` over HTTPS before it is followed, and a
  repeated URL stops the loop with an error.
- Apple error responses render through the existing `asc.APIError` path, so
  exit codes and messages match every other command.
- Diagnostics go to stderr; data goes to stdout.

## Telemetry

The telemetry command path is derived from the command tree before the first
positional argument, so it is `asc api` regardless of method and path. Failure
parameters are limited to the shared allowlist: the missing-`--confirm` error
carries `--confirm`, and the unknown-operation error carries an `invalid_input`
diagnostic with no parameter so the `--allow-unknown-path` hint in its message
is never scanned as the failing flag. The method, path, query, and body never
reach telemetry.

## Compatibility

New command, no lifecycle impact. The client gains an exported
`RawRequest` and `RawPaginatedGET` helper pair for the passthrough; existing
typed client methods are unchanged. `internal/cli/schema` exports
`MatchOperation` and `NearestOperations` on top of the existing embedded index.

## Verification

- `internal/cli/cmdtest`: help and usage errors (missing arguments, bad
  method, unknown path with suggestions, missing `--confirm`, `--paginate` on
  `POST`, body on `GET`, conflicting body flags, non-object inline body,
  control character in `PATH`, non-JSON operations including one reached with
  `--allow-unknown-path`, foreign host), `GET`
  passthrough, paginated `GET`, `POST` with `--confirm` and inline or file
  bodies, `DELETE` with an empty response, and `--allow-unknown-path`.
- `cmd`: telemetry command path stays `asc api` and spaced-boolean
  normalization leaves positional arguments untouched.
- `internal/cli/api`: the non-JSON exclusion table re-derived from
  `docs/openapi/latest.json`, plus a cmdtest that resolves every recommended
  replacement command through the real root command.
- `internal/cli/schema`: path matching and nearest-operation ranking.
- `internal/asc`: pagination merge and next-URL host validation.
- Live: one read-only `asc api GET /v1/apps --query limit=1` when credentials
  are available on the host.

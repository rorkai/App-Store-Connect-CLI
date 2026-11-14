# OpenAPI snapshot (offline)

This folder keeps an offline snapshot of the App Store Connect OpenAPI spec for
agents that cannot access the internet.

## Files

- `latest.json`: full OpenAPI spec snapshot (see source below)
- `paths.txt`: generated path+method index for quick existence checks

## Source

Preferred sources for the OpenAPI spec:

- Official Apple download (zip): `https://developer.apple.com/sample-code/app-store-connect/app-store-connect-openapi-specification.zip`
- Community mirror that tracks Apple's published spec: `https://github.com/EvanBacon/App-Store-Connect-OpenAPI-Spec`

Note: The published OpenAPI spec can lag reality and may omit some operations that
still work in the API (parity checks can surface these gaps).

## Update process

1. Replace `latest.json` with a newer spec file.
2. Run `make update-openapi` to regenerate `paths.txt`.
3. Run `make update-schema-index` to regenerate the runtime `asc schema` index.
4. Run `make check-docs` to verify both generated indexes are current.
5. Update the "Last synced" date below and commit the snapshot and indexes together.

Last synced: 2026-11-05

Snapshot version: `4.6`, from the official Apple archive listed above.

- Archive SHA-256: `f41ed4489c78bf1488cd2291f23394565364fe37b402130f48c3e3c3f5ea1acd`
- Extracted `openapi.oas.json` SHA-256: `74732362a91236dcb66eb1620f8be18c2b6060771269b483eab578473bb32763`

The snapshot preserves the extracted JSON bytes. Both indexes are generated
from that snapshot using the update process above.

## Web interface schema (`web/`)

`web/latest.json` describes the requests the `asc web` commands send to the
App Store Connect and Apple Developer web interfaces. No published schema
exists for these services, so this file is ours. It follows the conventions of
`latest.json` (OpenAPI 3.0.1, one tag per resource, `<resource>_<action>`
operation IDs, `XResponse`/`XsResponse`/`XCreateRequest`/`XUpdateRequest`
schema names, JSON:API resources as `type`/`id`/`attributes`/`relationships`)
so it can later drive generated commands. `web/paths.txt` is its method and
path index, generated the same way as `paths.txt`.

The first version was derived on 2026-11-14 from the client code in
`internal/web`. It has not been checked against live traffic yet. It describes what our code sends and reads, not everything the
services accept or return:

- Response objects set `additionalProperties: true` and list only the fields
  the client decodes. A property is `required` only where the client fails
  without it (`data` on lists the client pages through).
- Enums appear only where the code sends a fixed value or holds an explicit
  list; other strings are plain `string`. A query parameter is `required`
  when every request to the operation carries it.
- Success responses are `2XX` because the client treats any status below 400
  as success. Errors are a `default` response; only the JSON:API hosts carry
  the `ErrorResponse` body the client reads.

### Hosts

| Server | Path prefixes |
|---|---|
| `https://appstoreconnect.apple.com` (top-level server) | `/iris/v1`, `/iris/v2` (JSON:API resources), `/olympus/v1` (session, actors, contract messages), `/analytics/api/v1`, `/analytics/api/v2`, `/ci/api`, `/ci/auth` (Xcode Cloud), `/ppm/complianceform/v1`, `/sandbox/v2`, `/WebObjects/iTunesConnect.woa/ra` (app bundles, business agreements, transaction tax reports), `/logout` |
| `https://developer.apple.com` (path-level `servers`) | `/services-account/QH65B2` (form endpoints), `/services-account/v1` |
| `https://idmsa.apple.com` (path-level `servers`) | `/appleauth/auth` (sign-in and two-factor) |

Paths keep their full prefix, as `latest.json` keeps `/v1`, so every path key
is unique across hosts.

### Extensions

- `x-asc-source`: the exported `internal/web` functions and methods that send
  the operation, sorted.
- `x-asc-observed`: where the operation's shape comes from and when, for
  example `{"code": "2026-11-14"}`. A browser pass adds `"live"` with its date.
- `x-asc-hand-written`: sign-in, two-factor and session operations (tag
  `Session`). They are not resource-shaped and stay hand-written in the
  client; generated code must not replace them.
- `x-asc-not-implemented`: an operation no client method sends yet. The drift
  test accepts it only with this mark.
- `x-asc-server-provided-urls` (top level): methods that also request a URL
  taken from a response (signed downloads). Such URLs have no path in the
  schema.

Operation IDs use the action words of `latest.json`. Words added for this
file: `replaceInstance` (PUT), `query` (a POST that only reads analytics
data), `overrideInstance` and `overrideCollection` (a Developer Portal POST
whose `X-HTTP-Method-Override` header names the real action), and
`headInstance` (HEAD).

### Left out

- Requests to server-provided URLs, listed under `x-asc-server-provided-urls`.
- Session continuation headers (`scnt`, `X-Apple-ID-Session-Id`, the widget
  key) and the Developer Portal `csrf` and `csrf_ts` headers. The security
  scheme names the session cookie; the client replays every cookie the
  services set.
- Query values and example values. The file holds field names, types and
  fixed values only, never values from an account.

### Updating the web schema

1. Change `internal/web`.
2. Run `ASC_BYPASS_KEYCHAIN=1 go test ./internal/web/ -run TestWebAPISchema`.
   `TestWebAPISchemaCoversClientRequests` names every request the client can
   send that has no operation, every operation no client method sends, and
   the `x-asc-source` each operation should carry.
   `TestWebAPISchemaIsValid` checks that the file parses, every `$ref`
   resolves, operation IDs are unique and every path variable has a path
   parameter.
3. Edit `web/latest.json` by hand, and set `x-asc-observed` for what you
   changed.
4. Run `make update-openapi`. It sorts paths and schemas, writes the file in
   canonical form, and regenerates `web/paths.txt`.
5. Run `make check-docs`.

CI runs the full test suite for any change to `web/latest.json`, so the drift
test also guards schema-only changes.

The drift test reads the `internal/web` source with `go/ast`. From each
exported function and method it follows calls into unexported functions of
the package and evaluates the method and URL handed to `net/http`
(`doRequestBaseWithHTTPClient`, `doDeveloperPortalHTTP`,
`appleauth.DoTwoFactorJSONRequest`, and every other `http.NewRequest`). It
cannot see:

- requests built in other packages, except through the listed functions;
- calls through function values or interfaces; the test fails if a `Client`
  method is used as a value or shares its name with another type's method,
  so this gap cannot open silently;
- which constructor's base URL a method uses when its URL starts at
  `Client.baseURL`: any of them matches;
- which method goes with which URL when both are chosen at run time: every
  method and every URL must appear in at least one matching operation;
- query strings, headers and bodies, which it does not compare.

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
4. Run `make generate-client` to regenerate the client code in `internal/asc`.
5. Run `make check-docs` to verify the generated indexes and client code are current.
6. Update the "Last synced" date below and commit the snapshot and indexes together.

Last synced: 2026-11-05

Snapshot version: `4.6`, from the official Apple archive listed above.

- Archive SHA-256: `f41ed4489c78bf1488cd2291f23394565364fe37b402130f48c3e3c3f5ea1acd`
- Extracted `openapi.oas.json` SHA-256: `74732362a91236dcb66eb1620f8be18c2b6060771269b483eab578473bb32763`

The snapshot preserves the extracted JSON bytes. Both indexes are generated
from that snapshot using the update process above.

## Generated client code

`internal/openapigen` writes the `*_gen.go` files in `internal/asc` from
`latest.json` and the bindings in `internal/asc/openapi_bindings/`, one file
per spec tag named `<Tag>.json`. A tag's bindings list the types and the
operations to generate, with the Go choices the spec cannot supply: method
name, parameter names, blank-argument checks, the paged-read option type and
query builder, and error prefixes. Do not edit generated files or the bindings
by hand.

- `make generate-client` regenerates every generated file.
- `make check-openapi` and `go test ./internal/openapigen` fail when a
  generated file is out of date.
- `go test ./internal/asc -run TestGeneratedClient` checks the generated code
  against `latest.json`: every bound operation and type must still exist, and
  every generated method must send the operation's HTTP method and path, only
  query parameters the operation defines, and a body whose type, attribute and
  relationship names the request schema defines, with the required ones
  present. A snapshot update that drops or changes a bound operation fails
  here, naming the operation and the method.
- A known difference that must stay for now is recorded by hand on its
  operation in the bindings file, as `"deviations": [{"problem": "<the test's
  message>", "reason": "<why it stays>"}]`. The test then requires exactly
  that message: it fails when the method stops producing it, produces another
  one, or the reason is empty. The generator keeps these records when it
  rewrites a bindings file and generates nothing from them.
- `cd internal/asc && go run ../openapigen -report` lists the hand-written
  client methods that send a request no operation in `latest.json` describes,
  and the hand-written attribute structs that lack an attribute of their
  resource and neither keep unmodeled attributes (`apiAttributes`) nor encode
  themselves. It changes nothing and never fails. It follows request paths
  written as string literals (including `+` concatenation), through helpers
  that take a path segment or the HTTP method from their callers and through
  package functions that return a path; a path assembled without any `/v…`
  literal is not seen.
- To move hand-written code of more tags to the generator, run
  `cd internal/asc && go run ../openapigen -adopt <Tag>[,<Tag>...]`. For each
  operation of a tag it renders the method from the spec and the choices read
  out of the existing method, and replaces the existing method only when both
  are the same Go tokens; types are handled the same way. It writes only the
  bindings files of the tags it adopted code from. It prints what it
  generated, why it kept the rest, and the name the rule in
  `internal/openapigen/names.go` gives each operation that has no method yet.
  Add `-dry-run` to only print the report.

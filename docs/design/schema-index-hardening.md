# Runtime schema index hardening

The App Store Connect 4.4.1 specification adds create requests whose payloads
contain required JSON:API relationships but no attributes. The compact runtime
index currently records only `data.attributes`, so `asc schema` identifies the
request schema without showing the relationship fields needed to construct a
valid payload.

## Chosen approach

Keep the existing `asc schema [flags] [query]` command and JSON output stable,
and add an optional `requestRelationships` object to each matching endpoint.
Each relationship records its target resource type, `one` or `many`
cardinality, and whether the relationship is required. This is generated from
the exact operation request schema in `docs/openapi/latest.json`, alongside the
existing request attributes.

Add deterministic checks for both generated OpenAPI artifacts to `make
check-docs`: `docs/openapi/paths.txt` must match the snapshot, and
`internal/cli/schema/schema_index.json` must match the compact generator. The
checks are read-only and print the regeneration command on failure.

## Compatibility and failure behavior

The output change is additive: endpoints without request relationships are
unchanged, and existing fields and flags retain their behavior. No network or
authentication is involved. A stale generated artifact makes `make check-docs`
exit nonzero before CI can accept the change.

## Alternatives

Embedding every request schema would be complete but would substantially grow
the binary and make runtime output harder to consume. Recording only
relationship names would stay smaller, but it would omit the target resource
type and cardinality needed to form payloads. The compact relationship summary
preserves the current index's purpose while covering relationship-only create
requests.

## Verification

Unit tests cover required to-one and optional to-many relationships, the three
new 4.4.1 version-create endpoints, and stale path/index detection. A built
binary must show the new relationship fields on stdout as valid JSON, with no
stderr output and exit code 0. The repository format, documentation, lint, and
full test gates remain required.

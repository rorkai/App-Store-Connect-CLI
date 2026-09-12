# Multi-target signing sync push

## Scope

`asc signing sync push` keeps its current single-target invocation and gains an
optional `--targets-file` selector. The selector is a non-secret JSON manifest
whose only purpose is to make a bounded, deterministic list of bundle IDs. It
does not introduce another signing store, another App Store Connect workflow,
or per-target profile settings.

```json
{
  "schemaVersion": 1,
  "targets": [
    {"bundleId": "com.example.app"},
    {"bundleId": "com.example.widget"}
  ]
}
```

The manifest is read before password, identity, authentication, App Store
Connect, repository, or Git side effects. The selected path is opened through
the current working-directory root with absolute paths and `..` escapes
rejected, must use no-follow traversal, must be a regular file, and is capped
at 64 KiB. It is intentionally not subject to the owner-only permission rule
used for secret inputs; a readable `0644` manifest is valid. JSON decoding is
strict: schema version 1, exactly the `schemaVersion` and `targets` fields at
the top level, exactly `bundleId` in each target, one to 32 targets, non-empty
bundle IDs, no control/bidi/path characters, and no case-insensitive duplicate
IDs. IDs are trimmed and sorted case-insensitively with a deterministic
tie-breaker before any App Store Connect request.

`--targets-file` and `--bundle-id` are mutually exclusive. The existing
`--profile-type`, `--certificate-type`, `--device`, `--create-missing`, and
identity flags apply uniformly to every target. There is no per-target profile
type or device override in this slice.

## Execution and repository contract

The command resolves all selected bundle IDs and signing assets in canonical
order using the existing list/read/create API operations. Missing profiles are
created sequentially only when `--create-missing` is set. A single temporary
clone is prepared lazily and is reused for all targets. No encrypted file is
written until all targets have resolved, identity checks have passed, and all
planned repository paths have passed rooted/no-follow collision checks. A
failure before commit therefore cannot publish a partial batch.

Certificates are deduplicated by resource ID and written in canonical ID order.
In batch mode profiles use a target-scoped path:

```text
profiles/<profile-type-directory>/<safe-bundle-id>--<safe-profile-resource-id><profile-extension>.enc
```

`<profile-extension>` is `.provisionprofile` for native macOS profile types and
`.mobileprovision` for iOS and tvOS profile types. Existing encrypted files
with the legacy suffix remain readable and are not renamed automatically.

The existing single-target iOS and tvOS profile paths remain unchanged; native
macOS single-target paths use the same platform-specific suffix rule. Shared
identities remain one authenticated core per certificate and one current authenticated
context per team/bundle/profile-type; each target context binds its exact
profile path, resource ID, UUID, and content digest.

Batch writes preserve semantic no-op behavior. Existing certificate/profile
legacy envelopes are decrypted and compared before replacement; equivalent
plaintext is retained, while an unreadable or differently authenticated
artifact fails closed. Versioned identity artifacts use their existing
authenticated comparison and replacement rules. One `git add`, at most one
commit, and at most one push cover the whole batch. The deterministic commit
message is `Update signing assets for <profile-type> (<target-count> targets)`.
An unchanged batch performs no commit or push.

## Output and compatibility

Single-target JSON remains field-compatible. Batch JSON adds `bundleIds` and a
`targets` array; each target reports `bundleId`, `profileType`, `profilePath`,
`profileCreated`, and its `files`. The top-level `files` field is the sorted
union of all target files, and existing identity fields remain truthful. Table
output has one row per target. Asset data is stdout-only; progress, profile
creation, and diagnostics are stderr-only. Invalid manifest/flag input exits
with usage status 2, while API, encryption, repository, and Git failures exit
with operational status 1. No password, private key, repository credential, or
raw secret is included in output or errors.

## Non-goals

This slice does not add manifest-driven pull filtering, a storage backend,
keychain installation, repair or migration, delete/revoke behavior, parallel
App Store Connect mutations, per-target settings, or changes to
`signing reconcile`.

## Verification

Tests cover strict manifest parsing, size and path safety, symlinks,
non-owner-only permissions, ordering and duplicate rejection, pre-auth
validation, one-clone/one-commit behavior, target-scoped paths, certificate
deduplication, identity-context binding, no-op reruns, fail-closed existing
artifacts, and failure-before-commit behavior. The normal build, formatting,
documentation, lint, and test gates remain required.

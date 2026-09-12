# Target-selective signing sync pull

## Scope and command contract

`asc signing sync pull` keeps its current full-repository behavior when no
selector is supplied. This change adds the same bounded bundle selector used by
multi-target push, plus the profile type needed to identify one signing
context:

```text
asc signing sync pull \
  --repo git@github.com:team/certs.git \
  --bundle-id com.example.app \
  --profile-type IOS_APP_STORE \
  --password-file ~/.config/asc/signing-sync-password \
  --output-dir .asc/signing/pulled
```

`--bundle-id` and `--targets-file` are mutually exclusive. Either selector
requires `--profile-type`; `--profile-type` without a selector is rejected so
it cannot be silently ignored. The profile type must be one exact value from
the App Store Connect profile-type enum; suffixes and partial matches are
rejected before passwords or repository access. The existing schema-versioned targets file,
including its size, rooted-path, strict-JSON, count, character, duplicate, and
ordering checks, is reused unchanged. Selector validation happens before the
password, repository clone, or output-directory side effects.

The selected pull is additive. Existing invocations, JSON fields, full-pull
file selection, decryption limits, and Git behavior do not change.

Both native macOS `.provisionprofile` files and legacy `.mobileprovision`
profile files are recognized during repository validation and selection. The
stored relative path remains exact, so pulling a legacy file does not rename or
overwrite it merely because the preferred macOS suffix is different.

Profile pushes now replace the legacy encrypted profile payload with a
versioned envelope containing non-secret, authenticated bundle, profile-type,
resource-ID, and optional UUID metadata. Certificates retain their existing
format. The profile upgrade is published in the same existing single Git
commit and push; an unchanged versioned profile is a semantic no-op. Existing
legacy profiles remain valid for full pulls and for selective contexts whose
signed claims are unambiguous.

## Selection and validation

The command still authenticates and validates every encrypted artifact before
selecting output files. This preserves repository-wide path collision checks,
cumulative size limits, authenticated metadata checks, identity-context graph
validation, and stale identity-core handling. A selector is not allowed to
hide a corrupt or conflicting repository.

After repository validation, the command parses provisioning profiles and
selects each profile whose exact bundle identifier and distribution class
match a requested bundle/profile-type pair. Profile matching is based on the
signed profile payload, not filenames. Development, ad hoc, store, in-house,
and direct-distribution profile classes are distinguished from the signed
payload. For every selected profile it includes:

- the provisioning profile itself;
- every stored public certificate whose DER fingerprint appears in the
  profile's developer-certificate list, without requiring inactive or
  otherwise unsynced embedded certificates to be present;
- the authenticated identity context for the exact team, bundle, and profile
  type, when one exists; and
- the usable PKCS#12 identity core referenced by that context.

Certificate-only stores remain selectable because their profiles and public
certificates do not normally require identity-context metadata. Native Mac and
Mac Catalyst profiles are the exception: their signed platform and
distribution claims overlap, so exact selection requires authenticated
profile-type provenance. Push now stores that exact API profile type with each
profile. An older identity context provides equivalent provenance for the
profile it binds. An older certificate-only Mac profile with neither source is
rejected rather than guessed; pushing the profile again upgrades its encrypted
artifact. A selected identity context must reference one of the selected
profiles and its validated core; the existing graph validator enforces that
binding before selection.

Every requested bundle must resolve to at least one matching profile. The
command fails before writing any output when a requested bundle is absent,
when a profile cannot be classified, or when a selected profile's embedded
certificate list has no matching stored public certificate. An identity-backed
selection additionally requires the stored public certificate for its exact
core identity. Multiple matching
profiles for one requested context are returned rather than choosing by
filename or repository order.

Only selected destinations participate in output collision preflight. A file
for an unselected target already present in the output directory therefore
does not block a selected pull. All selected destinations are preflighted
before the first output write, and private identity files retain create-only
mode-0600 publication.

## Output and compatibility

A one-bundle selected pull reports the existing singular `bundleId` and
`profileType` fields. A targets-file pull uses the existing batch shape:
`bundleIds`, one deterministic `targets` entry per requested bundle, and no
singular `bundleId`. Each target reports `profilePaths`, its first deterministic
path in the compatible singular `profilePath` field, and the files needed for
that target; the top-level `files` field remains the sorted union of files
actually written. Shared certificates and identity cores appear once in the
top-level list.

`identityPresent` and `sensitiveFiles` describe only the selected output. No
password, private key, decrypted payload, device identifier, or repository
credential is added to output or diagnostics. Data remains on stdout and
progress or errors remain on stderr. Invalid flag or manifest input exits 2;
repository, authentication, decryption, validation, and filesystem failures
exit 1.

## Non-goals

This slice does not rotate passwords, add a storage backend, install profiles
or identities, delete stale assets, choose a profile by date, relax
whole-repository validation, or add per-target profile types. Its only push
change is preserving exact profile-type provenance in authenticated profile
metadata.

## Alternatives and trade-offs

Inferring every profile type from signed claims was rejected because native
Mac and Mac Catalyst profiles can share the same platform and distribution
claims. Inferring from repository directories or filenames was also rejected:
those paths group distribution classes and user-controlled profile names do
not authenticate the API type. A separate repository index would add a stale
second source of truth. Keeping provenance in the profile's authenticated
envelope binds the type to the ciphertext and lets an ordinary push migrate a
legacy artifact without a separate command.

The compatibility trade-off is fail-closed selection for an older,
certificate-only Mac profile until it is pushed again. Returning no exact
match is safer than selecting a native Mac profile for a Mac Catalyst request,
or the reverse. Full pulls remain available for inspection and migration.

## RED-GREEN and verification

Coverage begins with failing tests for selector flag relationships, pre-secret
manifest validation, single and multi-target selection, certificate-only and
identity-backed stores, shared certificate/identity deduplication, missing
contexts, corrupt unselected artifacts, unselected output collisions, batch
output shape, exact native Mac versus Mac Catalyst provenance, legacy profile
upgrade, and unchanged full-pull behavior. Focused signing and renderer tests
run before built-binary help and behavior checks. A disposable local encrypted
Git repository exercises push, authenticated metadata readback, selected pull,
and wrong-scope refusal without using live signing assets. Live App Store
Connect mutation is not required for this selection-only workflow; the push
path consumes the same exact profile type already returned and filtered by the
existing API client. Because command help changes, generated command
documentation and the normal build, formatting, documentation, lint, and test
gates are required before each push.

The remaining operational risk is legacy certificate-only Mac data without
exact provenance. The command reports no exact match and leaves the repository
and output directory unchanged. Re-pushing that bundle and exact profile type
adds the authenticated provenance. Multiple active profiles for the same exact
context are still returned intentionally rather than selected by recency.

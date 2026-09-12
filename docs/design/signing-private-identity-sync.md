# Private signing identity sync

> Status: the legacy `--password` flag and `ASC_MATCH_PASSWORD` fallbacks described below were removed in 5.0.0; `--password-file` and `ASC_SIGNING_SYNC_PASSWORD` are the only password sources. See `migrate-to-5-0.mdx`.

## Placement and invocation

This change extends the existing `asc signing sync push` command.
It does not add a second signing store or imply that App Store Connect can
return a private key.

```sh
asc signing sync push \
  --bundle-id com.example.app \
  --profile-type IOS_APP_ADHOC \
  --repo git@github.com:team/signing.git \
  --password-file ~/.config/asc/signing-sync-password \
  --identity ./distribution.p12 \
  --identity-password-file ~/.config/asc/distribution-p12-password

asc signing sync push \
  --bundle-id com.example.app \
  --profile-type IOS_APP_ADHOC \
  --repo git@github.com:team/signing.git \
  --password-file ~/.config/asc/signing-sync-password \
  --private-key ./distribution-key.pem \
  --identity-sha256 <64-hex-ASC-certificate-fingerprint>

asc signing sync push \
  --bundle-id com.example.mac \
  --profile-type MAC_APP_DIRECT \
  --repo git@github.com:team/signing.git \
  --password-file ~/.config/asc/signing-sync-password \
  --identity ./developer-id-application.p12 \
  --identity-password-file ~/.config/asc/developer-id-password
```

`--identity` and `--private-key` are mutually exclusive. A PKCS#12 containing
more than one private identity requires `--identity-sha256` to select the
certificate fingerprint. `--private-key` also requires the fingerprint so the
remote certificate choice is explicit when one key has been reissued.
The original PKCS#12 password is accepted only through
`--identity-password-file`; an unprotected PKCS#12 needs no password file.

The encrypted repository password resolves in this order:

1. `--password-file`, which must name a no-follow regular file private to its owner.
2. The explicit legacy `--password` flag, with a deprecation warning.
3. `ASC_SIGNING_SYNC_PASSWORD`.
4. The legacy `ASC_MATCH_PASSWORD` environment variable, with a deprecation warning.

Profile artifacts use `.provisionprofile.enc` for native macOS profile types
and `.mobileprovision.enc` for iOS and tvOS profile types. Existing legacy
profile paths remain readable; sync does not rename them implicitly.

The existing certificate/profile-only invocation remains valid. Its structured
result reports `identityPresent: false`. Pull results list decrypted identity
artifacts separately in `sensitiveFiles`. Sensitive identities are new-only and
mode `0600`; new ordinary outputs also use `0600`, while existing ordinary
certificate/profile outputs retain their prior mode during atomic replacement.
Private identity sync also accepts `MAC_APP_DIRECT` and
`MAC_CATALYST_APP_DIRECT`. Their signed distribution claims overlap with store
profiles in some fields, but direct profiles carry the all-device claim. The
command requires that claim, the signed macOS platform claim, and the exact
active profile type returned by App Store Connect, resolves associated Developer
ID Application certificates from either supported generation, and verifies that
the local private key, API certificate, embedded profile certificate, team,
bundle ID, and signed profile all agree before publication.
Data is written to stdout, progress and deprecation warnings to stderr, invalid
flag combinations use exit code 2, and operational failures use exit code 1.

The direct-distribution extension is additive: existing iOS, tvOS, Mac App
Store, certificate-only, single-target, and multi-target invocations keep their
behavior and output. No new secret source or output field is introduced. A
missing local identity is therefore an operational input failure for direct
profiles, not an unsupported-flag usage error.

## Identity validation and storage

Local secret inputs are opened without following the final path component and
must be regular files with no group or other permission bits. PEM input accepts
RSA or EC private keys. The public key must match one active, unexpired
certificate associated with the selected profile. When a profile must be
created, only the matching App Store Connect certificate is eligible. The
downloaded profile, its embedded developer certificate, the API certificate,
and the local private key are checked as one identity; profile expiration and
the certificate team organizational unit are also checked.

The normalized artifact contains exactly one private key and its matched
certificate. It is encoded as a password-protected PKCS#12 using the sync
password, then stored once per team/certificate at
`identities/<development-or-distribution>/<CERT_SHA256>.p12.enc`. Separate
authenticated context records bind that identity to each team, bundle, profile
type, certificate, and exact verified provisioning profile without duplicating
private material. Each scope has one authenticated current-context record;
profile renewal atomically advances that record while Git history retains the
prior state. Its encrypted
wrapper has a versioned metadata header authenticated as AES-GCM additional
data. Identity metadata binds the relative path, asset kind, sensitivity,
certificate fingerprint, and team. Context metadata binds the bundle identifier
and profile type. V1 identity envelopes use bounded scrypt parameters recorded
in authenticated metadata; pull bounds each artifact, metadata header, artifact
count, and cumulative bytes before invoking the KDF.
Existing certificate/profile encrypted files keep their stable PBKDF2 format,
and existing repositories remain readable.
No password or identity data is emitted in JSON, diagnostics, or errors.

The sync password also protects the normalized local PKCS#12, whose modern
interchange-compatible encoder uses PBKDF2. Use a generated high-entropy secret
(for example, 32 random bytes) rather than a human-memorable password.

The App Store Connect operations remain the existing profile and certificate
list/create/download calls. There is no API operation for downloading a private
key; all private-key material comes from the explicit local input.

Push writes only into an isolated temporary clone. It commits and pushes after
all encrypted artifacts are published successfully; any local multi-file write
failure returns before Git commit and cleanup removes the clone, so no partial
identity graph reaches the remote repository.

Treating every signed all-device profile as an exact direct-distribution type
was rejected because that claim does not distinguish native Mac from Mac
Catalyst and is also used by other distribution families. Treating a filename
or repository directory as authoritative was also rejected because those values
are local conventions. The all-device and macOS platform claims establish
direct-distribution semantics, while the selected App Store Connect resource's
exact `profileType` is the native Mac versus Mac Catalyst discriminator. The
signed profile and associated Developer ID certificate then provide the
cryptographic binding.

If the API resource type is absent or differs from `--profile-type`, the
command stops before publication. It also stops when the associated certificate
does not match the local private key or embedded profile certificate, or when
the profile team, bundle identifier, state, or expiration is invalid. Profile
creation, when requested, still uses the existing preflight-before-mutation
boundary and only the matching certificate.

## Tests and verification

RED-GREEN coverage includes public flag validation and password-source
compatibility, protected no-follow input reads, RSA and EC keys, single and
multi-identity PKCS#12 parsing, certificate mismatch, profile/certificate/team
and expiration checks, native Mac and Mac Catalyst direct-distribution
identity validation, normalized PKCS#12 round trips, versioned authenticated
envelopes, legacy envelope reads, `sensitiveFiles`, `0600` pull output, and
secret canaries across output and errors. Focused signing packages, generated
command docs, a command-level mock-ASC/local-Git push-pull round trip, and the normal repository gate
complete verification. The direct extension adds synthetic signed-profile
coverage for both native Mac and Mac Catalyst, including missing and non-macOS
platform-claim rejection, plus command-boundary coverage showing that local
identity failures remain operational errors. Live account
or keychain mutation is outside this extension; its remaining risk is provider
data whose associated Developer ID certificate or profile content differs from
the documented API contract, which fails closed before Git publication.

## Alternatives

Storing the source PKCS#12 byte-for-byte would preserve unrelated identities,
unknown passwords, and ambiguous certificate selection. Normalization makes the
semantic payload canonical and unambiguous while PKCS#12 and outer encryption
remain intentionally randomized. Creating a separate
identity command would duplicate repository, encryption, and profile resolution
logic; extending the existing sync command keeps one coherent
store while preserving certificate/profile-only workflows.

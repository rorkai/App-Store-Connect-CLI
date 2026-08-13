# Private signing identity sync

## Placement and invocation

This change extends the existing experimental `asc signing sync push` command.
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

The existing certificate/profile-only invocation remains valid. Its structured
result reports `identityPresent: false`. Pull results list decrypted identity
artifacts separately in `sensitiveFiles`. Sensitive identities are new-only and
mode `0600`; new ordinary outputs also use `0600`, while existing ordinary
certificate/profile outputs retain their prior mode during atomic replacement.
Private identity sync rejects `MAC_APP_DIRECT` and `MAC_CATALYST_APP_DIRECT`
before reading secrets or mutating App Store Connect because their signed-profile
semantics are not supported yet; certificate/profile-only sync remains available.
Data is written to stdout, progress and deprecation warnings to stderr, invalid
flag combinations use exit code 2, and operational failures use exit code 1.

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

## Tests and verification

RED-GREEN coverage includes public flag validation and password-source
compatibility, protected no-follow input reads, RSA and EC keys, single and
multi-identity PKCS#12 parsing, certificate mismatch, profile/certificate/team
and expiration checks, normalized PKCS#12 round trips, versioned authenticated
envelopes, legacy envelope reads, `sensitiveFiles`, `0600` pull output, and
secret canaries across output and errors. Focused signing packages, generated
command docs, a command-level mock-ASC/local-Git push-pull round trip, and the normal repository gate
complete verification. Live keychain or account mutation is outside this PR.

## Alternatives

Storing the source PKCS#12 byte-for-byte would preserve unrelated identities,
unknown passwords, and ambiguous certificate selection. Normalization makes the
semantic payload canonical and unambiguous while PKCS#12 and outer encryption
remain intentionally randomized. Creating a separate
identity command would duplicate repository, encryption, and profile resolution
logic; extending the existing experimental sync command keeps one coherent
store while preserving certificate/profile-only workflows.

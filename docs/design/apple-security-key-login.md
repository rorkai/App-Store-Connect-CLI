# Apple web sign-in with a security key

The initial implementation adds explicit USB FIDO2 authentication to
`asc web auth login`. API-key authentication under `asc auth` is unchanged.

## Local build and use

On macOS, install the native development dependency and build the optional
backend for the host architecture:

```bash
brew install libfido2 pkg-config
CGO_ENABLED=1 go build -tags fido2 -o build/asc-security-key .
./build/asc-security-key web auth login --apple-id "user@example.com" --two-factor-method security-key
```

Connect exactly one USB FIDO2 key registered with the Apple Account. Touch it
when the CLI reports the pending assertion. If the authenticator requires a
PIN, supply `ASC_WEB_SECURITY_KEY_PIN` through the environment. The PIN is not
stored, sent to Apple, printed, or retried automatically. Existing password
input and storage behavior still applies. No new interactive input prompt is
introduced.

`ASC_WEB_2FA_METHOD=security-key` also selects this method for web commands that
need to sign in again. Code commands and phone selection cannot be combined
with security-key mode. A non-terminal login can use this mode without a code
command, but still requires a connected key and physical touch.

Builds without `darwin && cgo && fido2` return an explicit unsupported-backend
usage error before sign-in. This change does not enable the backend in release
artifacts or alter native dependency packaging for either macOS architecture.

## Protocol and ownership

`internal/appleauth` recognizes `fsaChallenge` from Apple's web auth options.
Security-key challenges take precedence over phone or trusted-device metadata.
Explicit key mode refuses a code-only challenge. The default and SMS code
flows retain their existing behavior for accounts without an FSA challenge.

`internal/securitykey` is a narrow Go adapter over libfido2 using pkg-config.
It requests user presence, leaves user verification at the device default,
and passes a PIN only when supplied. Native operations have a maximum
60-second budget, further bounded by the context deadline. Cancellation sends
CTAP cancel; cleanup waits for the cancellation callback before freeing the
device. Multiple connected keys produce an ambiguity error.

`internal/web` follows XcodesLoginKit's private Apple protocol. It hashes the
exact client-data JSON (`webauthn.get`, RP `apple.com`, origin
`https://idmsa.apple.com`, `crossOrigin: true`) and signs only Apple's allowed
credentials. The verification body uses Apple's flat standard-base64 fields,
including raw authenticator data of its actual length. Absent user handles
become empty strings; non-UTF-8 handles are explicitly unsupported.

The assertion is submitted once to `POST
https://idmsa.apple.com/appleauth/auth/verify/security/key`, carrying the
existing cookie jar, widget key, session ID, and `scnt`. Success continues
through the existing trust and ASC session validation requests before normal
provider selection and persistence. A failed finalization remains distinct
from a rejected assertion. These private authentication endpoints are not in
the public App Store Connect OpenAPI snapshot.

## Verification scope

Validation includes builds with and without the native backend and the
repository's formatting, lint, documentation, and full-test checklist.
Regression tests cover FSA routing through decoded auth options, preservation
of the cached continuation, unsupported-backend CLI errors, native method
selection, exact signing inputs and Apple payload encodings, intact raw
authenticator data, PIN isolation, cancellation before submission, one-shot
assertion submission, and finalization failures. The protocol tests replace
the signer and use a local HTTP server; they never contact Apple or access a
physical key. The account owner has verified this flow against their own Apple
Account with registered security keys, using a Ledger Stax over USB on macOS:
successful sign-in, rejection on the device, Ctrl+C while approval is pending,
and successful sign-in after each cancellation. Device rejection returned
`FIDO_ERR_OPERATION_DENIED`; Ctrl+C returned `context canceled`.

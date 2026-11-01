# Apple web sign-in with a security key

The initial implementation adds explicit USB FIDO2 authentication to
`asc web auth login`. API-key authentication under `asc auth` is unchanged.

## Local build and use

Security-key support is an opt-in source build for macOS and Linux. Install
libfido2 **1.14 or newer**, pkg-config, and a C compiler for the host architecture.
Version 1.14 is the minimum for the raw authenticator-data accessors Apple needs.
The native libraries must remain installed when running the binary. Windows is
unavailable. Release binaries and their native dependency packaging are unchanged;
this change does not bundle or enable libfido2 in official releases.

On macOS:

```bash
brew install libfido2 pkg-config
pkg-config --atleast-version=1.14.0 libfido2
CGO_ENABLED=1 go build -tags fido2 -o build/asc-security-key .
```

On Linux (Debian/Ubuntu package names):

```bash
sudo apt-get install libfido2-dev pkg-config build-essential
pkg-config --atleast-version=1.14.0 libfido2
CGO_ENABLED=1 go build -tags fido2 -o build/asc-security-key .
```

Linux also requires distribution or vendor FIDO udev rules that grant the
current user access to the key's `/dev/hidraw*` device. Install/configure those
rules and reconnect the key before signing in; building successfully does not
establish hardware permissions.

```bash
./build/asc-security-key web auth login --apple-id "user@example.com" --two-factor-method security-key
```

Connect exactly one USB FIDO2 key registered with the Apple Account. Physical
touch or approval is required, including for a non-terminal login. If the key
requires a PIN, supply `ASC_WEB_SECURITY_KEY_PIN` through the environment for an
explicit `asc web auth login`. The PIN is not stored, sent to Apple, printed, or
retried within that sign-in attempt. No new interactive input prompt is added.
Existing password input and storage behavior still applies.

`ASC_WEB_2FA_METHOD` accepts `auto`, `sms`, and `security-key`; an explicit
`--two-factor-method` wins. `security-key` also selects key authentication for
commands that need to sign in again. Automatic reauthentication refuses to use
an environment PIN when a key challenge is needed and directs the operator to
run `asc web auth login` explicitly. This prevents successive commands from
reusing a stale PIN and exhausting the key's attempts. Verify the PIN before
another explicit login. Code commands and phone selection cannot be combined
with security-key mode.

Builds without `(darwin || linux) && cgo && fido2` return an explicit
unsupported-backend usage error before an explicit security-key sign-in.

## Protocol and ownership

`internal/appleauth` recognizes `fsaChallenge` from Apple's web auth options.
Automatic routing preserves Apple's SMS-first path when `noTrustedDevices` is
true, even if an FSA challenge is present. Otherwise it selects an FSA challenge
when available. Explicit key mode refuses a code-only challenge; explicit SMS
retains the phone-code path when Apple makes it available.
Explicit key mode also refuses phone-priority options without requesting a code;
use `auto` or `sms` when Apple reports `noTrustedDevices`.

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

CI builds with and without the native backend. Native macOS and Linux jobs
install libfido2 and pkg-config, enforce the minimum library version, compile
with `-tags fido2`, and run focused tagged tests without hardware. These feed
the existing required build gate. The repository's formatting, lint,
documentation, and full-test checklist still applies.
Regression tests cover FSA routing through decoded auth options, preservation
of the cached continuation, unsupported-backend CLI errors, native method
selection, exact signing inputs and Apple payload encodings, intact raw
authenticator data, PIN isolation, cancellation before submission, one-shot
assertion submission, and finalization failures. The protocol tests replace
the signer and use a local HTTP server; they never contact Apple or access a
physical key. The account owner previously verified this flow against their own Apple
Account with registered security keys, using a Ledger Stax over USB on macOS:
successful sign-in, rejection on the device, Ctrl+C while approval is pending,
and successful sign-in after each cancellation. Device rejection returned
`FIDO_ERR_OPERATION_DENIED`; Ctrl+C returned `context canceled`.

Live PIN entry, YubiKey behavior, and Linux hardware access have **not** been
verified. Native builds and cancellation tests
do not establish those hardware results.

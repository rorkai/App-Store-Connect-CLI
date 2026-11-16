# Persistent signing keychain installation

## Status

Proposed for 4.12.0 as a command.

## Problem

`asc signing run` already creates an isolated temporary keychain, installs one
identity for a child process, and removes the keychain afterward. Provisioning
profiles already have a separate persistent installer under
`asc profiles local install`. There is no equivalent non-interactive path for
keeping a private signing identity in a dedicated keychain across commands or
CI steps.

## Decision

Add:

```bash
asc signing keychain install \
  --identity .asc/signing/App.p12 \
  --identity-password-file .asc/secrets/p12-password \
  --keychain .asc/keychains/release.keychain-db \
  --keychain-password-file .asc/secrets/keychain-password \
  --add-to-search-list \
  --confirm
```

The command creates one new, dedicated keychain and imports exactly one
currently valid code-signing identity. It never imports into an existing
keychain. Refusing an existing destination avoids changing unrelated keys and
keeps the partition-list update scoped to the newly created identity.

The keychain and PKCS#12 passwords come only from protected regular files.
Neither secret is accepted directly as a flag, included in output, or placed
in a subprocess argument. `--confirm` is required because the keychain remains
on disk after the process exits. `--add-to-search-list` is explicit and
preserves the existing user search-list order.

The operation shares the signing-environment lock used by `asc signing run`.
Keychain creation, import, verification, and the complete search-list
transaction are serialized so one command cannot replace another command's
new search-list entry with a stale snapshot.

An optional `--expected-certificate-sha256` binds the operation to a known
certificate. The PKCS#12 is decoded and checked for a matching private key,
current certificate validity, code-signing usage, and the expected digest
before the keychain is created.

Before import and the final codesign usability probe, the new keychain is
temporarily staged in the user search list. macOS identity resolution requires
that staging even when codesign receives the keychain path explicitly. The
entry is removed again after a successful install unless
`--add-to-search-list` was requested. The final probe is copied into an
ASC-owned private temporary directory, never the operator-selected keychain
directory. An existing file beside the destination keychain is therefore
outside the probe's write and cleanup scope.

## Failure and rollback contract

All input, destination, and output-format validation happens before the first
side effect. The user search list is snapshotted before keychain creation in
both activation modes. Once creation succeeds, any later import, verification,
isolation, or search-list failure triggers deletion of the new keychain and
restoration of that exact snapshot, including a stale destination entry that
existed before the destination file did. A host-created automatic or stale
destination entry is isolated while import and verification run, then removed
when `--add-to-search-list` is absent; explicit activation keeps the staged
entry after verification. If both the primary operation and rollback fail,
both errors are returned. Without `--add-to-search-list`, a fresh user search-list
read must confirm that the destination is absent before emitting a success
receipt. macOS can report successful removal while retaining the entry when
the requested list is empty; this triggers rollback instead of success.
Rollback reads back the exact original list after deletion and restoration,
reports successful cleanup only when those operations and verification succeed,
and preserves the original failure alongside any cleanup or readback error.
The focused regression is `TestExecuteSigningKeychainInstallVerifiesSearchListChanges`.
Rollback
uses an independent bounded context, so cancellation of the initiating command
does not prevent keychain deletion or search-list restoration. The same rule
applies when cancellation interrupts initial keychain configuration before the
outer installer can mark creation complete. On macOS, SIGINT, SIGTERM, and
SIGHUP cancel the initiating
context so termination follows this rollback path. Keychain creation and
unlocking are separate checked stages;
an unlock failure deletes the newly created destination through the same
Security framework keychain reference before returning. A cleanup failure is
joined with the unlock failure.

The command does not delete, replace, or merge an existing keychain. It does
not install provisioning profiles; use `asc profiles local install` for that
independent persistent operation. It does not change the default keychain.

## Output

JSON output is a computed result with only public information:

```json
{
  "action": "installed",
  "keychainPath": "/absolute/path/release.keychain-db",
  "certificateSha256": "...",
  "certificateSha1": "...",
  "teamId": "TEAM12345",
  "searchListUpdated": true
}
```

Human-readable renderers expose the same fields. Source identity paths and all
password material are omitted.

## Alternatives and trade-offs

Import into the login or another existing keychain was rejected because a
partition-list update can affect unrelated private keys and because rollback
cannot safely restore an arbitrary shared keychain. Reusing `asc signing run`
was rejected for archive and export pipelines split across multiple commands:
its keychain is intentionally deleted when its one child process exits.
Passing passwords to `/usr/bin/security` as arguments was rejected because
process listings can expose them. The Security framework handles creation and
PKCS#12 import, while the one utility operation that needs the keychain
password receives it on stdin.

The dedicated-keychain approach consumes one explicit path and password file,
and the operator owns successful-run cleanup. In exchange, it provides a
bounded mutation surface, deterministic rollback, and a receipt that can be
passed between CI steps without secret material.

## Edge cases and failure modes

The destination parent must resolve to an existing trusted directory, and the
destination file must not exist. Symlinked or multiply linked secret inputs,
weak input permissions, empty passwords, malformed PKCS#12 data, missing
private-key binding, expired or not-yet-valid certificates, non-code-signing
certificates, fingerprint mismatches, import failures, unusable identities,
and search-list failures all stop the operation. A PKCS#12 certificate chain is
allowed, but the inspected leaf certificate must appear exactly once after
import.

Concurrent signing commands wait on the shared lock. Cancellation while
waiting returns without mutation. Cancellation after creation triggers the
same independent-context rollback as any other post-creation failure.

## Compatibility

The command is additive and macOS-only. It requires a cgo-enabled build so the
Security framework can create the keychain and import PKCS#12 data without
putting passwords in process arguments. Other platforms return a validation
error without reading secrets or creating files.

It does not call App Store Connect, change a project, create a Git commit, or
push a repository. The only persistent successful-run changes are the new
keychain file and, when requested, one user search-list entry. Provisioning
profile installation remains a separate explicit command.

## Validation and live verification

Unit coverage verifies command validation, command help,
private input handling, certificate checks, exact output fields, destination
refusal, search-list isolation and activation, shared-lock ordering,
independent-context rollback after cancellation, exact search-list restoration,
rollback error propagation, configuration-failure cleanup after cancellation,
and probe isolation from the destination directory. The Darwin Security
framework wrapper deletes the just-created keychain through its retained
reference when unlocking fails and reports any cleanup error alongside the
unlock error. Darwin coverage accepts a leaf certificate plus its chain while
rejecting a missing or duplicated leaf. cgo-disabled, Linux, and Windows
compile paths verify the platform guard.

A gated macOS integration test (`ASC_SIGNING_KEYCHAIN_INSTALL_LIVE_TEST=1`)
creates a unique disposable keychain, imports a protected PKCS#12 containing a
generated self-signed non-CA code-signing leaf, runs the real
Security-framework import, partition-list update, and codesign probe, confirms
search-list activation, removes the entry, deletes the keychain, and proves the
exact original search list is restored. It uses no personal signing
credentials; the gate is opt-in because it mutates only the operator's
temporary keychain/search-list state during the test. Repository formatting,
documentation, build, lint, and full tests run before each push. The built CLI
is also checked for exit 2 and empty stdout when `--confirm` is missing.

## Unresolved risks

Successful installation is intentionally persistent. A terminated runner that
does not execute its outer cleanup can leave the dedicated keychain on disk;
CI must install a trap before invoking the command. The command verifies local
import and codesign usability but cannot prove a later archive, export, or
remote signing service will accept the identity. It also does not rotate or
delete previously installed persistent keychains.

# Agent-native ad hoc distribution

## Product direction

ASC should turn a local Xcode archive into a verifiable install result that an
agent can hand to a person or another system. The public contract is based on
the outcome, not on a particular automation framework or storage-provider
vocabulary. Each stage emits
structured output, writes deterministic artifacts under an operator-selected
root, and can be retried without repeating completed account mutations.

The complete workflow is planned as separate reviewable changes:

1. Generate modern Xcode `release-testing` export options and export an IPA.
2. Inspect the IPA and prepare a self-contained web-install bundle.
3. Publish that bundle through a caller-provided S3-compatible endpoint.
4. Reconcile registered devices and ad hoc provisioning profiles.
5. Sync and install the private signing identity under explicit local control.
6. Compose the stages into a resumable run with a durable receipt.

The first change in this stack owns only item 1.

## Placement and current behavior

`asc xcode export-options generate` currently always writes
`method=app-store-connect`. `asc xcode export` implicitly generates the same
options when `--export-options` is omitted. A caller can provide a custom plist,
but ASC cannot generate the non-App-Store export used for ad hoc delivery.

Xcode 26.6 and Xcode 27 call this method `release-testing`. Both versions still
accept `ad-hoc`, but mark it deprecated. ASC will use the current Xcode name and
will not introduce the deprecated spelling as a new public value.

## PR 1 public contract

The standalone generator adds:

```text
asc xcode export-options generate \
  --archive-path .asc/artifacts/App.xcarchive \
  [--method app-store-connect|release-testing] \
  [--destination export|upload] \
  [--signing-style automatic|manual]
```

`--method` defaults to `app-store-connect`, preserving every existing
invocation. `release-testing` requires `--destination export` because it creates
a local IPA rather than an App Store Connect upload. Its default output is
`.asc/export-options-release-testing.plist`; the existing App Store default
remains `.asc/export-options-app-store.plist`.

`asc xcode export` receives the same `--method` flag for its implicit generator:

```text
asc xcode export \
  --archive-path .asc/artifacts/App.xcarchive \
  --method release-testing \
  --signing-style manual \
  --ipa-path .asc/artifacts/App.ipa
```

An explicit `--export-options` file remains authoritative and cannot be combined
with `--method`, `--signing-style`, or `--team-id`. The default export method
remains `app-store-connect`. No existing output field changes; `method` reports
the actual generated value. Invalid values are usage errors with exit code 2.
Data remains on stdout and diagnostics remain on stderr.

## Implementation and compatibility

The repository-owned generator passes the selected method to the pinned Bitrise
typed models. App Store Connect continues to use the App Store model.
Release-testing uses the non-App-Store model. Manual signing resolution receives
the selected method so profile selection matches the requested export. The
pinned resolver still classifies installed ad hoc profiles with Xcode's legacy
`ad-hoc` enum, so ASC translates only at that internal resolver boundary and
continues to emit `release-testing` in the generated plist.

The change is additive. No deprecation or migration is required. The legacy
`ad-hoc` Xcode spelling is intentionally rejected with guidance to use
`release-testing`.

## RED-GREEN and verification

Coverage must establish:

- valid generator and implicit-export parsing for both methods;
- invalid and explicitly empty method values as usage errors;
- rejection of release-testing with `destination=upload` or `xcode export --wait`;
- conflict errors when `--method` accompanies an explicit plist;
- exact `method=release-testing` plist and JSON output;
- manual generator receipt of the selected method;
- portable and Darwin typed-model parity;
- unchanged app-store-connect defaults;
- generated command documentation, focused tests, built-binary stdout/stderr and
  exit codes, followed by the repository validation gate;
- real archive export with Xcode 26.6 and Xcode 27 before the distribution stack
  is declared complete.

## Handoff and promotion gates

Each slice must be committed on its own feature branch and pushed at the exact
revision that passed its focused tests and repository validation gates. A
downstream slice must not be folded into the same commit merely because it uses
the preceding command. Review handoff must record the tested Xcode versions,
the exact commit, live verification performed, and any gate that remains.

`--method` remains experimental until the complete workflow has exported a real
archive, published a fetch-verified HTTPS manifest and IPA, and installed the
expected bundle and build on a registered device. Manual exports still depend
on a locally available distribution private key and provisioning profiles that
cover every embedded target and capability. Later slices must also settle the
security and retention contract for caller-provided storage, bearer install
URLs, device identifiers, and resumable state before `asc distribute` can be
promoted to stable.

## Alternatives

Accepting `ad-hoc` would mirror older automation tools but create a deprecated
surface on day one. Hiding the method only inside the future distribution
orchestrator would leave `asc xcode export` incomplete and make that orchestrator
depend on a private code path. Supporting every Xcode export method in this
change would widen the review without helping the first install-link workflow.

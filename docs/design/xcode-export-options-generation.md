# Xcode export-options generation

## Placement and current behavior

This change adds `asc xcode export-options generate` beneath the existing
`asc xcode` group. No top-level registry entry changes.

Before this change, `asc xcode export` required `--export-options`. Local-build publish accepted
the flag or reuses `.asc/export-options-app-store.plist`, then fails before the
archive when neither exists. The documented workflow asks users to hand-write
that plist, including signing details for embedded extensions when automatic
signing is insufficient.

## Public command shape

The standalone command is:

```text
asc xcode export-options generate \
  --archive-path .asc/artifacts/App.xcarchive \
  [--method app-store-connect|release-testing] \
  [--output-path ExportOptions.plist] \
  [--destination export|upload] \
  [--signing-style automatic|manual] \
  [--team-id TEAM_ID] [--overwrite] \
  [--output json|table|markdown] [--pretty]
```

Defaults are `method=app-store-connect`, `destination=export`,
`signingStyle=automatic`, and the deterministic
`.asc/export-options-app-store.plist` output path. `release-testing` uses
`.asc/export-options-release-testing.plist` and requires `destination=export`.
The command never silently
replaces that file: a repeat write requires explicit `--overwrite` consent. The first
release supported only App Store Connect; this extension adds Xcode's current
name for registered-device distribution. Xcode's deprecated `ad-hoc` spelling
is rejected with guidance to use `release-testing`.

The archive is required so ASC can validate that the artifact is an Xcode
archive and infer its team and bundle metadata. `--team-id` overrides archive
inference. Automatic signing omits provisioning-profile mappings and lets
Xcode resolve every embedded app, extension, widget, watch component, or App
Clip. Manual signing delegates local certificate/profile selection to Bitrise's
export-options generator for its supported iOS/tvOS archive shapes and fails
clearly for unsupported or ambiguous archives; users can always retain an
explicit custom plist.

Existing invocations remain valid. `asc xcode export` accepts the same
`--method` value for implicit generation and makes
`--export-options` optional. When omitted it generates a uniquely named,
archive-adjacent plist for the current run; `--wait` selects
`destination=upload`, otherwise `destination=export`. The implicit path is
chosen only when it does not exist, so this convenience path never clobbers a
prior file. An explicit plist remains byte-for-byte authoritative.

Local-build `asc publish testflight` and `asc publish appstore` preserve their
precedence: an explicit `--export-options` wins, then an existing
`.asc/export-options-app-store.plist`. When neither exists, generation happens
after the archive and before export with `destination=export`, using the same
unique, non-clobbering archive-adjacent naming. Publish never generates
direct-upload options because it requires the local IPA for its upload stage.

No command prompts. Usage errors return exit code 2. Data stays on stdout and
diagnostics stay on stderr.

## Structured output

The generator result contains:

- `path` and `archive_path`;
- `method`, `destination`, and `signing_style`;
- inferred or explicit `team_id` when available;
- `signing_certificate` and a stable bundle-ID-to-profile map for manual
  signing and both supported methods;
- `overwritten`.

JSON uses those field names. Table and Markdown render the same values with one
stable row per provisioning-profile mapping. Existing xcode export and publish
results remain compatible; their export-options path records the actual
generated or explicit artifact.

## Implementation

ASC pins Bitrise `go-xcode/v2` master commit `ba29d6757432f0b8e23208a0ef2c0ee2103c8c3e`
at immutable pseudo-version
`v2.0.0-alpha.84.0.20260710143042-ba29d6757432`. The v2 generator currently
returns the stable v1 `exportoptions.ExportOptions` interface, so the adapter
contains that upstream split and exposes only repository-owned types.

ExportOptions keys are constructed through Bitrise's typed models: the pinned
v2 model on Darwin and the stable v1 model for portable automatic generation.
All `go-xcode/v2` and `go-utils/v2` imports live behind Darwin build tags so
Linux and Windows builds do not compile macOS-only signing dependencies. ASC
validates the final dictionary against its public contract before serialization.
Bitrise's direct file writer is not used: the complete plist is marshaled and
decoded first, then written with the repository's same-directory, symlink-safe
atomic writer. Parent directories are created; an existing standalone
destination requires `--overwrite`; directories and symlinks are rejected. A
failed validation or write leaves the previous file unchanged.

Automatic generation uses archive metadata only and does not touch the
Keychain or App Store Connect. Manual generation is macOS-only and may inspect
locally installed signing identities and provisioning profiles through Bitrise.
It does not create or mutate signing assets.

## Compatibility and lifecycle

The existing generator remains a stable additive subcommand. The new `--method`
extension and its `release-testing` value are experimental until the complete
direct-install workflow passes its promotion gates. Explicit export-options
files retain precedence and behavior, including custom Xcode keys not modeled
by ASC or Bitrise. No deprecation is required.

The command remains visible on every platform with the rest of `asc xcode`.
Automatic plist construction and atomic writing are cross-platform when the
archive can be inspected locally. Manual local-signing resolution and actual
archive export require macOS/Xcode.

## RED-GREEN and verification

Coverage includes:

- help, valid flags, invalid destination/signing style, positional arguments,
  output formats, and usage exit code 2;
- automatic generation with inferred and explicit team IDs;
- destination export/upload round-trips through direct-upload detection;
- manual generator success, invalid or missing signing output, team mismatch,
  and stable multi-bundle provisioning-profile mappings;
- a real app with an embedded widget extension under both tested Xcode versions;
- missing/malformed archives, unsafe application paths, mismatched bundle IDs,
  and unsupported or missing platform metadata;
- fresh writes, parent creation, overwrite refusal, atomic replacement,
  symlink/directory rejection, injected failures, and parse-before-write;
- explicit-plist precedence and automatic generation in xcode export;
- explicit/default/generated precedence in both local publish commands;
- built-binary stdout/stderr and exit-code checks;
- generated plists against local Xcode 26.6 and Xcode 27 beta 3, using the
  multi-target Presset project in temporary build/archive directories;
- focused tests followed by format, generated command docs, docs checks, lint,
  and the full test suite.

## Alternatives

A fixed four-key plist without archive inspection is smaller but does not catch
the multi-target signing failures that motivated the feature. Reimplementing
Bitrise's certificate/profile matcher would duplicate mature Go code. Requiring
Fastlane, Ruby, or Xcode UI export violates ASC's single-binary and noninteractive
contract. Making every Xcode export key a first-release flag recreates the
verbosity this command is meant to remove; explicit plists remain the escape
hatch.

# Selected Xcode smoke

The manually dispatched `Selected Xcode smoke` workflow selects Xcode with
`DEVELOPER_DIR` and records its actual version, build and path. It is independent
of PR checks. The `xcode-27` runner currently provides Xcode 27.0, 27.1 and 27.2;
the choices come from the [published runner inventory](https://github.com/actions/runner-images/blob/1ede08b87c78b9a8871f4761a20f9b5fd94796c4/images/macos/xcode-27-arm64-Readme.md#xcode).
If an image removes a selected version, the smoke fails instead of using another
toolchain silently.

Run the same check locally with a built CLI and a new work directory:

```bash
ASC_BYPASS_KEYCHAIN=1 DO_NOT_TRACK=1 python3 scripts/xcode_smoke.py \
  --binary ./asc --developer-dir /Applications/Xcode.app/Contents/Developer \
  --xcode-version 27.1 --work-dir /tmp/asc-xcode-smoke
```

The script uses an isolated HOME/config, discovers a copied Xcode project,
enumerates available iOS simulator destinations, and runs two unsigned macOS
XCTest cases. It checks the passed/skipped counts in the real `.xcresult` and
converted JUnit report. Compilation and tests use one worker. JSON receipts,
diagnostics, binary digest and runner image details are retained as workflow
artifacts. Local results also retain the generated project and result bundle.

It does not change the system's Xcode selection, boot or modify simulators,
install applications, sign artifacts, or call App Store Connect. This covers
runtime enumeration and host-side result parsing, not iOS test execution.

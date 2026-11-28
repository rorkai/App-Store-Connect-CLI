# Prebuilt IPA metadata validation

`asc publish appstore` and `asc publish testflight` now require a readable
top-level app `Info.plist` and a nonempty `CFBundleIdentifier` in prebuilt IPAs.
Invalid archives are rejected locally before authentication, app lookup, or
upload reservation, including with both `--version` and `--build-number`.
The same validation applies to `asc publish appstore --dry-run`.

Previously, providing both flags allowed an unreadable or missing plist to
bypass metadata checks. Re-export those archives with their app metadata before
publishing. Valid IPAs still support explicit version and build-number overrides,
including when those version keys are absent from an otherwise readable plist.
PKG publishing and successful publish receipts are unchanged.

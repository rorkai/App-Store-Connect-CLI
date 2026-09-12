# Prebuilt macOS PKG publishing

`asc publish testflight` and `asc publish appstore` now accept a prebuilt macOS
installer through `--pkg`. PKG uploads select `MAC_OS` automatically and
require explicit `--version` and `--build-number` values.

The existing `--ipa --platform MAC_OS` combination remains available during a
compatibility window, but now prints a deprecation warning. Migrate those
invocations to:

```bash
asc publish appstore \
  --app "APP_ID" \
  --pkg "MacApp.pkg" \
  --version "1.2.3" \
  --build-number "42"
```

The deprecated macOS IPA publishing combination will be removed in a future
major release. IPA publishing for iOS, tvOS, and visionOS is unchanged.

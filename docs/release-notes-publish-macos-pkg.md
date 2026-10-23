# Prebuilt macOS PKG publishing

> Status: the `--ipa --platform MAC_OS` combination described below was removed
> in 6.0.0 from `asc publish testflight` and `asc publish appstore`. It now
> fails with a usage error that names `--pkg` with `--version` and
> `--build-number`. IPA publishing for iOS, tvOS, and visionOS is unchanged.

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

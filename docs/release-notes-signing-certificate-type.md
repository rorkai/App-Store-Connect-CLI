# Signing creates cross-platform certificates

Changed in 7.0.0. When `asc signing fetch` or `asc signing sync push` runs
with `--create-missing-certificate` for an iOS or tvOS profile and the team has
no active certificate, it now creates Apple's cross-platform type:

| Profile types | Created before | Created now |
| --- | --- | --- |
| `IOS_APP_STORE`, `IOS_APP_ADHOC`, `TVOS_APP_STORE`, `TVOS_APP_ADHOC` | `IOS_DISTRIBUTION` | `DISTRIBUTION` |
| `IOS_APP_DEVELOPMENT`, `TVOS_APP_DEVELOPMENT` | `IOS_DEVELOPMENT` | `DEVELOPMENT` |

`DISTRIBUTION` ("Apple Distribution") and `DEVELOPMENT` ("Apple Development")
are the types Xcode creates. One certificate signs for iOS, tvOS, watchOS,
visionOS and macOS.

## Unchanged

- An active `IOS_DISTRIBUTION` or `DISTRIBUTION` certificate still satisfies
  the lookup, and so does `IOS_DEVELOPMENT` or `DEVELOPMENT` for development
  profiles. Nothing is created when one exists, so a team with a legacy
  certificate does not use another distribution certificate slot.
- Mac, Mac Catalyst and Developer ID profiles create the same types as before.
- In-house profiles (`IOS_APP_INHOUSE`, `TVOS_APP_INHOUSE`) still create
  `IOS_DISTRIBUTION`.
- A type passed with `--certificate-type` is created as given.

The inferred type list is now ordered with the cross-platform type first, for
example `DISTRIBUTION,IOS_DISTRIBUTION`. It appears in that order in the
certificates `filter[certificateType]` request and in the error
`no certificates found for type ...`.

## Migration

To keep creating the legacy type while still reusing either kind, name both
types with the legacy one first. The first type in the list is the one created:

```bash
asc signing fetch \
  --bundle-id com.example.app \
  --profile-type IOS_APP_STORE \
  --create-missing \
  --create-missing-certificate \
  --certificate-type IOS_DISTRIBUTION,DISTRIBUTION \
  --identity-password-file .asc/secrets/distribution-p12-password
```

Use `IOS_DEVELOPMENT,DEVELOPMENT` for development profiles. Passing only
`--certificate-type IOS_DISTRIBUTION` also creates the legacy type, but then an
existing `DISTRIBUTION` certificate no longer satisfies the lookup.

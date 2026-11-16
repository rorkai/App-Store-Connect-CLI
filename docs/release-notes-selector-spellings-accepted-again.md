# Selector spellings removed in 5.0 are accepted again

Changed in 7.0.0. The flag spellings that 5.0.0 removed in favour of another
flag of the same command with the same meaning run again, as a second name
for that flag. For example, `asc versions view --id VERSION_ID` now runs as
`asc versions view --version-id VERSION_ID` does, where 5.0 to 6.x printed:

```text
Error: `--id` was removed in 5.0.0; use `--version-id` (see migrate-to-5-0)
```

The restored spelling is not shown in help, prints no warning, and behaves
exactly as the flag it stands for: the same validation, the same error text
(which names the documented flag), and the same request. Giving both
spellings with different values exits with status 2:

```text
Error: `--id` and `--version-id` are the same flag; pass one value
```

| Command | Spelling | Same as |
| --- | --- | --- |
| `apps view` | `--app` | `--id` |
| `apps public view`, `prices`, `descriptions` | `--id` | `--app` |
| `apps info view` | `--app-info` | `--info-id` |
| `apps info relationships *`, `apps info territory-age-ratings list` | `--id` | `--info-id` |
| `builds list` | `--app-id` | `--app` |
| `builds wait` | `--newest` | `--latest` |
| `builds app view`, `pre-release-version view`, `icons list`, `beta-app-review-submission view`, `build-beta-detail view`, `app-encryption-declaration view` | `--id` | `--build-id` |
| `builds test-notes view`, `update`, `delete` | `--id` | `--localization-id` |
| `bundle-ids capabilities list` | `--bundle-id` | `--bundle` |
| `localizations list`, `localizations update` | `--version-id` | `--version` |
| `screenshots list` | `--localization-id` | `--version-localization` |
| `subscriptions view` | `--subscription-id` | `--id` |
| `subscriptions list`, `subscriptions view` | `--version-limit` | `--versions-limit` |
| `subscriptions versions list`, `view` | `--image-limit`, `--localization-limit` | `--images-limit`, `--localizations-limit` |
| `subscriptions review screenshots delete` | `--id` | `--screenshot-id` |
| `testflight groups view` | `--group-id` | `--id` |
| `versions view`, `versions update` | `--id` | `--version-id` |
| `xcode-cloud status` | `--id` | `--run-id` |
| The commands below that have no `--build-number` | `--build` | `--build-id` |

`--build` is accepted again on `apps app-encryption-declarations list`,
`build-bundles list`, `build-localizations list` and `create`, `encryption
declarations list` and `assign-builds`, `performance diagnostics list`,
`download` and `metrics view`, `release stage`, `testflight config export`,
`crashes list`, `distribution view`, `feedback list`, `notifications send`,
`review submit`, `review submissions list`, `testers list`, `add-builds`,
`remove-builds` and `export`, and `validate testflight`.

Still removed, with the same message as before: `--build` on commands that
also take `--build-number` (it could mean either), and the flags whose
replacement is a command, an environment variable or nothing:
`testflight distribution edit --external-testing`, `pre-orders enable
--available-in-new-territories`, `game-center details list --limit`, `--next`
and `--paginate`, `game-center details create` and `update
--challenge-enabled`, `web … --two-factor-code`, and `signing sync push` and
`pull --password`.

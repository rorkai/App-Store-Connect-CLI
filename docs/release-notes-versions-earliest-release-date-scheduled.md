# Earliest release date sets a scheduled release

Changed in 7.0.0. App Store Connect applies `earliestReleaseDate` only when the
version's release type is `SCHEDULED`. Earlier releases sent
`--earliest-release-date` on its own, so a version kept its manual or
after-approval release and the date had no effect.

`asc versions create` and `asc versions update` now handle the date this way:

| Flags | 7.0.0 behavior |
|---|---|
| `--earliest-release-date` without `--release-type` | Also sends `releaseType: SCHEDULED` and prints `Warning: release type set to SCHEDULED because --earliest-release-date was given` on stderr |
| `--earliest-release-date` with `--release-type MANUAL` or `AFTER_APPROVAL` | Usage error, exit code `2`, before any request |
| `--earliest-release-date` with `--release-type SCHEDULED` | Unchanged |
| No `--earliest-release-date` | Unchanged |

## Migration

Pass `--release-type SCHEDULED` with `--earliest-release-date` to keep the
request explicit and stderr quiet:

```bash
asc versions update --version-id "VERSION_ID" --release-type SCHEDULED --earliest-release-date "2026-02-01T08:00:00+00:00"
```

Scripts that passed the date together with `--release-type MANUAL` or
`--release-type AFTER_APPROVAL` must drop one of the two flags.

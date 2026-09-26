# AppInfo kidsAgeBand deprecation

API 4.5 removed `kidsAgeBand` from the `AppInfo` response schema. AppInfo sparse
selectors now emit one deprecation warning on stderr per invocation. Existing
flags and HTTP query forwarding remain available for compatibility, and JSON
stdout is unchanged. Apple errors are still reported normally.

The warning applies to `apps`/`apps list`, `apps view`, `apps info list`,
`apps info view`, `localizations list --type app-info`, and
`xcode-cloud products app` when they select AppInfo `kidsAgeBand`. Supported
continuation URLs carrying that selector also warn.

Replace reads such as:

```bash
asc apps info view --info-id "APP_INFO_ID" --fields kidsAgeBand
```

with:

```bash
asc age-rating view --app-info-id "APP_INFO_ID"
```

`AgeRatingDeclaration.kidsAgeBand` remains in API 4.5, and the age-rating
`--kids-age-band` edit flag is unchanged. This release does not remove any
stable flag or announce a removal date.

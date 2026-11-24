# `asc localizations create` and `update` check that the selected app owns a version ID

Changed in 7.4.0. With `--version` set to an App Store version ID,
`asc localizations create` and `asc localizations update` now read the version
(`GET /v1/appStoreVersions/{id}?include=app`) and compare its app with the app
selected by `--app`, `ASC_APP_ID`, or `app_id` in the CLI config. A version of
another app is refused before the locale lookup and before any write, with exit
code `1`:

```text
$ ASC_APP_ID=9876543210 asc localizations update --version "VERSION_ID" --locale "en-US" --whats-new "Bug fixes"
Error: localizations update: verify version ownership: version "VERSION_ID" belongs to app "1234567890", not "9876543210"
```

| Case | Through 7.3.1 | 7.4.0 |
|---|---|---|
| version ID, no app selected | the write is sent | unchanged |
| version ID, `ASC_APP_ID` or config app owns the version | the app is ignored; the write is sent | the write is sent after the ownership read |
| version ID, `ASC_APP_ID` or config app is another app | the app is ignored; the write is sent | refused, exit `1` |
| version ID with `--app` | usage error (`--app only applies when --version is a version string`), exit `2` | checked like `ASC_APP_ID` |
| version string such as `1.2.3` | resolved on the app | unchanged |

The selected app may be a numeric app ID, an apps `links.self` URL, a bundle
ID, or an exact app name. A bundle ID or name is looked up first
(`GET /v1/apps?filter[bundleId]=...`), so a default such as
`ASC_APP_ID=com.example.app` is compared by the app ID it resolves to.

## Migration

A script that sets `ASC_APP_ID` (or a config `app_id`) to one app and passes
version IDs of another app to these two commands now fails. Pass the owning app
for that call; `--app` takes precedence over `ASC_APP_ID` and the config:

```bash
asc localizations update --app "OWNING_APP_ID" --version "VERSION_ID" --locale "en-US" --whats-new "Bug fixes"
```

# App-tag territory deprecation

Apple's [API 4.5 release notes](https://developer.apple.com/documentation/appstoreconnectapi/app-store-connect-api-4-5-release-notes)
deprecate the app-tag `territories` relationship and the full-resource and
ID-only territory endpoints. They also retire territory-related list parameters,
although the published OpenAPI snapshot still exposes them.

The CLI retains existing flags, HTTP requests, and pagination for compatibility.
Territory selections on `app-tags list`/`view`, the `territories` and
`territories-links` commands, and territory parameters in continuation URLs now
emit one stderr warning per invocation. JSON stdout and API errors are unchanged.
No stable command is removed and no removal date is announced.

For core tag reads, omit territory selections:

```bash
asc app-tags list --app "APP_ID" --fields name,visibleInAppStore
asc app-tags view --app "APP_ID" --id "TAG_ID"
```

Remove `territories` from `--fields` and omit `--include territories`,
`--territory-fields`, and `--territory-limit`. Apple does not name an equivalent
replacement for territory lookups. `app-tags links` and tag visibility updates
remain warning-free.

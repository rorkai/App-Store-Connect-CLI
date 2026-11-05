# INSTALLS sales report deprecation

Apple's [API 4.6 release notes](https://developer.apple.com/documentation/appstoreconnectapi/app-store-connect-api-4-6-release-notes)
deprecate `INSTALLS` for GET `/v1/salesReports`. Apple stopped generating new
INSTALLS reports on November 4, 2026; historical reports remain downloadable.

`asc analytics sales --type INSTALLS` now warns on stderr. Requests, downloads,
JSON stdout, and API errors retain their existing behavior.

For a historical download, choose a reporting period for which Apple already
generated a report:

```bash
asc analytics sales --vendor "12345678" --type INSTALLS \
  --subtype SUMMARY --frequency MONTHLY --date "2026-09"
```

Update recurring jobs so they no longer expect new INSTALLS reports. Apple does
not name a replacement in the 4.6 release notes.

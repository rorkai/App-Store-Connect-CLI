# App Store Server Notifications URLs

`asc apps update` currently supports bundle ID, primary locale, and content rights,
but cannot configure App Store Server Notifications endpoints. Add two optional
flags in the existing command rather than a separate command or web-only path:

```bash
asc apps update --id APP_ID --subscription-status-url https://example.com/notifications
asc apps update --id APP_ID --sandbox-subscription-status-url https://example.com/sandbox-notifications
```

The offline OpenAPI snapshot supports both fields in `PATCH /v1/apps/{id}` via
`AppUpdateRequest.data.attributes`: `subscriptionStatusUrl` and
`subscriptionStatusUrlForSandbox`. Each URL is paired with its corresponding
`subscriptionStatusUrlVersion` or `subscriptionStatusUrlVersionForSandbox` set to
`V2`. The schema permits V1 and V2; this command explicitly configures V2 signed
payloads, while omitted endpoints retain their current version and URL. URL removal
and configuring legacy V1 payloads are outside this additive change.

URLs must use absolute HTTPS without credentials or fragments. Explicit empty
values are usage errors, validated before authentication or network access. Existing
app metadata flags remain available and can be combined with either URL flag.
Mutations keep the Apple resource envelope and existing output formats. Add all four
fields to app response attributes so JSON reads and mutation responses retain them.

Verification covers exact PATCH payloads for each endpoint, combined metadata,
omitted-field preservation, returned URLs and versions, and invalid URLs without
network activity. Establish RED on the unsupported flags, reach GREEN, verify built
binary help and usage-error exit code 2, and run the repository validation and review
gates. Any authorized live update must target only the operator-selected app and
read its configuration back afterward.

Live verification found that Apple omits notification fields from default GET app
responses even after a successful update. Add `apps view --fields` for the app
attributes decoded by this client, passing the selection through `fields[apps]`.
The exact GET endpoint schema permits all four notification fields. Default reads
stay unchanged; explicit empty or unknown selectors are usage errors. A CLI
regression asserts the sparse-field query and all four decoded response values.

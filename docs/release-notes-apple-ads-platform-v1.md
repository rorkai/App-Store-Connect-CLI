# Apple Ads Platform API v1 and v5 deprecation

> Status: the `asc ads v5` namespace described below was removed in 6.0.0.
> See `docs/release-notes-apple-ads-v5-removal.md`.

Release 4.4.0 makes Apple Ads Platform API v1 the direct `asc ads` resource
surface. Campaign Management API v5 remains runnable under `asc ads v5`, but
Apple retires that API on January 26, 2027 and every v5 leaf now prints a dated
deprecation warning on stderr.

Existing scripts that must keep their v5 endpoint, payload, IDs, response, and
organization context need to add `v5` to the command path:

| Before 4.4.0 | 4.4.0 compatibility path |
| --- | --- |
| `asc ads campaigns list --org ORG_ID` | `asc ads v5 campaigns list --org ORG_ID` |
| `asc ads reports campaigns --org ORG_ID --file report.json` | `asc ads v5 reports campaigns --org ORG_ID --file report.json` |
| `asc ads reports preset --level ads --org ORG_ID` | `asc ads v5 reports preset --level ads --org ORG_ID` |
| `asc ads api request --path v5/me` | `asc ads v5 api request --path v5/me` |

New automation should use the direct v1 commands. V1 account-scoped requests
take `--ad-account` or `ASC_ADS_AD_ACCOUNT_ID`; they don't substitute the v5
`--org` value. Query and report bodies also use v1 schemas, so the CLI doesn't
translate old selectors or reporting requests:

```bash
asc ads campaigns find \
  --ad-account "AD_ACCOUNT_ID" \
  --file query.json \
  --output json

asc ads reports apps campaigns \
  --ad-account "AD_ACCOUNT_ID" \
  --file report.json \
  --output json
```

Named Ads profiles keep their organization and ad-account contexts separate.
After upgrading, a selected profile won't inherit either value from the root
Ads config. If an older named profile depended on the root `ads.org_id`, store
its organization again with `asc ads auth login --name NAME --org ORG_ID`, or
pass `--org` on its v5 command. Profile-less access-token and environment auth
can still use the root context.

`asc ads auth discover` now calls Platform API v1 `GET /v1/me` and
`GET /v1/acls`. Use `asc ads me view` and `asc ads acls list` when a script
needs one response at a time.

Seven v5 leaves have no one-command v1 replacement: product-page countries,
product-page devices, three keyword bulk-delete operations, and custom
impression-share report list and view. Their warnings explain the manual
migration path. Run the matching command with `--help` before removing the v5
namespace from an existing script.

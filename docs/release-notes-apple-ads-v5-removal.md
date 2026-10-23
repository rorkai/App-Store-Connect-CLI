# Apple Ads Campaign Management API v5 removal

Apple retired the Apple Ads Campaign Management API v5 on January 26, 2027.
Release 6.0.0 removes the `asc ads v5` namespace that 4.4.0 deprecated, along
with its raw request command, `reports preset` helper, and campaign
`pause`/`resume` workflows. The Platform API v1 commands under `asc ads` are
unchanged.

Every former `asc ads v5 ...` invocation now exits with status 2 before
authentication or any network request. The error names the v1 replacement for
the command when one exists:

```text
Error: `asc ads v5 campaigns list` was removed in 6.0.0 after Apple retired Campaign Management API v5 on January 26, 2027. Use `asc ads campaigns find`. Platform API v1 commands take --ad-account and v1 request bodies.
```

## Migrate a script

1. Replace each `asc ads v5 <command>` with the v1 command named in the error,
   or in the table in `commands/ads.mdx` ("Migrating from Campaign Management
   API v5").
2. Replace `--org ORG_ID` with `--ad-account AD_ACCOUNT_ID`, or set
   `ASC_ADS_AD_ACCOUNT_ID`. Find the ad account with
   `asc ads auth discover --output json` (`accounts[].ad_account_id`). The v1
   commands never fall back to the organization ID.
3. Rewrite request files with the v1 schema shown by the command's `--help`.
   The CLI doesn't translate a v5 `Selector` or `ReportingRequest`; v1 query
   commands reject the most common v5 selector members with a migration hint.
4. Update response parsing. V1 responses use `result` envelopes and string
   resource IDs.

| Removed command | Replacement |
| --- | --- |
| `asc ads v5 campaigns list --org ORG_ID` | `asc ads campaigns find --ad-account AD_ACCOUNT_ID --file query.json` |
| `asc ads v5 reports campaigns --org ORG_ID --file report.json` | `asc ads reports apps campaigns --ad-account AD_ACCOUNT_ID --file report.json` |
| `asc ads v5 reports preset --level LEVEL` | the `asc ads reports apps` command for `LEVEL` |
| `asc ads v5 campaigns pause --campaign ID --confirm` | `asc ads campaigns pause --campaign ID` |
| `asc ads v5 api request --path v5/...` | `asc ads api request --path v1/...` |

Seven v5 commands have no one-command v1 replacement: product-page countries
and devices, the three keyword bulk deletes, and custom impression-share report
list and view. Their removal errors describe the manual v1 path.

`asc ads auth login --org`, `ASC_ADS_ORG_ID`, and stored `org_id` values still
work; they only set the organization that `asc ads auth status` and
`asc ads auth discover` report.

# Saved catalog plan recovery and identity

`asc catalog plan --out` still refuses deferred targets. When a localization
needs a new version, its error now names the version-create command for that
resource. An existing approved product already has a parent; creating another
catalog plan for that parent cannot create its editable version by itself.

For a new product or group, first plan and apply a manifest that omits pricing,
offers, and localizations. When localization planning needs an editable version,
create it with the applicable command and the existing parent resource ID:

```bash
asc subscriptions versions create --subscription-id SUBSCRIPTION_ID
asc iap versions create --iap-id IAP_ID
asc subscriptions groups versions create --group-id GROUP_ID
```

Apple must permit version creation. Resolve any state error returned by the
command before retrying. Then generate and review a new saved plan with the
original manifest:

```bash
asc catalog plan --app APP_ID --file catalog.json --out catalog-plan.json
```

The `planSha256` field in plan and apply JSON receipts identifies the canonical
compact plan JSON. Its algorithm and field name are unchanged. The saved file
has indentation and a trailing newline, so `shasum -a 256 catalog-plan.json`
produces a different value. Formatting-only whitespace changes preserve plan
identity; other edits can invalidate the plan. Record the receipt's `planSha256`
when comparing the plan and apply receipts. Record a separate raw file checksum
if byte-for-byte file identity is also needed.

The saved plan's `manifestSha256` has a different contract: it hashes the exact
manifest file bytes, including formatting. Changing those bytes requires a new
saved plan.

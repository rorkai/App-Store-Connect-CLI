# Destructive commands ask for `--confirm` the same way

Changed in 7.0.0. A destructive command run without `--confirm` now prints one
of two lines, then its usage page, and exits with usage code `2`:

```text
Error: --confirm is required
Error: --confirm is required unless --dry-run is set     # commands that have --dry-run
```

It also checks `--confirm` after its other required flags, so a command missing
both an ID and `--confirm` names the ID first. `--confirm` is still checked
before the command sends a request, reads a file or prompts.

```text
$ asc webhooks delete
Error: --webhook-id is required          # was "--confirm is required"

$ asc versions delete --version-id VERSION_ID
Error: --confirm is required             # was "--confirm is required to delete a version"
```

Exit code `2`, the usage page and empty stdout are unchanged.

## One text (50 commands)

Commands that added a reason, a prefix or nothing at all now print
`Error: --confirm is required`:

- `asc app-clips advanced-experiences delete` (was `... to remove`)
- `asc app-clips default-experiences delete` (was `... to delete`)
- `asc app-clips default-experiences localizations delete` (was `... to delete`)
- `asc app-clips header-images delete` (was `... to delete`)
- `asc app-clips invocations delete` (was `... to delete`)
- `asc app-clips invocations localizations delete` (was `... to delete`)
- `asc app-events localizations placements delete` (was `placements delete: --confirm is required`)
- `asc app-events localizations placements swap` (was `placements swap: --confirm is required`)
- `asc app-events submit` (was `... to submit for review`)
- `asc asset-library images archive` (was `asset-library: --confirm is required`)
- `asc asset-library images delete` (was `asset-library: --confirm is required`)
- `asc asset-library videos archive` (was `asset-library: --confirm is required`)
- `asc asset-library videos delete` (was `asset-library: --confirm is required`)
- `asc auth logout` (was `... to remove stored credentials`)
- `asc builds expire` (was `... to expire build`)
- `asc localizations placements delete` (was `placements delete: --confirm is required`)
- `asc localizations placements swap` (was `placements swap: --confirm is required`)
- `asc localizations screenshot-sets delete` (was `... to delete`)
- `asc nominations delete` (was `... to delete`)
- `asc notarization staple` (was `... for in-place ticket stapling`)
- `asc product-pages custom-pages localizations placements delete` (was `placements delete: --confirm is required`)
- `asc product-pages custom-pages localizations placements swap` (was `placements swap: --confirm is required`)
- `asc product-pages custom-pages localizations preview-sets sync` (was `... to sync`)
- `asc product-pages custom-pages localizations screenshot-sets sync` (was `... to sync`)
- `asc product-pages experiments treatments localizations placements delete` (was `placements delete: --confirm is required`)
- `asc product-pages experiments treatments localizations placements swap` (was `placements swap: --confirm is required`)
- `asc product-pages experiments treatments localizations screenshot-sets sync` (was `... to sync`)
- `asc review items remove` (was `... to remove`)
- `asc review items-remove` (was `... to remove`)
- `asc review submissions-cancel` (was `... to cancel`)
- `asc review submissions-submit` (was `... to submit`)
- `asc screenshots apply` (was `... to apply screenshot uploads`)
- `asc screenshots delete` (was `... to delete`)
- `asc signing keychain delete` (was `signing keychain delete: --confirm is required`)
- `asc signing keychain install` (was `... to create a persistent signing keychain`)
- `asc signing sync rotate-password` (was `... to rotate the signing sync password`)
- `asc submit cancel` (was `... to cancel a submission`)
- `asc subscriptions pricing monthly-commitment disable` (was `... to disable monthly-commitment billing`)
- `asc subscriptions pricing monthly-commitment enable` (was `... to enable monthly-commitment billing`)
- `asc testflight groups delete` (was `... to delete`)
- `asc versions delete` (was `... to delete a version`)
- `asc versions release` (was `... to release a version`)
- `asc video-previews delete` (was `... to delete`)
- `asc web service-ids domains set` (printed only the usage page, with no `Error:` line)

Commands with `--dry-run` now print
`Error: --confirm is required unless --dry-run is set`, as the other commands
with `--dry-run` already did:

- `asc builds expire-all` (was `... to expire builds`)
- `asc profiles local clean` (was `--confirm is required`)
- `asc signing sync import` (was `... to write the destination store (or pass --dry-run to preview)`)
- `asc signing sync nuke` (was `... to delete profiles and revoke certificates (or pass --dry-run to preview)`)
- `asc versions phased-release guard` (was `... to pause or complete the phased release (or pass --dry-run to preview)`)
- `asc web subscriptions pricing monthly-commitment bootstrap` (was `--confirm is required`)

## `--confirm` after the other required flags (36 commands)

These commands checked `--confirm` before some of their required flags. They now
report the missing required flag first:

- `asc ads ad-groups delete`, `asc ads ads delete`, `asc ads assets delete`,
  `asc ads budget-orders delete`, `asc ads campaigns delete`,
  `asc ads creatives delete`, `asc ads location-groups delete`,
  `asc ads negative-keywords delete`, `asc ads targeting-keywords delete`
- `asc app-events submit`
- `asc apps search-keywords set` (`--keywords`)
- `asc bundle-ids capabilities reconcile apply`
- `asc iap promoted-purchases delete`, `asc subscriptions promoted-purchases delete`
- `asc localizations search-keywords delete` (`--keywords`)
- `asc pricing availability remove-from-sale`
- `asc product-pages custom-pages localizations preview-sets sync`,
  `asc product-pages custom-pages localizations screenshot-sets sync`,
  `asc product-pages experiments treatments localizations screenshot-sets sync`
- `asc product-pages custom-pages localizations search-keywords delete` (`--keywords`)
- `asc profiles local clean` (a clean mode such as `--expired`)
- `asc release stage`
- `asc review items remove`, `asc review items-remove`,
  `asc review submissions-cancel`, `asc review submissions-submit`
- `asc sandbox clear-history`
- `asc screenshots apply`
- `asc storekit auth logout`
- `asc storekit retention-messaging defaults delete`,
  `asc storekit retention-messaging images delete`,
  `asc storekit retention-messaging messages delete`
- `asc submit cancel`
- `asc users delete`, `asc users invites revoke`
- `asc webhooks delete`

`asc release stage` also checks `--platform`, `--timeout`, `--copy-fields` and
`--exclude-fields` before `--confirm` now, because those checks sit between its
required flags.

## Unchanged

- The Apple Ads acknowledgement (`--confirm is required to acknowledge potential
  Apple Ads spend, ...`) and the `asc auth token` warning keep their text.
- `asc distribute apply` keeps `--confirm PLAN_HASH is required`: its
  `--confirm` takes the plan hash as a value.
- Messages that say when `--confirm` becomes required (`--confirm is required
  with --submit`, `... when --allow-deletes is set`, and similar) keep their
  text.
- These commands still check `--confirm` before a required flag, because that
  flag is checked while reading a file, resolving credentials or prompting:
  `asc apps wall submit`, `asc migrate import`, `asc notarization staple`,
  `asc signing keychain delete`, `asc signing sync import` (`--repo`),
  `asc web review drafts create` and `asc web review drafts update` (`--message`
  or `--body-file`), and the StoreKit commands for `--environment`.

## If a script matched the old text

Match the start of the line instead: `Error: --confirm is required`. It is the
same in every destructive command. A script that relied on `--confirm` being
reported before a missing ID gets the ID error first; add the ID and run again.

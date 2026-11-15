# `--limit`, `--next` and `--paginate` have the same help in every list command

Changed in 7.0.0. List commands now describe their paging flags with the same
help lines, so `--help` on two sibling commands shows the same text for the
same flag:

```text
  --limit        Maximum results per page (1-200) (default: 0)
  --next         Fetch next page using a links.next URL
  --paginate     Automatically fetch all pages (aggregate results) (default: false)
```

The number in the `--limit` line is the command's own maximum, as before.
Only help text changed: the `--help` output and the usage page printed after a
usage error show the new lines. Every flag still accepts the same values, sends
the same requests and exits with the same codes.

## Changed help lines (59 lines in 48 commands)

`--limit` (2 commands):

- `asc performance diagnostics list` (was `Limit number of signatures (max 200)`)
- `asc storekit groups members` (was `Maximum members per page (1-100)`; now
  `Maximum results per page (1-100)`)

`--next` (21 commands):

- `asc app-events localizations placements list`, `asc localizations placements list`,
  `asc product-pages custom-pages localizations placements list`,
  `asc product-pages experiments treatments localizations placements list`
  (was `Fetch a links.next URL instead of selecting a localization`)
- `asc asset-library images list`, `asc asset-library images placements`,
  `asc asset-library videos list`, `asc asset-library videos placements`
  (was `Fetch a links.next URL instead of selecting a resource`)
- `asc performance diagnostics list` (was `Next page URL`)
- `asc pricing price-points`, `asc pricing territories list`,
  `asc review items list`, `asc review items-list`,
  `asc review submissions list`, `asc review submissions-items-ids`,
  `asc review submissions-list`, `asc testflight pre-release list`,
  `asc versions list` (was `Next page URL from a previous response`)
- `asc testflight distribution view`, `asc testflight review submissions list`,
  `asc testflight review view` (was `View next page using a links.next URL`)

`--paginate` (36 commands):

- `asc iap versions list`, `asc iap versions images list`,
  `asc iap versions localizations list`, `asc iap versions links images`,
  `asc iap versions links localizations`, `asc iap versions links versions`,
  and 18 `asc subscriptions` commands under `groups versions`,
  `retention-messages` and `versions` (was `Automatically fetch all pages`)
- the 4 placements lists and 4 asset-library lists above (was
  `Fetch all pages and aggregate the collection`)
- `asc performance diagnostics list`, `asc web app-groups list` (was
  `Fetch all pages`)
- `asc web apps distribution users list` (was `Fetch every recipient page`)
- `asc testflight review submissions list` (was
  `Automatically view all pages (aggregate results)`)

## Help lines that stay

These say something the shared lines do not and are unchanged:

- `--limit` that counts something other than results per page, or caps a
  command without pages: Game Center metrics (`Maximum groups per page`),
  `asc app-clips invocations localizations list`, `asc build-bundles list`,
  `asc ads apps search`, `asc ads change-history view`, `asc apps public search`,
  `asc apps wall`, `asc notarization list`, `asc optimize keywords discover`,
  `asc performance diagnostics view`, `asc performance download`, `asc search`,
  `asc web xcode-cloud workflows options schemes`.
- `--limit ... 0 uses server default` on the placements and asset-library
  lists, and the `With --set-id:` lines of `asc screenshots list`.
- `asc localizations download`: `--next` starts from a page and the command
  always continues to the last one; `--paginate` has no effect.
- `asc analytics view`, `asc age-rating audit` and `asc api` keep their own
  `--paginate` lines.

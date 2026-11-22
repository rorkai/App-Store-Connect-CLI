# iMessage Duo screenshot support

`asc screenshots sizes` and screenshot upload validation reject
`IMESSAGE_APP_IPHONE_DUO` before any network request. The shared screenshot
catalog already has `APP_IPHONE_DUO`. Add the iMessage slot beside the other
`IMESSAGE_APP_*` entries and reuse `iphoneDuoDimensions`.

## Verified contract

Apple's published `ScreenshotDisplayType` enum and the OpenAPI snapshot omit
both Duo values. The live API accepts `IMESSAGE_APP_IPHONE_DUO` as
`screenshotDisplayType` on `POST /v1/appScreenshotSets` (the same acceptance
check that established `APP_IPHONE_DUO`). Keep the snapshot unchanged and allow
the runtime value in the schema compatibility test.

Apple's screenshot specifications list one iPhone Duo size set:

- 1398×2034 and 2034×1398 (outer display)
- 2007×2853 and 2853×2007 (inner display)

Every other iMessage display type in the catalog uses the matching app
dimensions. Asset Library reference data serves those sizes from
`IMESSAGE_IPHONE_DUO_PROFILE` (`displayClassId` `IPHONE_DUO`, platform
`IMESSAGE_APP_STORE`).

## CLI behavior

- `asc screenshots sizes --display-type IMESSAGE_APP_IPHONE_DUO` lists the four
  sizes with family `IMESSAGE`. `asc screenshots sizes --all` includes the type.
- `asc screenshots upload --device-type IMESSAGE_APP_IPHONE_DUO` validates those
  sizes and uses the existing upload path. A standard iPhone size is rejected.
- Preview types are unchanged. `IPHONE_DUO` still maps to `APP_IPHONE_DUO`.

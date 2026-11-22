# iMessage Duo screenshots (7.2.0)

`asc screenshots sizes` and screenshot upload validation now include
`IMESSAGE_APP_IPHONE_DUO`. Accepted sizes match iPhone Duo: 1398×2034,
2034×1398, 2007×2853, and 2853×2007.

```bash
asc screenshots sizes --display-type "IMESSAGE_APP_IPHONE_DUO"
asc screenshots upload --version-localization "LOC_ID" \
  --path "./imessage-duo.png" \
  --device-type "IMESSAGE_APP_IPHONE_DUO"
```

# Game Center blocked players

API 4.5 adds a blocked-player collection for a Game Center detail and an endpoint
for updating a player's blocked status.

```bash
asc game-center details blocked-players list --detail-id "DETAIL_ID" --paginate
asc game-center details blocked-players update --id "PLAYER_ID" --blocked true --confirm
asc game-center details blocked-players update --id "PLAYER_ID" --blocked false --confirm
```

The list contains only blocked players. It is not a directory of every player.
Resolve the Game Center detail ID with `asc game-center details list --app
"APP_ID"`. Player IDs for a new block can come from a submitted score's `player`
relationship. Apple does not expose a standalone player lookup endpoint in this
API version.

Lists support `--fields nickname,blocked,bundleId`, `--limit` (1–200), `--next`,
and `--paginate`. A continuation URL cannot be combined with a detail ID, fields,
or limit because it already specifies the complete query. Each page gets a fresh
request timeout.

Updates require an explicit `--blocked true|false` and `--confirm`. An optional
`--bundle-id` sends Apple's `bundleId` update attribute; omitting it leaves that
attribute unchanged. An explicitly empty bundle ID is rejected. JSON retains
the resource envelope, including explicit false and empty nickname values.
Table and Markdown output show the ID, nickname, blocked status, and bundle ID.

The ID-only blocked-player relationship endpoint remains accessible with
`asc api`.

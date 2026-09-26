# Game Center score moderation

API 4.5 adds score moderation for v2 leaderboards. List the submitted scores and
include their player relationships:

```bash
asc game-center leaderboards v2 score-moderations list --leaderboard-id "LEADERBOARD_ID" --include player --paginate
```

The list supports `--limit` (1–200), `--next`, `--fields`, and `--player-fields`.
`--player-fields` requires `--include player`. Including a player automatically
retains the `player` relationship in an explicit score field selection.
`--exists-blocked true|false` forwards Apple's `exists[blocked]` filter.
A `--next` URL supplies its complete query, so it cannot be combined with the
leaderboard ID, filters, includes, fields, or limit.

Block or unblock a score by its moderation ID:

```bash
asc game-center leaderboards v2 score-moderations update --id "SCORE_ID" --blocked true --confirm
asc game-center leaderboards v2 score-moderations update --id "SCORE_ID" --blocked false --confirm
```

Both updates require `--confirm`. This changes an individual score's blocked
status. It does not block the player. JSON retains Apple's resource envelope,
player relationships, included resources, and numeric score strings without
converting them to floating-point numbers. Table and Markdown output summarize
ID, rank, score, blocked status, and submission date.

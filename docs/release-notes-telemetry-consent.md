# Telemetry asks before it identifies an installation

Changed in the next release. Telemetry stays on by default, but events no
longer carry a persistent identifier unless you opt in, and `asc` tells you
about telemetry before it sends anything.

- Events carry no install ID unless you run `asc telemetry enable --install-id`.
  Without opt-in, an event carries only a random event ID and a random
  per-process session ID, and no identifier is stored on disk.
  `asc telemetry enable --install-id=false` opts out again and deletes the ID.
- Before the first event, `asc` prints a one-time notice to stderr that says
  what is collected and how to turn it off. If the notice cannot be recorded in
  `~/.asc/telemetry.json`, nothing is sent.
- An install ID stored by an earlier version without opt-in is deleted the
  first time the notice is shown, and is never sent.
- Events move to schema version 8. `first_run` is `true` only on the first
  event from a new local installation, so new installations can be counted
  without an identifier; upgrades, CI, and Rork sandboxes always send `false`.
  The schema v7 fields `upload_phase` and `upload_size_bucket` are sent as
  `null`.
- Rork sandboxes and Rork GitHub workflows skip the notice; Rork's privacy
  policy covers those runtimes.
- `asc telemetry disable` now also deletes the install ID.
- `asc telemetry off` and `asc telemetry on` are aliases for `disable` and
  `enable`.
- New root flag `--no-telemetry` sends nothing for one invocation, alongside
  `DO_NOT_TRACK=1` and `ASC_TELEMETRY_DISABLED=1`.
- `asc telemetry reset-id` without opt-in deletes any stored ID and prints
  `No install ID is stored; telemetry events carry none`.
- `asc telemetry status --output json` adds `install_id_enabled`, and shows
  `install_id` only after opt-in.
- Docs: the 2.1.1 release notes and the read-only mode page called this
  telemetry "anonymous". With an install ID it is pseudonymous; the docs now
  say so, and a new Privacy page names who receives the data.

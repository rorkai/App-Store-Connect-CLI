# `asc builds upload --reserve-only`; `--dry-run` stops reserving in 7.4.0

Changed in 7.3.2. `asc builds upload --dry-run` has always created a build
upload in App Store Connect (`POST /v1/buildUploads` and
`POST /v1/buildUploadFiles`) and printed its upload operations without sending
the file. That is a write to your account, not a dry run.

7.3.2 adds `--reserve-only`, which does exactly what `--dry-run` did, with the
same output and exit codes:

```bash
asc builds upload --app "123456789" --ipa "app.ipa" --reserve-only
```

`--dry-run` still reserves the upload in 7.3.2, and prints this warning on
stderr before the first request:

```text
Warning: builds upload --dry-run reserves an upload in App Store Connect (POST /v1/buildUploads and /v1/buildUploadFiles). From 7.4.0, --dry-run sends nothing to App Store Connect and only checks the artifact locally; use --reserve-only to keep reserving the upload.
```

stdout and exit codes are unchanged. `--dry-run` and `--reserve-only` together
exit with usage code `2`, and `--reserve-only` rejects `--concurrency`,
`--checksum`, `--wait`, `--test-notes` and `--verify-timeout` the way
`--dry-run` does.

## Changed in 7.4.0: `--dry-run` sends nothing

`asc builds upload --dry-run` sends nothing to App Store Connect, not even the
app read. It checks the artifact, the IPA's bundle ID, version, build number and
platform, and that credentials are configured, then prints what it would
upload:

```json
{"dryRun":true,"app":"123456789","fileName":"app.ipa","fileSize":12345,"platform":"IOS","version":"1.0.0","buildNumber":"42","bundleId":"com.example.app"}
```

`app` is the `--app` value as given (a bundle ID or name is not looked up), and
`bundleId` is omitted for `--pkg`. The receipt no longer has `uploadId`,
`fileId` or `operations`, and the IPA's bundle ID is no longer compared with the
app's. One line on stderr says so:

```text
Dry run: checked app.ipa locally; nothing was sent to App Store Connect. The app's bundle ID was not checked; use --reserve-only to check it and reserve the upload.
```

Exit codes: `0` when the local checks pass, `2` for the same flag conflicts as
before, `3` when no credentials are configured. The 7.3.2 warning is gone.
`--reserve-only` is unchanged.

## Migration

- If a script uses `--dry-run` to get an upload ID or upload operations, switch
  to `--reserve-only` now. Its output is the same.
- If a script uses `--dry-run` as a preflight check, keep it. From 7.4.0 it no
  longer creates a build upload in your account. Add `--reserve-only` to a
  preflight that must also confirm the IPA belongs to the app.

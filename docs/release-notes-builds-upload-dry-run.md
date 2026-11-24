# Build upload reservation and dry-run transition

7.4.0 adds `--reserve-only` to create a build upload in App Store Connect
(`POST /v1/buildUploads` and `POST /v1/buildUploadFiles`) and print its upload
operations without sending the file. This is the behavior of `--dry-run`
through 7.4.0.

```bash
asc builds upload --app "123456789" --ipa "app.ipa" --reserve-only
```

`--dry-run` emits the following warning on stderr before the first request.

```text
Warning: builds upload --dry-run reserves an upload in App Store Connect (POST /v1/buildUploads and /v1/buildUploadFiles). In a future release, --dry-run will check the artifact locally without contacting App Store Connect. Use --reserve-only to keep reserving uploads.
```

stdout and exit codes are unchanged. `--dry-run` and `--reserve-only` together
exit with usage code `2`, and `--reserve-only` rejects `--concurrency`,
`--checksum`, `--wait`, `--test-notes` and `--verify-timeout` the way
`--dry-run` does.

## After 7.4.0: `--dry-run` sends nothing

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
before, `3` when no credentials are configured. The warning above is gone.
`--reserve-only` is unchanged.

## Migration

- If a script uses `--dry-run` to get an upload ID or upload operations, switch
  to `--reserve-only` now. Its output is the same.
- If a script uses `--dry-run` as a preflight check, keep it. It still creates
  an upload reservation in 7.4.0; once `--dry-run` sends nothing it no longer
  creates a build upload in your account. Add `--reserve-only` to a preflight
  that must also confirm the IPA belongs to the app.

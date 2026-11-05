# Signing identity sync release note

> Status: the legacy password sources described below were removed in 5.0.0.
> `--password` is no longer registered and `ASC_MATCH_PASSWORD` is no longer
> read; use `--password-file` or `ASC_SIGNING_SYNC_PASSWORD`. See
> `migrate-to-5-0.mdx`.

`asc signing sync push` can now pair a local PKCS#12 or RSA/EC private key with
the certificate embedded in the selected provisioning profile, then store one
canonical encrypted identity for other agents and CI workers. New identity
envelopes use scrypt plus authenticated metadata, while existing
certificate/profile repository behavior stays compatible.

Use `--password-file` or `ASC_SIGNING_SYNC_PASSWORD` for the signing repository
password. The older `--password` flag and `ASC_MATCH_PASSWORD` environment
variable remain available through 4.x with a deprecation warning and will be
rejected in 5.0.0.

## Profile lifecycle

`asc signing sync push --renew-expired` replaces an expired profile with a
same-name successor. `--force-for-new-devices` recreates a development or ad
hoc profile when its devices differ from the enabled device list, and
`--include-mac-in-profiles` adds enabled Apple silicon Macs to iOS development
and ad hoc device sets. `asc signing sync nuke --profile-type TYPE --confirm`
deletes every profile of one type, revokes its certificates, and removes their
encrypted artifacts; `--dry-run` previews the plan without changes. See
[Renew, refresh, and remove synced profiles](../website/src/content/docs/commands/signing.mdx#renew-refresh-and-remove-synced-profiles).

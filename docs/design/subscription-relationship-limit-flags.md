# Subscription relationship limit flags

## Decision

Subscription product and version reads use plural relationship names in both
App Store Connect JSON:API payloads and query parameters. Their canonical CLI
flags therefore use the same plural nouns:

- `asc subscriptions list|view --versions-limit`
- `asc subscriptions versions list|view --images-limit`
- `asc subscriptions versions list|view --localizations-limit`

The existing singular spellings remain accepted as hidden deprecated aliases.
Using one writes one migration warning to stderr. Supplying both spellings is a
usage error, even when their values match. Opaque `--next` URLs remain mutually
exclusive with relationship-limit flags; an alias is normalized first so the
error names the canonical plural flag.

## API contract

The affected operations are `GET /v1/subscriptionGroups/{id}/subscriptions`,
`GET /v1/subscriptions/{id}`, `GET /v1/subscriptions/{id}/versions`, and
`GET /v1/subscriptionVersions/{id}`. Their OpenAPI query keys remain
`limit[versions]`, `limit[images]`, and `limit[localizations]`. Responses and
stdout output are unchanged. Valid limits remain 1 through 50; invalid values
return a usage error before authentication or HTTP.

## Compatibility and migration

This is a deprecation, not a removal. Existing scripts continue to run while
receiving a direct replacement command. Generated command documentation remains
in sync, and live command help lists only the plural flags. Release notes are
generated from this pull request, whose title and body call out the deprecated
aliases and exact replacements.

The external ASC workflow skills do not currently use these limit flags. Their
4.4.1 synchronization pull request should still record the plural names as the
only recommended spellings and must not add singular examples.

## Verification

RED-GREEN coverage spans all four command leaves, valid and invalid canonical
values, hidden aliases, one-warning behavior, dual-spelling conflicts, and
opaque-next conflicts. Existing HTTP tests continue to assert the three exact
JSON:API query keys. Built-binary checks cover canonical help, alias migration,
conflicts, stdout/stderr separation, and exit code 2 for usage failures.

An alternative was to remove the singular flags immediately. That would break
stable scripts and violates the CLI lifecycle. Keeping the singular names as
the documented interface was also rejected because it disagrees with every
other 4.4.1 relationship-limit flag and with the API resource names.

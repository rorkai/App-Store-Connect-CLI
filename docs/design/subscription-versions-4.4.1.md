# Subscription versions (App Store Connect API 4.4.1)

## Placement and command shape

The new version-scoped subscription resources live under the existing
`asc subscriptions` taxonomy:

```text
asc subscriptions versions create --subscription-id SUBSCRIPTION_ID
asc subscriptions versions list --subscription-id SUBSCRIPTION_ID
asc subscriptions versions view --id VERSION_ID
asc subscriptions versions links --subscription-id SUBSCRIPTION_ID
asc subscriptions versions localizations list --version-id VERSION_ID
asc subscriptions versions localizations links --version-id VERSION_ID
asc subscriptions versions localizations view|create|update|delete ...
asc subscriptions versions images list --version-id VERSION_ID
asc subscriptions versions images primary --version-id VERSION_ID
asc subscriptions versions images links --version-id VERSION_ID
asc subscriptions versions images primary-link --version-id VERSION_ID
asc subscriptions versions images view|upload|update|delete ...
```

The existing `asc subscriptions localizations` and
`asc subscriptions images` commands remain unchanged. They accept a
subscription/product ID and call the v1 product-scoped endpoints. The new
commands require version IDs and call the 4.4.1 version-scoped endpoints; no
stable command silently changes ID meaning.

## API contract

The client covers all 18 new subscription operations:

- `POST /v1/subscriptionVersions` using `SubscriptionVersionCreateRequest`.
- Related and relationship reads for subscription versions, version
  localizations, the singular version image, and plural version images.
- `GET /v1/subscriptionVersions/{id}`.
- v2 subscription-localization create, get, update, and delete.
- v2 subscription-image create, get, update, and delete.

Version creation has no attributes and requires a `subscription` relationship.
Localization creation requires `name`, `locale`, and a `version` relationship;
description is optional. Localization updates support name and description,
but not locale. Image creation requires file name, file size, and a `version`
relationship. The image upload command reserves the resource, executes the
returned upload operations, and commits it with `uploaded: true`.

Version list accepts the documented state enum, sparse fields, includes,
relationship limits, `--next`, and `--paginate`. Related localizations/images
and linkage lists support their endpoint-specific limit and pagination bounds.
Detail commands expose only the include, sparse-field, and relationship-limit
parameters accepted by their exact endpoints.

API responses remain JSON:API resources. JSON output preserves compound
`included` resources. Table output renders the primary version, localization,
image, or linkage rows.

## Output, errors, and lifecycle

Data is written to stdout through the shared output renderer. Diagnostics and
required-flag messages use stderr. Required flags and invalid values return
usage exit code 2. Deletes require `--confirm`; there are no prompts. List
commands support `--paginate`; upload work uses the shared upload timeout.

The new version lifecycle does not remove or deprecate the existing stable v1
commands in this change. Review-submission item support is intentionally a
cross-cutting follow-up because another 4.4.1 slice owns the shared review item
types.

## Tests and verification

CLI tests establish the hierarchy, required flags, invalid limits/state, and
version-scoped request paths and payloads. HTTP tests assert all 18 methods,
paths, query parameters, request relationships, response decoding, pagination,
and API failures. Existing subscription GET tests assert the added `versions`
include, sparse fields, relationship limits, and included-resource preservation.

The built binary is checked for help, JSON stdout, empty stderr on success, and
exit 2 on missing flags. The full format, docs, lint, and test gate runs before
the PR opens. Live reads are attempted only when credentials and a disposable
subscription version are safely available; otherwise live behavior is reported
as unverified.

## Alternatives

Replacing the existing localization/image commands with v2 calls would be
shorter, but would silently reinterpret subscription IDs as version IDs and
break stable automation. A separate top-level `subscription-versions` command
would avoid that ambiguity but would fragment the existing subscription
taxonomy. Nesting under `subscriptions versions` makes the lifecycle boundary
explicit while keeping related functionality together.

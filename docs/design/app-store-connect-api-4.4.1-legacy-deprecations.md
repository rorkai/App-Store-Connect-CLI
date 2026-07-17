# App Store Connect API 4.4.1 legacy resource deprecations

## Placement and command shape

Apple deprecated the v1 image, localization, and submission resources for
in-app purchases, subscriptions, and subscription groups. Twenty-seven affected
CLI leaves are stable and two localization `sync` leaves are experimental. This
change starts a visible deprecation window for all of them; it does not remove
commands, flags, or client methods.

The affected command leaves remain at their current paths:

- `asc iap images {list,view,create,update,delete}`
- `asc iap localizations {list,create,update,delete}`
- `asc iap submit`
- `asc subscriptions images {list,view,create,update,delete}`
- `asc subscriptions localizations {list,view,create,update,delete,sync}`
- `asc subscriptions groups localizations {list,view,create,update,delete,sync}`
- `asc subscriptions review submit` and
  `asc subscriptions review submit-group`

Their direct help is marked `DEPRECATED` and points to the matching
version-scoped command or review-item workflow. Running one of the 29 public
command leaves writes one warning to stderr before preserving the existing
flags, request, stdout, and exit behavior. Twenty-seven leaves are stable; the
two localization `sync` leaves are explicitly experimental.

`asc iap setup` and `asc subscriptions setup` are not deprecated. They warn
only when localization flags request the legacy localization steps. A
subscription setup invocation that requests both subscription and group
localizations still emits one combined warning.

## API mapping

The old commands continue to call their existing v1 operations during the
deprecation window. Callers should migrate as follows:

- IAP images and localizations: create or resolve an
  `inAppPurchaseVersion`, then use `asc iap versions images ...` or
  `asc iap versions localizations ...`.
- Subscription images and localizations: create or resolve a
  `subscriptionVersion`, then use `asc subscriptions versions images ...` or
  `asc subscriptions versions localizations ...`.
- Subscription group localizations: create or resolve a
  `subscriptionGroupVersion`, then use
  `asc subscriptions groups versions localizations ...`.
- IAP submissions: add the version with `asc review items add --submission
  "SUBMISSION_ID" --item-type inAppPurchaseVersions --item-id
  "IAP_VERSION_ID"`.
- Subscription submissions: add the version with `asc review items add
  --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id
  "SUBSCRIPTION_VERSION_ID"`.
- Subscription group submissions: add the group version with `asc review items
  add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions
  --item-id "GROUP_VERSION_ID"`.

The 33 public Go client methods that target the deprecated resources are kept
and receive Go `Deprecated:` documentation pointing at their v2 or review-item
replacement. Private browser-session clients are out of scope.

## Streams, compatibility, and failure behavior

Warnings are diagnostics on stderr. Successful response data remains on stdout,
TTY-aware output selection is unchanged, destructive commands still require
`--confirm`, and command errors retain their current exit classification. The
warning does not imply that Apple has removed the endpoint; it announces the
CLI transition before a future removal release.

## Verification

The RED-GREEN matrix covers:

- every one of the 29 public leaf commands is labelled in direct help and has exactly
  one precise warning;
- representative commands from all seven command families retain their normal
  validation or HTTP behavior after warning;
- setup warns only when legacy localization flags are present;
- v2 version-scoped commands and `asc review items add` do not warn;
- generated command documentation no longer teaches the deprecated paths;
- formatting, documentation, lint, and the complete test suite pass.

No live mutation is needed for the warning wrapper itself. The underlying old
and new HTTP operations retain their existing command and HTTP tests; the API
4.4.1 rollout separately exercises version-scoped endpoints against the
disposable ASC app.

## Alternatives

Immediate deletion would match Apple's future direction but would break stable
automation without the repository's required deprecation window. Keeping the
commands visually unchanged would preserve compatibility but give callers no
migration signal. A narrow warning wrapper provides the required transition
while leaving endpoint behavior intact.

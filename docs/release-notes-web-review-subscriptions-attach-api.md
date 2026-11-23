# Subscription attach can use review submission items

Added in 7.3.0. `asc web review subscriptions attach` and `attach-group` take a
new `--review-items` flag. With it, they add the subscription's version to the
app's draft review submission through the App Store Connect API, the same
request as `asc review items add --item-type subscriptionVersions`, using the
API key instead of the Apple web session. It also works when the web session is
lent through a broker, where the web-session flow cannot send its request.

Without `--review-items` nothing changes: the commands use the web session
(`POST /iris/v1/subscriptionSubmissions`) with the same flags and JSON receipt.
That flow is not deprecated. `list`, `remove` and `asc validate` read Apple's
web listing, which does not show review items, so they keep matching it.

## `--review-items` behavior

- **Draft review submission:** with one `READY_FOR_REVIEW` draft, that draft is
  used. With several, pass `--submission SUBMISSION_ID` (a usage error lists
  them). With none, a draft is created for `--platform` (default `IOS`).
  An explicit `--platform` picks the draft on that platform; it is a usage
  error with `--submission`, when every draft is on another platform, or when
  the version is already on a draft for another platform. `--submission` and
  `--platform` are a usage error without `--review-items`, and the web-session
  flags are a usage error with it.
- **First subscription:** until the app has had a subscription approved
  (judged by subscription group version history and by states only a once
  approved subscription has), App Store Connect reviews its first subscription
  with an app version. The command then also adds the app's submittable App
  Store version on the draft's platform, and prefers the draft that already
  holds one. Apple checks that version's metadata when it is added, so it must
  be complete (description, keywords, support URL, copyright, category, review
  details, screenshots); otherwise Apple's 409 lists what is missing and
  nothing stays added. With no submittable app version it stops before any
  write and names `asc versions create`.
- **New subscription group:** whenever the subscription's group has never been
  approved, the group's submittable version is added too, also for apps that
  already have approved subscriptions in other groups.
- **Rollback:** if adding an item fails or times out, the items this run added
  are removed and a draft this run created is canceled, on a fresh timeout.
  App Store Connect usually refuses to cancel an unsubmitted draft; the draft
  is then kept, and the error names how to check it and reuse it. Nothing is
  submitted for review.
- **Idempotent:** items already on the draft are reported with `added: false`
  and not sent again. A version already on another draft is a usage error with
  `--submission`.
- **Not ready:** a `MISSING_METADATA` subscription, or one with no submittable
  version, is refused before any write (`state_not_ready`). `attach-group`
  lists such subscriptions as skipped.

## Submitting the draft

`asc review submit --version-id VERSION_ID` now reuses a draft that holds the
version plus subscription or subscription group versions (the draft
`--review-items` builds for a first subscription) instead of creating a second
one. Any other extra item still makes it skip the draft. A draft with only subscription items (no app
version), which `--review-items` builds when the app already has an approved
subscription, is still skipped; submit it with
`asc review submissions-submit --id SUBMISSION_ID --confirm`.

## `--review-items` receipt

A new shape. It reports only what the App Store Connect API returned:

- `reviewSubmissionId`, `reviewSubmissionCreated`, `changed`;
- `items[]`: `itemType` (`appStoreVersions`, `subscriptionGroupVersions`,
  `subscriptionVersions`), `resourceId`, `reviewSubmissionItemId`,
  `subscriptionId`, `added`;
- `subscription` (attach) or `changedSubscriptions`/`skippedSubscriptions`
  (attach-group) with `id`, `groupId`, `groupReferenceName`, `productId`,
  `name` and `state`. The web listing's attachment fields are not included.

The receipt has no `submissionId`; that key keeps its meaning on the default
path.

## Not changed

`asc web review subscriptions list`, `remove` and `remove-group` still use the
web session and Apple's web listing. To take an item `--review-items` added off
a draft, use `asc review items remove --id ITEM_ID --confirm`.

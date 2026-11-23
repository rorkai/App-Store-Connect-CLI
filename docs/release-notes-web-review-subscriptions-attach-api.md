# Subscription attach can use review submission items

Added in 7.3.0. `asc web review subscriptions attach` and `attach-group` take a
new `--review-items` flag. With it, they add the subscription's
version to the app's draft review submission through the App Store Connect
API, the same request as `asc review items add --item-type subscriptionVersions`,
using the API key instead of the Apple web session. It also works when the web
session is lent through a broker, where the web-session flow cannot send its
request.

## Deprecated: the web-session flow

Without `--review-items`, both commands work as before (web session,
`POST /iris/v1/subscriptionSubmissions`, same flags and JSON receipt) and now
print:

```text
Warning: `asc web review subscriptions attach` through the web session is deprecated; add --review-items to add the subscription to the app's review submission with the App Store Connect API key instead. The web-session flow may be removed in a future major release.
```

Migration: add `--review-items`, set up an API key (`asc auth login`) if you
only had a web session, and drop the web-session flags (`--apple-id`,
`--two-factor-code-command`, `--two-factor-method`, `--two-factor-phone-id`,
`--provider-id`, `--public-provider-id`), which are a usage error with
`--review-items`. Read the new receipt keys below instead of `submissionId`.
Making `--review-items` the default is left to a major release.

## `--review-items` behavior

- **Draft review submission:** with one `READY_FOR_REVIEW` draft, that draft is
  used. With several, pass `--submission SUBMISSION_ID` (a usage error lists
  them). With none, a draft is created for `--platform` (default `IOS`).
  An explicit `--platform` picks the draft on that platform; it is a usage
  error with `--submission`, or when every draft is on another platform.
  `--submission` and `--platform` are a usage error without `--review-items`.
- **First subscription:** until the app has an `APPROVED` subscription, App
  Store Connect reviews its first subscription with an app version. The
  command then also adds the app's submittable App Store version on the
  draft's platform, and, when the group was never approved, the group's
  submittable subscription group version. It prefers the draft that already
  holds an app version. With no submittable app version it stops before any
  write and names `asc versions create`.
- **Rollback:** if adding an item fails, the items this run added are removed
  and a draft this run created is canceled. Nothing is submitted for review.
- **Idempotent:** items already on the draft are reported with `added: false`
  and not sent again. A version already on another draft is a usage error with
  `--submission`.
- **Not ready:** a `MISSING_METADATA` subscription, or one with no submittable
  version, is refused before any write (`state_not_ready`). `attach-group`
  lists such subscriptions as skipped.

## `--review-items` receipt

A new shape. It reports only what the App Store Connect API returned:

- `reviewSubmissionId`, `reviewSubmissionCreated`, `changed`;
- `items[]`: `itemType` (`appStoreVersions`, `subscriptionGroupVersions`,
  `subscriptionVersions`), `resourceId`, `reviewSubmissionItemId`,
  `subscriptionId`, `added`;
- `subscription` (attach) or `changedSubscriptions`/`skippedSubscriptions`
  (attach-group), from the API; `submitWithNextAppStoreVersionKnown` is
  `false` because the web listing is not read.

The receipt has no `submissionId`; that key keeps its meaning (the web flow's
subscriptionSubmission ID) on the default path.

## Not changed

`asc web review subscriptions list`, `remove` and `remove-group` still use the
web session and Apple's web listing. To take an item `--review-items` added off
a draft, use `asc review items remove --id ITEM_ID --confirm`.

# Subscription attach uses the App Store Connect API

Changed in 7.3.0. `asc web review subscriptions attach` and `attach-group` now
add the subscription's version to the app's draft review submission through the
App Store Connect API, the same request as
`asc review items add --item-type subscriptionVersions`. They no longer send
`POST /iris/v1/subscriptionSubmissions` through the Apple web session, so they
also work when the web session is lent through a broker.

## Behavior changes

- **Credential:** both commands now need App Store Connect API key auth
  (`asc auth login`, or `ASC_KEY_ID`/`ASC_ISSUER_ID`/`ASC_PRIVATE_KEY_PATH`).
  A web session alone is no longer enough; without a key the command exits `3`
  and says it now uses the API key.
- **Draft review submission:** the version goes on a `READY_FOR_REVIEW` review
  submission. With one draft, that draft is used. With several, pass the new
  `--submission SUBMISSION_ID` (a usage error lists them). With none, a draft
  is created for the new `--platform` (default `IOS`); if adding the item then
  fails, that draft is canceled again. Nothing is submitted for review.
- `--platform` given explicitly picks the app's draft on that platform; it is a
  usage error together with `--submission`, or when no draft is on that
  platform while other drafts exist.
- A version already on a draft is reported with `changed: false` and nothing is
  sent. With `--submission` naming another draft, it is a usage error.
- A `MISSING_METADATA` subscription, or one with no version that can be
  submitted, is refused before any write (`state_not_ready`), with the next
  command to run. `attach-group` lists such subscriptions as skipped.

## JSON output

Every field is kept. Values that changed meaning:

- `submissionId` is the review submission ID (it was the hidden
  subscriptionSubmission ID).
- `subscription.submitWithNextAppStoreVersion` is `true` when the version is on
  a draft review submission. `isAppStoreReviewInProgress` is always `false`
  here; read the version state with `asc subscriptions versions list`.

Added: `reviewSubmissionItemId` and `subscriptionVersionId` on `attach`, and
`submissionId` on `attach-group`.

## Deprecated flags

On these two commands, `--apple-id`, `--two-factor-code-command`,
`--two-factor-method`, `--two-factor-phone-id`, `--provider-id` and
`--public-provider-id` are deprecated and have no effect. Setting one prints:

```text
Warning: --apple-id is deprecated for `asc web review subscriptions attach` and has no effect: the command uses the App Store Connect API key, not a web session. Drop the flag; it will be removed in a future release.
```

Migration: drop the flag. The flags are removed in a later major release.

## Not changed

`asc web review subscriptions list`, `remove` and `remove-group` still use the
web session and Apple's web listing. To take a subscription off a draft that
`attach` added it to, use `asc review items remove --id ITEM_ID --confirm`.

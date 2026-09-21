# Multi-item review submissions

Read this reference when one review submission must contain the app version plus versioned digital goods, Game Center components, or both.

Follow only the section for the current phase:

- During readiness, complete **Prepare every item**, then return to the main release flow.
- Enter **Assemble the submission** only after the app version is staged and the multi-item lane is selected.
- Enter **Submit** only after inspecting the assembled draft and obtaining confirmation.

## Prepare every item

- Resolve the exact `VERSION_ID` and each product or component version ID.
- Use `asc-submission-health` to repair missing IAP, subscription, or subscription-group versions.
- Confirm the Game Center app version exists:

```bash
asc game-center app-versions list --app "APP_ID" --output table
asc game-center app-versions create --app-store-version-id "VERSION_ID" --output json
```

Create only when the intended Game Center app version is absent.

## Assemble the submission

Do not create a review submission from the readiness gate. Start this phase only after the app version and build are staged and the user has selected the multi-item lane.

Inspect existing submissions before creating one:

```bash
asc review submissions list \
  --app "APP_ID" \
  --platform IOS \
  --include "items,appStoreVersionForReview" \
  --paginate \
  --output json
```

Branch before writing:

- If `asc-submission-health` or the caller handed off a `SUBMISSION_ID`, inspect it with `asc review submissions-get --id "SUBMISSION_ID" --include "items,appStoreVersionForReview" --output json`. Reuse it only when it is the intended `READY_FOR_REVIEW` draft. If it fails that check, stop and return the mismatch to `asc-submission-health` for diagnosis; do not create another submission or add items.
- With no handed-off ID, if exactly one `READY_FOR_REVIEW` draft matches the intended release, capture and reuse its ID.
- With no handed-off ID, if no matching draft or active submission exists for the intended version or review items, create one and capture its ID:

```bash
asc review submissions-create --app "APP_ID" --platform IOS --output json
```

- Otherwise—when drafts are ambiguous or the intended version or review items belong to another active submission—stop and diagnose through `asc-submission-health`. Do not create a second submission.

Use the reused or newly created ID as `SUBMISSION_ID` below. If neither branch assigned one, do not continue.

List the draft's current items before attaching anything:

```bash
asc review items list --submission "SUBMISSION_ID" --paginate --output json
```

Compare each intended item type and resource ID with the returned relationships. Skip exact matches already attached to this draft. Add only missing items, with the app version first, followed by the resolved product and Game Center version items:

```bash
asc review items add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"
asc review items add --submission "SUBMISSION_ID" --item-type inAppPurchaseVersions --item-id "IAP_VERSION_ID"
asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"
asc review items add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions --item-id "GROUP_VERSION_ID"
asc review items add --submission "SUBMISSION_ID" --item-type gameCenterLeaderboardVersions --item-id "GC_LEADERBOARD_VERSION_ID"
```

Supported Game Center item types also include:

- `gameCenterAchievementVersions`
- `gameCenterActivityVersions`
- `gameCenterChallengeVersions`
- `gameCenterLeaderboardSetVersions`

Add only types required for this release. Do not submit placeholder or unresolved IDs.

## Submit

Inspect the assembled submission before the final mutation, then require confirmation:

```bash
asc review submissions-get --id "SUBMISSION_ID" --include items --output table
asc review items list --submission "SUBMISSION_ID" --paginate --output table
asc review submissions-submit --id "SUBMISSION_ID" --confirm
```

If inspection shows an existing active or completed submission instead of the draft being assembled, stop and diagnose through `asc-submission-health` rather than creating another submission.

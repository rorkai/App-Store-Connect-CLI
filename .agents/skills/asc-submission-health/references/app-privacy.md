# App Privacy publish state

Read this reference only when readiness validation reports an App Privacy advisory or the public API cannot confirm the published state.

The public App Store Connect API does not cover the full App Privacy workflow. Treat the following commands as authenticated web-session automation, not public-API proof.

## Inspect and plan

Pull the current answers into a reviewable file:

```bash
asc web privacy pull --app "APP_ID" --out "./privacy.json"
```

Review the planned change before writing:

```bash
asc web privacy plan --app "APP_ID" --file "./privacy.json"
```

Check every data type, purpose, tracking answer, and linkage answer against the app's real behavior and third-party SDKs. Do not infer privacy answers from the binary name or store copy.

## Apply and publish

Apply the reviewed file:

```bash
asc web privacy apply --app "APP_ID" --file "./privacy.json"
```

Publish only after the applied answers have been checked:

```bash
asc web privacy publish --app "APP_ID" --confirm
```

Keep the plan and apply steps separate. A successful apply does not prove that the answers were published.

## Manual fallback

If the user declines web-session automation, open the app's App Privacy page and confirm the published state manually:

```text
https://appstoreconnect.apple.com/apps/APP_ID/appPrivacy
```

Report the boundary plainly: the public API readiness report can raise an advisory, while final publish-state verification requires the web flow or a manual check.

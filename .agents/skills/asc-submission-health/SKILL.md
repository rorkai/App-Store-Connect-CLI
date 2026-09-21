---
name: asc-submission-health
description: Diagnose App Store submission blockers and operate review health with asc, including readiness validation, repair routing, status monitoring, cancellation, and retry decisions. Use when validation fails, a version is not in a valid state, review status is unclear or stuck, or a failed submission must be repaired and retried. For staging, upload, publication, and submission execution, use asc-release-flow.
---

# App Store submission health

Use this skill to explain why a release cannot proceed and to manage an existing review submission. Hand healthy release execution back to `asc-release-flow`.

## Ownership boundary

This skill owns:

- readiness validation and blocker diagnosis;
- public-API, web-session, and manual repair routing;
- review status and history;
- cancellation and retry decisions.

Use `asc-release-flow` for staging, upload, publication, and submission. Switching skills continues the current task with its resolved targets, authorization, and verified progress; it does not require the user to restart the workflow. Continue authorized repairs and return to release execution when healthy. Ask for new authority only when a repair exceeds scope.

## Answer order

1. State whether the version is ready, blocked, or already under review.
2. Name each blocker and the evidence that proves it.
3. Separate public-API repairs from web-session and manual work.
4. Run the read-only checks needed to establish the diagnosis. For diagnosis-only requests, report the evidence and one proposed repair command without executing that repair. For authorized execution, continue the repair queue and report completed work and remaining blockers.

## Establish the target

- Resolve `APP_ID`, the version string or `VERSION_ID`, `BUILD_ID`, platform, and any known `SUBMISSION_ID`.
- Configure auth with `asc auth login` or `ASC_*` environment variables.
- Use `ASC_BYPASS_KEYCHAIN=1` only for repository tests and isolated verification, not normal user sessions.
- Prefer IDs once the target is resolved; stop when app, version, or product resolution is ambiguous.

## Diagnose readiness

Run the canonical readiness report first:

```bash
asc validate --app "APP_ID" --version "1.2.3" --platform IOS --output table
```

Use `--version-id "VERSION_ID"` when known. Add `--strict` when warnings must fail automation.

Ask the review-specific doctor for an ordered explanation:

```bash
asc review doctor --app "APP_ID" --version "1.2.3" --platform IOS --output table
```

Collect direct evidence when the report points at the build or version:

```bash
asc builds info --build-id "BUILD_ID" --output table
asc versions view --version-id "VERSION_ID" --include-build --include-submission --output table
```

For digital goods, run only the relevant product validator:

```bash
asc validate iap --app "APP_ID" --output table
asc validate subscriptions --app "APP_ID" --output table
```

Treat the ordered remediation plan from `asc validate` as the repair queue. Fix and verify one class of blocker before moving to the next.

## Route repairs

Use public API commands when the blocker is build processing, metadata, screenshots, review details, encryption, content rights, age rating, availability, or version-scoped product metadata.

Read [references/readiness-repairs.md](references/readiness-repairs.md) when diagnostics identify one of those common blockers or a first-release availability gap.

Read [references/digital-goods.md](references/digital-goods.md) only when IAP or subscription validation fails, Apple requires first-review attachment, or a versioned product must join an existing review submission.

Read [references/app-privacy.md](references/app-privacy.md) only when validation reports an App Privacy advisory or the publish state cannot be confirmed through the public API.

When validation reports a Game Center component or version blocker, hand it to `asc-release-flow` and request the multi-item reference's **Prepare every item** section. Do not route Game Center through general readiness or digital-goods repairs.

Use the web-session commands only for a gap the public API cannot cover, and say that an authenticated Apple web session is required. Keep a manual App Store Connect fallback when the user declines web-session automation.

## Decide whether the version is healthy

A version is ready to return to `asc-release-flow` when:

- `asc validate` has no blocking issues;
- the attached build is `VALID`;
- metadata, screenshots, app info, review details, content rights, encryption, age rating, pricing, and availability are resolved;
- the relevant `asc validate iap` and/or `asc validate subscriptions` checks have no blocking issues, and the required digital-goods versions are prepared;
- any Game Center version items have been checked through `asc-release-flow`'s multi-item submission reference;
- App Privacy is confirmed or published.

Do not call a version ready merely because one validator exits successfully. Report any warning that still needs a web-session or manual check.

## Monitor review

Use app-scoped status when the submission ID is unknown:

```bash
asc review status --app "APP_ID" --version "1.2.3" --platform IOS --output table
```

Use exact submission or version IDs when available:

```bash
asc submit status --id "SUBMISSION_ID" --output table
asc submit status --version-id "VERSION_ID" --output table
```

Use the release dashboard for surrounding build and review signals:

```bash
asc status --app "APP_ID" --include builds,appstore,submission,review --output table
```

Use history to distinguish a current stall from earlier rejected or completed submissions:

```bash
asc review history --app "APP_ID" --version "1.2.3" --paginate --output table
```

## Cancel an unhealthy submission

Resolve the exact active submission before cancelling. Preview status first, then require confirmation:

```bash
asc submit status --id "SUBMISSION_ID" --output table
asc submit cancel --id "SUBMISSION_ID" --confirm
```

When resolving by version, include the app for the modern review-submission lookup:

```bash
asc submit status --version-id "VERSION_ID" --output table
asc submit cancel --version-id "VERSION_ID" --app "APP_ID" --confirm
```

The lower-level equivalent is valid when the exact review submission is already known:

```bash
asc review submissions-cancel --id "SUBMISSION_ID" --confirm
```

Do not cancel a submission solely because review is taking longer than expected. Confirm the state and the user's intent first.

## Decide when to retry

There is no dedicated retry command. Use this sequence:

1. Cancel only if the active submission must be withdrawn.
2. Repair the proven blockers.
3. Re-run `asc validate` and the relevant product validators.
4. Confirm no active submission already owns the version or review items.
5. Hand the healthy version and any preserved `SUBMISSION_ID` to `asc-release-flow` for submission execution. Reuse an inspected `READY_FOR_REVIEW` draft; create a submission only when no matching draft or active submission exists.

## Common failure routing

| Symptom | First evidence | Repair route |
| --- | --- | --- |
| Version is not in a valid state | `asc validate`, `asc review doctor` | ordered readiness repairs |
| Export compliance must be approved | build info and encryption declaration | readiness repairs |
| Multiple app infos found | `asc apps info list --app "APP_ID"` | resolve exact app-info ID |
| IAP or subscription is not ready | product validator | digital-goods reference |
| Game Center component or version is not ready | `asc validate` diagnostic | `asc-release-flow` multi-item preparation section |
| App Privacy publish state is unclear | validation advisory | App Privacy reference |
| Review appears stuck | review status plus history | monitor; cancel only with evidence and approval |

## Guardrails

- Do not use removed `submit-preflight` or `submit-create` shortcuts.
- Do not submit from this skill; return healthy execution to `asc-release-flow`.
- Do not treat web-session automation as public App Store Connect API coverage.
- Do not retry until the earlier submission state and blocker repairs are verified.
- Use `--output table` for human diagnosis and JSON for automation.
- For macOS, use `--platform MAC_OS` while keeping the same health lifecycle.

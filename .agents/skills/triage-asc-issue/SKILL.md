---
name: triage-asc-issue
description: Triage App-Store-Connect-CLI GitHub issues against current code, CLI behavior, and App Store Connect API support. Use when the user asks to audit, reproduce, classify, label, scope, prioritize, or decide whether an ASC CLI issue should be fixed or implemented.
---

# Triage an ASC CLI issue

Produce a current, evidence-backed verdict and leave the issue with complete repository labels.

## Read current evidence

1. Fetch the live issue body, comments, labels, linked PRs, and referenced documentation.
2. Resolve the affected command through current `--help`, source, tests, and recent history. Do not infer support from an old command name.
3. Reproduce reported behavior with the current built or installed CLI when safe.
4. For API claims, verify the exact method and endpoint in `docs/openapi/latest.json`; use the `sosumi.ai` documentation mirror for explanatory context.
5. Check whether the report is already fixed on current `origin/main`, duplicated, unsupported by the public API, or blocked by Apple/platform behavior.

## Classify the outcome

Choose one primary verdict:

- Confirmed bug with a reproducible expected-versus-actual mismatch.
- Valid enhancement with a supported and coherent API or client-side design.
- Question requiring clarification or support guidance.
- Already fixed, duplicate, not reproducible, unsupported, or platform-limited.

Separate urgency from implementation size. Explain user impact, blast radius, workaround, compatibility risk, and the smallest proof needed for completion.

## Apply labels

Follow `CONTRIBUTING.md` and leave exactly one label from each bucket:

- Type: `bug`, `enhancement`, or `question`.
- Priority: `p0`, `p1`, `p2`, or `p3`.
- Difficulty: `easy`, `medium`, or `hard`.

Remove conflicting labels before adding replacements. If evidence is ambiguous, choose the lower priority or difficulty and state the assumption.

## Define implementation readiness

When the issue is actionable, provide:

1. Proposed command and UX shape.
2. API endpoint or explicit client-side behavior.
3. Files and shared surfaces likely affected.
4. RED-GREEN test plan and black-box exit/output cases.
5. Safe live verification and cleanup plan.
6. Compatibility or deprecation requirements.

If the user asks to implement the issue, hand the validated contract to `$develop-asc-change` in an isolated branch or worktree. Do not close the issue or claim completion before the implementation is merged and verified.

## Automation contract

A recurring issue-triage automation may classify new or incompletely labeled issues and report actionable findings. It must not implement code, close issues, or make speculative high-priority labels without explicit authorization.

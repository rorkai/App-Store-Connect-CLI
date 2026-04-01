# ASC Studio Coverage And IA Handoff

## Status Audit (March 30, 2026)

This section checks the original IA and product decisions against the current codebase.

Short answer: no, we have not done everything we decided.

The current branch has completed the shell, scope split, and a meaningful first slice of app surfaces, but a large part of the intended depth is still partial or missing.

### What Is Fully Done

- `ASC Studio` exists as a separate app in the same repo under `apps/studio`.
- The Wails shell is in place with translucent window/webview setup in `apps/studio/main.go`.
- The custom header now includes top-level scope tabs:
  - `App`
  - `Team`
  - `Signing`
  - `Automation`
- The sidebar is no longer a single flat list. It is grouped by scope and section.
- `Performance` remains inside the `App` scope rather than becoming a top-level scope.
- `Game Center` remains an app-platform surface inside the `App` sidebar.
- Team/admin and signing are no longer buried in the app-only sidebar. They now have dedicated top-level scopes.

### What Is Partially Done

#### App Scope

The `App` scope is the most developed area and now contains grouped sections for:

- `Overview`
- `Release`
- `Metadata`
- `Growth`
- `Monetization`
- `Insights`
- `Compliance`
- `Platform`

However, several of these sections are still bootstrap-level and not yet full Studio-native workspaces.

#### Release

The following now exist in the UI:

- `Builds`
- `TestFlight`
- `Submit`
- `Validate`
- `Release Notes`

But this is still incomplete because:

- `Publish` is still missing from the sidebar and the workspace.
- `Release` is still missing as its own surface.
- `Submit`, `Validate`, and `Release Notes` are still mostly command-backed/read-only rather than polished, approval-driven release workflows.

#### Metadata

The following now exist in the UI:

- `Localizations`
- `Screenshots`
- `Video Previews`
- `Categories`

This means the report's earlier recommendation to split metadata out of `overview` has started happening.

But this is still incomplete because:

- `Metadata` is not a first-class dedicated page yet.
- `App Setup` is missing.
- `App Tags` is missing.
- `Pre-orders` is missing.
- `Background Assets` is missing.

#### Team Scope

The `Team` scope now exists and includes:

- `Account`
- `Users`
- `Devices`

This is directionally correct, but still incomplete because:

- `Actors` is still missing.
- These surfaces are still mostly command-backed rather than fully designed management pages.

#### Signing Scope

The `Signing` scope now exists and includes:

- `Bundle IDs`
- `Certificates`
- `Profiles`

This is a strong start, but still incomplete because:

- `Signing Overview` is missing.
- `Merchant IDs` is missing.
- `Pass Type IDs` is missing.
- `Notarization` is missing.

#### Automation Scope

The `Automation` scope now exists and includes:

- `Xcode Cloud`
- `Webhooks`

This matches the intended top-level placement, but the scope is still very incomplete because:

- `Workflow` is missing.
- `Notifications` is missing.
- `Migrate` is missing.
- `Diff` is missing.
- `Schema` is missing.
- `ACP Sessions` does not exist as a dedicated page yet.

#### ACP / Chat

The ACP chat dock exists and the backend thread/session plumbing exists.

But it is still not complete:

- the frontend still contains the bootstrap fallback text saying live ACP transport is not wired yet
- the chat experience is still scaffolded rather than a fully integrated agent UX

### What Is Still Not Done

The following earlier decisions are still unimplemented in the product:

- full release execution coverage
- full metadata/store-admin coverage
- full platform/distribution coverage
- full team/admin coverage
- full signing coverage
- full automation coverage
- native-feeling ACP session management in the main product experience

### Practical Conclusion

The current app has completed the structural reframe:

- same-repo separate product
- Wails desktop shell
- header scope tabs
- grouped scope-aware navigation

But it has not completed the product-surface plan.

The most accurate summary is:

- shell and IA direction: mostly done
- page coverage: partial
- workflow depth: partial
- ACP integration: partial

## Purpose

This report is a handoff document for continuing `ASC Studio` as the desktop-first home of `asc`.

It answers four things:

1. What `ASC Studio` currently covers.
2. What is still missing relative to the current `asc` command surface.
3. How the information architecture should evolve, especially for team admin, signing, automation, and app-platform surfaces.
4. What the most native-feeling Wails/macOS implementation should be for the header, tabs, and top-level workspace split.

This report is based on the current Studio frontend in `apps/studio/frontend/src/App.tsx`, the current Wails shell in `apps/studio/main.go`, and the current root CLI registry in `internal/cli/registry/registry.go`.

## Current Reality

`asc` currently exposes about 75 root command families from `asc --help`.

`ASC Studio` currently exposes 21 app-facing sidebar sections in `apps/studio/frontend/src/App.tsx`:

- `overview`
- `status`
- `builds`
- `testflight`
- `history`
- `app-review`
- `app-privacy`
- `app-accessibility`
- `ratings-reviews`
- `in-app-events`
- `custom-product-pages`
- `ppo`
- `promo-codes`
- `game-center`
- `pricing`
- `iap`
- `subscriptions`
- `performance`
- `insights`
- `finance`
- `nominations`

The Studio app is therefore still a focused release-and-distribution client, not a broad first-party surface for the full CLI.

## Current Coverage Assessment

### Covered Or Mostly Covered

These command families already have a clear Studio home:

- `status`
- `builds`
- `testflight`
- `versions`
- `review`
- `reviews`
- `performance`
- `insights`
- `finance`
- `pricing`
- `iap`
- `subscriptions`
- `app-events`
- `product-pages`
- `nominations`
- `game-center`
- `accessibility`

### Partial Or Misplaced

These are represented in Studio, but not yet as clean first-class surfaces:

- `apps`
  - App selection exists, but there is no real app-management workspace.
- `metadata`
  - Metadata is implicitly scattered through `overview` and detail panels instead of being its own deliberate workspace.
- `localizations`
  - Some localization data is surfaced through the overview/detail flow, but not as a real section.
- `screenshots`
  - Screenshots are partially visible, but not organized as a dedicated asset workflow.
- `pricing`
  - Pricing has a real page, but availability scheduling and deeper pricing operations still feel secondary.
- `subscriptions`
  - Present, but still mostly a command-backed view, not a Studio-native management surface.
- `age-rating`
  - The current `App Privacy` mapping is misleading. In the current code, `app-privacy` is backed by `age-rating view`.
- `overview`
  - This is carrying too much responsibility and should be decomposed.

### Missing From Studio

These command families do not currently have a real Studio home:

#### Release Execution

- `publish`
- `submit`
- `validate`
- `release`
- `release-notes`

#### Metadata And Asset Operations

- `metadata`
- `localizations`
- `screenshots`
- `video-previews`
- `background-assets`
- `categories`
- `pre-orders`

#### App Setup / Store Admin

- `app-setup`
- `app-tags`
- `encryption`
- `eula`
- `agreements`

#### App Platform / Distribution Surface

- `app-clips`
- `android-ios-mapping`
- `marketplace`
- `alternative-distribution`
- `routing-coverage`

#### Build / TestFlight Adjacent

- `build-bundles`
- `build-localizations`
- `sandbox`
- `xcode`

#### Team / Access

- `account`
- `users`
- `actors`
- `devices`

#### Signing

- `signing`
- `bundle-ids`
- `certificates`
- `profiles`
- `merchant-ids`
- `pass-type-ids`
- `notarization`

#### Automation / Workflow

- `workflow`
- `webhooks`
- `xcode-cloud`
- `notify`
- `migrate`
- `diff`

#### Diagnostics / Power Tools

- `analytics`
- `schema`
- `crashes`
- `feedback`

#### Utility / Onboarding

- `auth`
- `doctor`
- `docs`
- `init`
- `install-skills`
- `snitch`
- `completion`
- `version`

## What Should Be Considered "Still Left"

If the goal is a polished v1.5 or v2 `ASC Studio`, the real unfinished buckets are:

1. Full release execution.
2. Full metadata and asset management.
3. Team and access administration.
4. Signing and provisioning.
5. Automation and workflow orchestration.
6. App-platform-specific distribution/admin surfaces.
7. Better app management and overview decomposition.

That is the real remaining product surface.

## Recommended Information Architecture

The current sidebar is too app-centric to absorb every remaining root command family cleanly. The right solution is not "add more sidebar items forever." The right solution is a two-level information architecture:

1. Header-level scope tabs.
2. Sidebar navigation within the selected scope.

## Recommended Top-Level Scopes

Use four top-level scopes in the header:

- `App`
- `Team`
- `Signing`
- `Automation`

This is the cleanest native-feeling split for the remaining command families.

### Why This Split Works

`App` is scoped to the selected app and should contain all app-specific product, release, metadata, monetization, insights, and app-platform surfaces.

`Team` is account-scoped and should contain users, devices, actors, account visibility, and operational access surfaces.

`Signing` is operational but not app-content-centric. It is its own professional workspace in Apple development, and it deserves a dedicated top-level scope.

`Automation` is where workflows, notifications, webhooks, migrations, diffs, and agent/ACP-related flows belong.

This prevents the app sidebar from becoming a giant drawer of unrelated concepts.

## Most Native Wails/macOS Header Strategy

### Short Answer

Yes, the app can have a native-feeling header tab experience in Wails.

### Important Nuance

Wails currently gives you native macOS title bar customization such as:

- transparent title bar
- hidden title
- full-size content
- toolbar usage
- hidden toolbar separator

Those are already enabled in `apps/studio/main.go`:

- `TitlebarAppearsTransparent: true`
- `HideTitle: true`
- `FullSizeContent: true`
- `UseToolbar: true`
- `HideToolbarSeparator: true`

Wails documents these options in the official options reference:

- [Wails Options](https://wails.io/docs/reference/options)

### Recommendation

Do not chase a true Safari/Xcode-style native macOS document-tab strip unless there is explicit runtime support for it.

Instead:

- keep the current Wails transparent toolbar/titlebar setup
- place a compact segmented control or tab-strip inside the custom header content area
- make it feel native through spacing, typography, materials, and placement

This is the most realistic and maintainable "native-feeling" solution in the current Wails setup.

### Concrete Recommendation For Header Tabs

Put a compact segmented control in the top header with:

- `App`
- `Team`
- `Signing`
- `Automation`

It should sit in the titlebar/toolbar region, not lower down as ordinary page tabs.

The selected app name and status should remain visible to the left or center of that same header zone, depending on visual balance.

## Where Each Remaining Area Should Go

### 1. App Scope

This should remain the default scope and keep the current app picker.

#### App Sidebar Groups

Recommended sidebar groups under `App`:

- `Overview`
- `Release`
- `Metadata`
- `Growth`
- `Monetization`
- `Insights`
- `Platform`
- `Compliance`

#### App / Overview Group

- `Overview`
- `Status`
- `History`

#### App / Release Group

- `Builds`
- `TestFlight`
- `Submit`
- `Validate`
- `Publish`
- `Release Notes`

#### App / Metadata Group

This is where the user’s intuition is correct: app-setup and store admin-ish metadata should largely live here.

Recommended:

- `App Information`
- `Metadata`
- `Localizations`
- `Screenshots`
- `Video Previews`
- `Background Assets`
- `Categories`
- `Pre-orders`
- `App Tags`
- `App Setup`

This group should absorb:

- `app-setup`
- `app-tags`
- `metadata`
- `localizations`
- `screenshots`
- `video-previews`
- `background-assets`
- `categories`
- `pre-orders`

#### App / Growth Group

- `Ratings & Reviews`
- `In-App Events`
- `Custom Product Pages`
- `Product Page Optimization`
- `Promo Codes`
- `Nominations`

#### App / Monetization Group

- `Pricing & Availability`
- `In-App Purchases`
- `Subscriptions`

#### App / Insights Group

- `Insights`
- `Performance`
- `Analytics`
- `Finance`
- `Crashes`

Recommendation: `Performance` should stay here, not become a top-level header scope. It is still app-scoped, and it belongs near insights/finance rather than beside team or signing.

#### App / Platform Group

This is where the user’s instinct is also correct: the app-platform surfaces belong in the sidebar itself, not in a separate top-level tab.

Recommended:

- `App Clips`
- `Game Center`
- `Android ↔ iOS Mapping`
- `Marketplace`
- `Alternative Distribution`
- `Routing Coverage`

#### App / Compliance Group

- `App Review`
- `Age Rating`
- `Accessibility`
- `Encryption`
- `EULA`
- `Agreements`

Current note: `App Privacy` should either become a real privacy surface if you implement one, or be renamed because it is currently backed by `age-rating`.

### 2. Team Scope

This should be account-scoped and not require the selected app to drive everything.

Recommended sidebar groups under `Team`:

- `Account`
- `People`
- `Devices`

Recommended pages:

- `Account`
- `Users`
- `Actors`
- `Devices`

This is the cleanest home for:

- `account`
- `users`
- `actors`
- `devices`

### 3. Signing Scope

This should be its own top-level workspace.

Recommended sidebar groups under `Signing`:

- `Identifiers`
- `Certificates`
- `Profiles`
- `Commerce`
- `Distribution`

Recommended pages:

- `Bundle IDs`
- `Certificates`
- `Profiles`
- `Signing Overview`
- `Merchant IDs`
- `Pass Type IDs`
- `Notarization`

This is the cleanest home for:

- `signing`
- `bundle-ids`
- `certificates`
- `profiles`
- `merchant-ids`
- `pass-type-ids`
- `notarization`

### 4. Automation Scope

This should be the home for workflows, integrations, and developer tooling around `asc`.

Recommended sidebar groups under `Automation`:

- `Workflows`
- `Integrations`
- `Agent Tools`
- `Utilities`

Recommended pages:

- `Workflow`
- `Webhooks`
- `Notifications`
- `Xcode Cloud`
- `Migrate`
- `Diff`
- `Schema`
- `ACP Sessions`

Recommended handling for `xcode`:

- keep `xcode` out of the primary user-facing IA for now
- do not make it a first-class scope
- if included at all, hide it behind an advanced utility page or command palette

That matches the user preference to exclude it from the main automation/admin story and keeps the app from feeling too much like an IDE clone.

## Specific Placement Answers

### Team Admin

Most native placement:

- top-level header tab: `Team`
- left sidebar within `Team`

Do not bury team admin inside the app-specific sidebar.

### Signing

Most native placement:

- top-level header tab: `Signing`
- left sidebar within `Signing`

Do not mix signing screens with app metadata or release workflows.

### Automation

Most native placement:

- top-level header tab: `Automation`
- left sidebar within `Automation`

Automation is broader than "app actions." It deserves its own operational scope.

### App Performance

Most native placement:

- keep it inside `App`
- under an `Insights` group

It should not be elevated to a header-level top scope.

### App Platform Surfaces

Most native placement:

- inside the `App` sidebar
- grouped under `Platform`

This includes:

- `App Clips`
- `Game Center`
- `Android ↔ iOS Mapping`
- `Marketplace`
- `Alternative Distribution`
- `Routing Coverage`

### App Setup / App Tags / Pre-orders

Most native placement:

- under `App` → `Metadata`

This is better than scattering them into admin-only or utility-only locations, because these are still app-facing store-configuration concerns.

The user intuition here is correct.

## Recommended V2 Sidebar Structure

### App

- `Overview`
- `Status`
- `History`
- `Builds`
- `TestFlight`
- `Submit`
- `Validate`
- `Publish`
- `Release Notes`
- `App Information`
- `Metadata`
- `Localizations`
- `Screenshots`
- `Video Previews`
- `Background Assets`
- `Categories`
- `Pre-orders`
- `App Tags`
- `App Setup`
- `Ratings & Reviews`
- `In-App Events`
- `Custom Product Pages`
- `Product Page Optimization`
- `Promo Codes`
- `Pricing & Availability`
- `In-App Purchases`
- `Subscriptions`
- `Insights`
- `Performance`
- `Analytics`
- `Finance`
- `Crashes`
- `App Review`
- `Age Rating`
- `Accessibility`
- `Encryption`
- `EULA`
- `Agreements`
- `App Clips`
- `Game Center`
- `Android ↔ iOS Mapping`
- `Marketplace`
- `Alternative Distribution`
- `Routing Coverage`
- `Nominations`

### Team

- `Account`
- `Users`
- `Actors`
- `Devices`

### Signing

- `Signing Overview`
- `Bundle IDs`
- `Certificates`
- `Profiles`
- `Merchant IDs`
- `Pass Type IDs`
- `Notarization`

### Automation

- `Workflow`
- `Webhooks`
- `Notifications`
- `Xcode Cloud`
- `Migrate`
- `Diff`
- `Schema`
- `ACP Sessions`

## Coverage Matrix

### Covered

- `apps` (partial through app picker)
- `versions`
- `status`
- `builds`
- `testflight`
- `review`
- `reviews`
- `performance`
- `insights`
- `finance`
- `pricing`
- `iap`
- `subscriptions`
- `app-events`
- `product-pages`
- `nominations`
- `game-center`
- `accessibility`

### Partial

- `apps`
- `metadata`
- `localizations`
- `screenshots`
- `pricing`
- `subscriptions`
- `age-rating`
- `finance`
- `overview`

### Missing

- `publish`
- `submit`
- `validate`
- `release`
- `release-notes`
- `video-previews`
- `background-assets`
- `app-setup`
- `app-tags`
- `categories`
- `pre-orders`
- `encryption`
- `eula`
- `agreements`
- `app-clips`
- `android-ios-mapping`
- `marketplace`
- `alternative-distribution`
- `routing-coverage`
- `build-bundles`
- `build-localizations`
- `sandbox`
- `xcode`
- `account`
- `users`
- `actors`
- `devices`
- `signing`
- `bundle-ids`
- `certificates`
- `profiles`
- `merchant-ids`
- `pass-type-ids`
- `notarization`
- `workflow`
- `webhooks`
- `xcode-cloud`
- `notify`
- `migrate`
- `diff`
- `analytics`
- `schema`
- `crashes`
- `feedback`
- `auth`
- `doctor`
- `docs`
- `init`
- `install-skills`
- `snitch`
- `completion`
- `version`

## Recommended Implementation Order

If another agent is continuing the work, this is the best order:

1. Introduce header scope tabs: `App`, `Team`, `Signing`, `Automation`.
2. Refactor the current `App` sidebar into grouped sections instead of a flat distribution-first list.
3. Split `overview` into dedicated `Metadata`, `Localizations`, and `Screenshots` sections.
4. Add release execution pages: `Submit`, `Validate`, `Publish`, `Release Notes`.
5. Add `Team` scope.
6. Add `Signing` scope.
7. Add `Automation` scope.
8. Add app-platform pages.
9. Clean up mislabeled sections like `App Privacy`.

## Codebase Notes For The Next Agent

### Current Wails Window Setup

See `apps/studio/main.go`. The app already uses:

- translucent window
- transparent webview
- transparent titlebar
- full-size content
- toolbar mode

This is already the right shell for a native-feeling macOS desktop app.

### Current Studio IA Source

Sidebar structure and section routing currently live mostly in `apps/studio/frontend/src/App.tsx`.

The next agent should expect to refactor that file rather than just append more conditions to it.

### Current Command Integration Style

Studio currently mixes:

- direct Wails backend methods like `GetPricingOverview`, `GetSubscriptions`, `GetFinanceRegions`
- generic command passthrough using `RunASCCommand`

That is acceptable for bootstrap, but the next phase should be more explicit:

- high-value pages should get typed backend methods
- low-priority or niche surfaces can remain command-backed initially

## Final Product Guidance

The next stage should not be "show every CLI command in the sidebar."

The next stage should be:

- define top-level operational scopes
- keep app-specific platform/store surfaces in the app sidebar
- move global operational concerns into header-level workspaces
- turn metadata into a real content workspace rather than an overloaded overview page

That is the path to a Studio app that feels deliberate instead of just comprehensive.

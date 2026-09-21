---
name: asc-app-create-ui
description: Create a new App Store Connect app record via browser automation. Use when there is no public API for app creation and you need an agent to drive the New App form.
---

# asc app create (UI automation)

Use this skill to create a new App Store Connect app by driving the web UI.
This is opt-in, local-only automation that requires the user to be signed in.

## Preconditions
- A browser automation tool is available (Playwright, Cursor browser MCP, or equivalent).
- User is signed in to App Store Connect (or can complete login + 2FA).
- The **bundle ID must already be registered** in the Apple Developer portal.
- Required inputs are known:
  - app name (max 30 characters)
  - bundle ID (must exist and be unused by another app)
  - SKU
  - platform (iOS, macOS, tvOS, visionOS)
  - primary language
  - user access (Full Access or Limited Access)

## Safety Guardrails
- Never export or store cookies.
- Use a visible browser session only.
- Pause for a final confirmation before clicking "Create" (for standalone scripts).
- Do not retry the Create action automatically on failure.

## Workflow

### 1. Preflight: register bundle ID and verify no existing app
```bash
# Register the bundle ID via public API (if not already registered)
asc bundle-ids create --identifier "com.example.app" --name "My App" --platform IOS

# Confirm no app record exists yet
asc apps list --bundle-id "com.example.app" --output json
```

### 2. Open App Store Connect
Navigate to `https://appstoreconnect.apple.com/apps` and ensure the user is signed in.

### 3. Open the New App form
The "New App" button (blue "+" icon) opens a **dropdown menu**, not a dialog directly.
- Click the "New App" button to open the dropdown.
- Click the "New App" **menu item** inside the dropdown.
- The creation dialog/modal appears.

### 4. Fill required fields (in order)

#### Platform (checkboxes)
The platforms are **checkboxes** (not radio buttons). Click the checkbox for the desired platform(s):
- iOS, macOS, tvOS, visionOS
- Multiple can be selected.

#### Name (text input)
- Label: `Name`
- Max 30 characters.

#### Primary Language (select/combobox)
- Label: `Primary Language`
- Use `select_option` or equivalent with the language label (e.g., `"English (U.S.)"`).

#### Bundle ID (select/combobox)
- Label: `Bundle ID`
- This is a `<select>` dropdown. The options load asynchronously after platform selection.
- Wait for the dropdown to finish loading (it shows "Loading..." initially).
- Select by matching the label text which includes both the name and identifier:
  `"My App - com.example.app"`

#### SKU (text input)
- Label: `SKU`

#### User Access (radio buttons) -- REQUIRED
- **This field is required.** The Create button stays disabled until one option is selected.
- Options: `Limited Access` or `Full Access`.
- These are custom radio buttons with `<span>` overlays.
- **Known issue:** Accessibility-based clicks may be intercepted by the overlay `<span>`.
- **Workaround:** Use `scrollIntoView` on the radio element first, then click the radio ref directly. This bypasses the overlay interception.

### 5. Click Create
- The "Create" button is disabled until all required fields are filled **and** User Access is selected.
- After clicking, the button text changes to "Creating" while processing.
- Wait for navigation to the new app's page (URL pattern: `/apps/<APP_ID>/...`).

### 6. Verify creation via API
```bash
asc apps view --id "APP_ID" --output json --pretty
# or
asc apps list --bundle-id "com.example.app" --output json
```

### 7. Hand off to post-create setup
```bash
asc app-setup info set --app "APP_ID" --primary-locale "en-US"
asc app-setup categories set --app "APP_ID" --primary GAMES
asc pricing availability create \
  --app "APP_ID" \
  --territory "USA,GBR" \
  --available true \
  --available-in-new-territories true
```

Use `asc pricing availability create` only for the first availability bootstrap. If Apple rejects that public-API bootstrap, authenticate a web session with `asc web auth login --apple-id "EMAIL"` and retry with `asc web apps availability create --app "APP_ID" --territory "USA,GBR" --available-in-new-territories true`, or configure Pricing and Availability in App Store Connect. If app availability already exists, switch to `asc pricing availability edit --app "APP_ID" ...` for later territory changes.

## Known UI Automation Issues

### "New App" is a dropdown menu, not a direct action
The first click opens a menu with "New App" and "New App Bundle". You must click the menu item, not just the button.

### User Access radio buttons have span overlays
Apple's custom radio buttons wrap the `<input type="radio">` in styled `<span>` elements. Direct ref-based clicks may fail with "click target intercepted". The fix is:
1. Scroll the radio element into view (`scrollIntoView`).
2. Click the radio ref directly (not via offset or label click).

### Bundle ID dropdown loads asynchronously
After selecting a platform, the Bundle ID dropdown shows "Loading..." and is disabled. Wait for it to become enabled and populated before selecting.

### browser_fill may not trigger form validation
Apple's Ember.js forms use custom change handlers. `browser_fill` (atomic set) may not trigger validation. If the Create button stays disabled after filling all fields:
- Retype the value slowly (character-by-character) in at least one text field.
- Or click the field, clear it, and type slowly.

## Failure Handling
- If any field or button cannot be located, stop and request user help.
- Capture a screenshot and report the last known step.
- Do not retry the Create click automatically.
- On failure, the user should check the browser for validation errors (red outlines, inline messages).

## Notes
- This skill is a workaround for a missing public API. Apple's docs explicitly state: "Don't use this API to create new apps; instead, create new apps on the App Store Connect website."
- UI selectors can change without notice. Prefer role/label/text selectors over CSS.
- The only manual step should be signing in. Everything else is agent-drivable.

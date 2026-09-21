---
name: asc-shots-pipeline
description: Orchestrate iOS screenshot automation with xcodebuild/simctl for build-run, AXe for UI actions, JSON settings and plan files, Koubou-based framing (`asc screenshots frame`), and screenshot upload (`asc screenshots upload`). Use when users ask for automated screenshot capture, AXe-driven simulator flows, frame composition, or screenshot-to-upload pipelines.
---

# asc screenshots pipeline (xcodebuild -> AXe -> frame -> asc)

Use this skill for agent-driven screenshot workflows where the app is built and launched with Xcode CLI tools, UI is driven with AXe, and screenshots are uploaded with `asc`.

## Current scope
- Implemented now: build/run, AXe plan capture, frame composition, and upload.
- Device discovery is built-in via `asc screenshots list-frame-devices`.
- Local screenshot automation commands are experimental in asc cli.
- Framing is pinned to Koubou `0.18.1` for deterministic output.
- Feedback/issues: https://github.com/rorkai/App-Store-Connect-CLI/issues/new/choose

## Defaults
- Settings file: `.asc/shots.settings.json`
- Capture plan: `.asc/screenshots.json`
- Raw screenshots dir: `./screenshots/raw`
- Framed screenshots dir: `./screenshots/framed`
- Default frame device: `iphone-air`

## 1) Create settings JSON first

Create or update `.asc/shots.settings.json`:

```json
{
  "version": 1,
  "app": {
    "bundle_id": "com.example.app",
    "project": "MyApp.xcodeproj",
    "scheme": "MyApp",
    "simulator_udid": "booted"
  },
  "paths": {
    "plan": ".asc/screenshots.json",
    "raw_dir": "./screenshots/raw",
    "framed_dir": "./screenshots/framed"
  },
  "pipeline": {
    "frame_enabled": true,
    "upload_enabled": false
  },
  "upload": {
    "version_localization_id": "",
    "device_type": "IPHONE_65",
    "source_dir": "./screenshots/framed"
  }
}
```

If you intentionally skip framing, set:
- `"frame_enabled": false`
- `"upload.source_dir": "./screenshots/raw"`

## 2) Build and run app on simulator

Use Xcode CLI for build/install/launch:

```bash
xcrun simctl boot "$UDID" || true

xcodebuild \
  -project "MyApp.xcodeproj" \
  -scheme "MyApp" \
  -configuration Debug \
  -destination "platform=iOS Simulator,id=$UDID" \
  -derivedDataPath ".build/DerivedData" \
  build

xcrun simctl install "$UDID" ".build/DerivedData/Build/Products/Debug-iphonesimulator/MyApp.app"
xcrun simctl launch "$UDID" "com.example.app"
```

Use `xcodebuild -showBuildSettings` if the app bundle path differs from the default location.

## 3) Capture screenshots with AXe (or `asc screenshots run`)

Prefer plan-driven capture:

```bash
asc screenshots run --plan ".asc/screenshots.json" --udid "$UDID" --output json
```

Useful AXe primitives during plan authoring:

```bash
axe describe-ui --udid "$UDID"
axe tap --id "search_field" --udid "$UDID"
axe type "wwdc" --udid "$UDID"
axe screenshot --output "./screenshots/raw/home.png" --udid "$UDID"
```

Minimal `.asc/screenshots.json` example:

```json
{
  "version": 1,
  "app": {
    "bundle_id": "com.example.app",
    "udid": "booted",
    "output_dir": "./screenshots/raw"
  },
  "steps": [
    { "action": "launch" },
    { "action": "wait", "duration_ms": 800 },
    { "action": "screenshot", "name": "home" }
  ]
}
```

## 4) Frame screenshots with `asc screenshots frame`

The asc CLI pins framing to Koubou `0.18.1`.
Install and verify before running framing steps:

```bash
pip install koubou==0.18.1
kou --version  # expect 0.18.1
# If Koubou reports missing device frames, run once with network access:
kou setup-frames
```

List supported frame device values first:

```bash
asc screenshots list-frame-devices --output json
```

Frame one screenshot (defaults to `iphone-air`):

```bash
asc screenshots frame \
  --input "./screenshots/raw/home.png" \
  --output-dir "./screenshots/framed" \
  --device "iphone-air" \
  --output json
```

Supported `--device` values:
- `iphone-air` (default)
- `iphone-17-pro`
- `iphone-17-pro-max`
- `iphone-16e`
- `iphone-17`
- `mac`

## 5) Upload screenshots with asc

Generate and review artifacts before upload:

```bash
asc screenshots review-generate --framed-dir "./screenshots/framed" --output-dir "./screenshots/review"
asc screenshots review-open --output-dir "./screenshots/review"
asc screenshots review-approve --all-ready --output-dir "./screenshots/review"
```

For reviewed multi-locale sets, prefer the plan/apply flow so existing remote screenshot counts are included before upload:

```bash
asc screenshots plan --app "APP_ID" --version "1.2.3" --review-output-dir "./screenshots/review" --output json
asc screenshots apply --app "APP_ID" --version "1.2.3" --review-output-dir "./screenshots/review" --confirm --output json
```

Upload from the configured source directory (default `./screenshots/framed` when framing is enabled):

```bash
asc screenshots upload \
  --version-localization "LOC_ID" \
  --path "./screenshots/framed" \
  --device-type "IPHONE_65" \
  --output json
```

List or validate before upload when needed:

```bash
asc screenshots sizes --output table
asc screenshots list --version-localization "LOC_ID" --output table
```

## Agent behavior
- Always confirm exact flags with `--help` before running commands.
- Re-check command paths with `asc screenshots --help` because screenshot commands are evolving quickly.
- Keep outputs deterministic: default to JSON for machine steps.
- Prefer `asc screenshots list-frame-devices --output json` before selecting a frame device.
- Ensure screenshot files exist before upload.
- Use explicit long flags (`--app`, `--output`, `--version-localization`, etc.).
- Treat screenshot-local automation as experimental and call it out in user-facing handoff notes.
- Use `asc screenshots plan` / `asc screenshots apply` for reviewed batches when you need append-limit guardrails across existing remote screenshots.
- If framing fails with a version error, re-install pinned Koubou: `pip install koubou==0.18.1`.
- If framing fails because device frames are missing, run `kou setup-frames` once with network access.

## 6) Multi-locale capture (optional)

Do not use `xcrun simctl launch ... -e AppleLanguages` for localization.
`-e` is an environment variable pattern and does not reliably switch app language.

For this pipeline, use simulator-wide locale defaults per UDID. This works with
`asc screenshots capture`, which relaunches the app internally.

```bash
# Map each locale to a dedicated simulator UDID.
# (Create these simulators once with `xcrun simctl create`.)
declare -A LOCALE_UDID=(
  ["en-US"]="UDID_EN_US"
  ["de-DE"]="UDID_DE_DE"
  ["fr-FR"]="UDID_FR_FR"
  ["ja-JP"]="UDID_JA_JP"
)

set_simulator_locale() {
  local UDID="$1"
  local LOCALE="$2"            # e.g. de-DE
  local LANG="${LOCALE%%-*}"   # de
  local APPLE_LOCALE="${LOCALE/-/_}" # de_DE

  xcrun simctl boot "$UDID" || true
  xcrun simctl spawn "$UDID" defaults write NSGlobalDomain AppleLanguages -array "$LANG"
  xcrun simctl spawn "$UDID" defaults write NSGlobalDomain AppleLocale -string "$APPLE_LOCALE"
}

for LOCALE in "${!LOCALE_UDID[@]}"; do
  UDID="${LOCALE_UDID[$LOCALE]}"
  echo "Capturing $LOCALE on $UDID..."
  set_simulator_locale "$UDID" "$LOCALE"

  xcrun simctl terminate "$UDID" "com.example.app" || true
  asc screenshots capture \
    --bundle-id "com.example.app" \
    --name "home" \
    --udid "$UDID" \
    --output-dir "./screenshots/raw/$LOCALE" \
    --output json
done
```

If you launch manually (outside `asc screenshots capture`), use app launch arguments:

```bash
xcrun simctl launch "$UDID" "com.example.app" -AppleLanguages "(de)" -AppleLocale "de_DE"
```

## 7) Parallel execution for speed

Run one locale per simulator UDID in parallel:

```bash
#!/bin/bash
# parallel-capture.sh

declare -A LOCALE_UDID=(
  ["en-US"]="UDID_EN_US"
  ["de-DE"]="UDID_DE_DE"
  ["fr-FR"]="UDID_FR_FR"
  ["ja-JP"]="UDID_JA_JP"
)

capture_locale() {
  local LOCALE="$1"
  local UDID="$2"
  local LANG="${LOCALE%%-*}"
  local APPLE_LOCALE="${LOCALE/-/_}"

  echo "Starting $LOCALE on $UDID"
  xcrun simctl boot "$UDID" || true
  xcrun simctl spawn "$UDID" defaults write NSGlobalDomain AppleLanguages -array "$LANG"
  xcrun simctl spawn "$UDID" defaults write NSGlobalDomain AppleLocale -string "$APPLE_LOCALE"
  xcrun simctl terminate "$UDID" "com.example.app" || true

  asc screenshots capture \
    --bundle-id "com.example.app" \
    --name "home" \
    --udid "$UDID" \
    --output-dir "./screenshots/raw/$LOCALE" \
    --output json

  echo "Completed $LOCALE"
}

for LOCALE in "${!LOCALE_UDID[@]}"; do
  capture_locale "$LOCALE" "${LOCALE_UDID[$LOCALE]}" &
done

wait
echo "All captures done. Now framing..."
```

Or use `xargs` with `locale:udid` pairs:

```bash
printf "%s\n" \
  "en-US:UDID_EN_US" \
  "de-DE:UDID_DE_DE" \
  "fr-FR:UDID_FR_FR" \
  "ja-JP:UDID_JA_JP" | xargs -P 4 -I {} bash -c '
    PAIR="{}"
    LOCALE="${PAIR%%:*}"
    UDID="${PAIR##*:}"
    LANG="${LOCALE%%-*}"
    APPLE_LOCALE="${LOCALE/-/_}"
    xcrun simctl boot "$UDID" || true
    xcrun simctl spawn "$UDID" defaults write NSGlobalDomain AppleLanguages -array "$LANG"
    xcrun simctl spawn "$UDID" defaults write NSGlobalDomain AppleLocale -string "$APPLE_LOCALE"
    xcrun simctl terminate "$UDID" "com.example.app" || true
    asc screenshots capture --bundle-id "com.example.app" --name "home" --udid "$UDID" --output-dir "./screenshots/raw/$LOCALE" --output json
  '
```

## 8) Full multi-locale pipeline example

```bash
#!/bin/bash
# full-pipeline-multi-locale.sh

declare -A LOCALE_UDID=(
  ["en-US"]="UDID_EN_US"
  ["de-DE"]="UDID_DE_DE"
  ["fr-FR"]="UDID_FR_FR"
  ["es-ES"]="UDID_ES_ES"
  ["ja-JP"]="UDID_JA_JP"
)

DEVICE="iphone-air"
RAW_DIR="./screenshots/raw"
FRAMED_DIR="./screenshots/framed"

# Step 1: Parallel capture with per-simulator locale defaults
for LOCALE in "${!LOCALE_UDID[@]}"; do
  (
    UDID="${LOCALE_UDID[$LOCALE]}"
    LANG="${LOCALE%%-*}"
    APPLE_LOCALE="${LOCALE/-/_}"

    xcrun simctl boot "$UDID" || true
    xcrun simctl spawn "$UDID" defaults write NSGlobalDomain AppleLanguages -array "$LANG"
    xcrun simctl spawn "$UDID" defaults write NSGlobalDomain AppleLocale -string "$APPLE_LOCALE"
    xcrun simctl terminate "$UDID" "com.example.app" || true

    asc screenshots capture \
      --bundle-id "com.example.app" \
      --name "home" \
      --udid "$UDID" \
      --output-dir "$RAW_DIR/$LOCALE" \
      --output json
    echo "Captured $LOCALE"
  ) &
done
wait

# Step 2: Parallel framing
for LOCALE in "${!LOCALE_UDID[@]}"; do
  (
    asc screenshots frame \
      --input "$RAW_DIR/$LOCALE/home.png" \
      --output-dir "$FRAMED_DIR/$LOCALE" \
      --device "$DEVICE" \
      --output json
    echo "Framed $LOCALE"
  ) &
done
wait

# Step 3: Generate review (single run, aggregates all locales)
asc screenshots review-generate \
  --framed-dir "$FRAMED_DIR" \
  --output-dir "./screenshots/review"

# Step 4: Upload (run per locale if needed)
for LOCALE in "${!LOCALE_UDID[@]}"; do
  asc screenshots upload \
    --version-localization "LOC_ID_FOR_$LOCALE" \
    --path "$FRAMED_DIR/$LOCALE" \
    --device-type "IPHONE_65" \
    --output json
done
```

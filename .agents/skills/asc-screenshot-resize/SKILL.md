---
name: asc-screenshot-resize
description: Resize and validate App Store screenshots with current asc screenshot-size data and macOS sips. Use when preparing or fixing screenshots for App Store Connect submission.
---

# asc screenshot resize

Use this skill to prepare screenshots for App Store Connect. Do not rely on a hard-coded dimension table in this skill; the CLI owns the current size matrix.

## Source of truth

Always discover current accepted sizes from `asc` first:

```bash
asc screenshots sizes --output table
asc screenshots sizes --all --output table
```

For local validation before upload:

```bash
asc screenshots validate --path "./screenshots/iphone" --device-type "IPHONE_65" --output table
asc screenshots validate --path "./screenshots/ipad" --device-type "IPAD_PRO_3GEN_129" --output table
```

Common current device-type anchors:

- `IPHONE_65` for the common 6.5-inch iPhone screenshot set.
- `IPAD_PRO_3GEN_129` for the common 12.9/13-inch iPad screenshot set.

Run `asc screenshots sizes --all` when targeting other display types such as 6.9-inch iPhone, Apple TV, Mac, Vision Pro, iMessage, or Watch.

## Workflow

### 1. Sanitize filenames

macOS screenshots can contain hidden Unicode spaces that make tools fail with "not a valid file". Sanitize before batch work:

```bash
python3 -c "
import os
for f in os.listdir('.'):
    clean = f.replace('\u202f', ' ')
    if f != clean:
        os.rename(f, clean)
        print(f'Renamed: {clean}')
"
```

### 2. Inspect dimensions and metadata

```bash
sips -g pixelWidth -g pixelHeight screenshot.png
sips -g hasAlpha -g space screenshot.png
```

App Store Connect rejects screenshots with alpha transparency. Strip alpha by round-tripping through JPEG:

```bash
sips -s format jpeg input.png --out /tmp/asc-screenshot-no-alpha.jpg
sips -s format png /tmp/asc-screenshot-no-alpha.jpg --out output.png
rm /tmp/asc-screenshot-no-alpha.jpg
```

Batch-strip alpha from PNGs:

```bash
for f in *.png; do
  if sips -g hasAlpha "$f" | grep -q "yes"; then
    sips -s format jpeg "$f" --out /tmp/asc-screenshot-no-alpha.jpg
    sips -s format png /tmp/asc-screenshot-no-alpha.jpg --out "$f"
    rm /tmp/asc-screenshot-no-alpha.jpg
    echo "Stripped alpha: $f"
  fi
done
```

### 3. Resize only after choosing a target from asc

Pick a width and height from `asc screenshots sizes --all`. `sips -z` takes height first, then width:

```bash
# Example: portrait IPHONE_65 1284 x 2778
sips -z 2778 1284 input.png --out output.png
```

Batch resize to a chosen target:

```bash
mkdir -p resized
for f in *.png; do
  sips -z 2778 1284 "$f" --out "resized/$f"
done
```

### 4. Validate outputs with asc

```bash
sips -g pixelWidth -g pixelHeight -g hasAlpha resized/*.png
asc screenshots validate --path "./resized" --device-type "IPHONE_65" --output table
```

### 5. Upload only after validation

```bash
asc screenshots upload --version-localization "LOC_ID" --path "./resized" --device-type "IPHONE_65" --dry-run --output table
asc screenshots upload --version-localization "LOC_ID" --path "./resized" --device-type "IPHONE_65"
```

## Guardrails

- Treat `asc screenshots sizes --all` as authoritative; Apple size requirements change.
- Do not stretch screenshots across incompatible aspect ratios unless the user accepts the visual tradeoff.
- Always output to a separate file or directory to preserve originals.
- Screenshots must be PNG or JPEG and must not include alpha transparency.
- Convert Display P3 or other color spaces to sRGB when needed:

```bash
sips -m "/System/Library/ColorSync/Profiles/sRGB IEC61966-2.1.icc" input.png --out output.png
```

- Prefer `asc screenshots validate` over visual inspection before upload.

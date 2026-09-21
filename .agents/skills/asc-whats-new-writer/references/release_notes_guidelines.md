# Release Notes Guidelines

Rules and examples for writing engaging App Store release notes (What's New).

## Structure

Three optional sections. Only include sections with content — omit empty ones.

- **New** — new features or capabilities
- **Improved** — enhancements to existing features
- **Fixed** — bug fixes users would notice

## The 170-Character Rule

The first ~170 characters of What's New are visible on the app page without tapping "more." This is the hook.

- Lead with the single most impactful change
- Write a complete, compelling sentence — not a truncated list
- Assume most users will never tap "more"

Example hook (168 chars):
"Search just got faster — find what you need in seconds. Plus: smarter notifications and smoother transitions throughout the app."

## Tone

- **Benefit-focused, not feature-focused.** "Find your favorites in seconds" not "Optimized search indexing algorithm."
- **Engaging but not fluffy** — every word earns its place
- **Direct address ("you")** to create connection
- **Action verbs** over passive descriptions ("Track your progress" not "Progress tracking added")
- **Specific over vague** — mention concrete improvements, not abstract promises

## Anti-Patterns

| Don't | Why |
|-------|-----|
| "Bug fixes and improvements" | Tells the user nothing. Wastes the conversion opportunity. |
| "Version 2.1.0 — We've been working hard!" | Version numbers in headings violate Apple guidelines. Self-congratulation wastes space. |
| Mentioning competitors by name | Against App Store Review Guidelines. |
| References to other platforms | "Now matching our Android version" alienates iOS users. |
| Keyword stuffing | What's New is NOT indexed for search. Every word should serve conversion, not SEO. |
| Marketing fluff with no substance | "The best update ever!" without specifics erodes trust. |
| Walls of text | Users skim. Use short paragraphs or bullet points. |

## Good vs. Bad Examples

**Bad:**
"Bug fixes and performance improvements."

**Good:**
"Search just got faster — find what you need in seconds. Plus: improved notification accuracy and smoother transitions throughout the app."

---

**Bad:**
"Version 2.1.0 — We've been working hard on improvements!"

**Good:**
"New sleep timer options let you drift off to soothing audio. Choose 15, 30, 45, or 60 minutes — perfect for winding down before bed."

---

**Bad:**
"Fixed bugs. Updated UI. Various improvements across the app."

**Good:**
"Real-time highlighting is now perfectly synced, even at 2x speed. Dark mode colors are easier on the eyes, and the app launches 40% faster."

---

**Bad:**
"We fixed a crash that some users reported. Also updated some things in the background."

**Good:**
"No more crashes when switching between tabs — thanks for reporting this! Background sync is now 3x faster, so your data stays up to date."

## Keyword Echo Strategy

What's New is **not indexed** for App Store search. Keywords here serve **conversion only** — users who see familiar search terms in the release notes feel confident they found the right app.

**How to echo:**
1. Read the locale's `keywords` field from local metadata
2. Identify keywords relevant to the changes being described
3. Weave them naturally into sentences — do NOT force irrelevant keywords
4. If the keywords field is empty or missing, skip this step

**Example:** If keywords include `workout,tracker,calories`:
"Improved workout tracking accuracy and real-time calorie counter updates" naturally echoes three keywords.

**Do NOT:** Insert keywords that have nothing to do with the update. "Bug fix for login screen" should not mention "workout" or "calories."

## Promotional Text Pairing

When drafting What's New, optionally draft a matching **Promotional Text** (170 chars max):

- Summarize the update's theme in one punchy line
- Can reference seasonal events (Ramadan, Eid, back-to-school, New Year)
- **Updatable without app submission** — the only "living" metadata field
- Not indexed for search — write purely for conversion
- Refresh monthly or with each major update for re-engagement

**Example:** "New sleep timer + faster search. Your evening routine just got better."

## Localization Notes

- **Formal register** across all languages — use formal "you" forms (Russian: вы, German: Sie, French: vous, Spanish: usted)
- **Cultural adaptation over literal translation** — idioms and playful phrases need local equivalents, not word-for-word translation
- **Load locale-specific keywords** and echo them in the translated notes (each locale has different keywords)
- **Account for text expansion** — some languages expand 30-40% vs. English. Aim for 500-1500 chars in English to leave room.
- **Validate** all translations fit within 4000-char limit
- If translation exceeds the limit, **shorten — never truncate mid-sentence**

## Character Limits

| Field | Limit | Indexed? | Requires Submission? |
|-------|-------|----------|---------------------|
| What's New | 4,000 | No | Yes |
| Promotional Text | 170 | No | No |

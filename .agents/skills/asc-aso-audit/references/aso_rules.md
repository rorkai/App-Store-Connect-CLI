# ASO Rules Reference

Rules enforced by the asc-aso-audit skill. Each rule links to the check that tests it.

## Indexing Rules

- **Title, subtitle, and keyword field are indexed** for App Store search.
- **Description and promotional text are NOT indexed.** Description is for conversion (users see search terms reflected → higher download rate); promotional text is for seasonal messaging.
- **Description keyword coverage still matters** — while not indexed, descriptions that naturally include keyword terms improve conversion rate, which indirectly boosts search rankings.
- **Screenshot captions are OCR-indexed** (since June 2025 algorithm update). Use high-value keywords in caption text. *(Informational — not checked by this audit.)*
- **Apple Full Text Search combines words across title + subtitle + keywords.** Single words enable more combinations than multi-word phrases. Example: "quran" + "recitation" in separate fields still matches "quran recitation".
- **Cross-field combo optimization:** When adding keywords, consider what search queries they enable in combination with words already in title/subtitle. Example: adding "holy" to keywords when "Quran" is in subtitle enables the search "holy quran".

## Keyword Field Rules

- **Comma-separated, no spaces after commas.** Spaces waste characters. `quran,recitation` not `quran, recitation`.
- **Do not duplicate words already in title or subtitle.** Apple indexes all three fields together; repeating a word wastes keyword budget.
- **Do not use the app name in keywords.** It is already indexed.
- **Avoid plurals if the singular is already present** — Apple handles stemming.
- **Prefer single words over multi-word phrases** — single words enable more cross-field combinations and use fewer characters.
- **Always validate with popularity data** — never guess keyword value. Use Astro MCP or similar tools to check popularity scores before making swaps.

## Character Limits

| Field | Limit |
|-------|-------|
| Name | 30 |
| Subtitle | 30 |
| Keywords | 100 |
| Description | 4,000 |
| What's New | 4,000 |
| Promotional Text | 170 |

## Localization Rules

- **Localize keywords per market** — do not just translate your primary keywords. Research what users in each locale actually search for.
- **English (US) keywords may carry into other English-speaking storefronts** but dedicated localization always outperforms.
- **Identical keyword fields across locales** usually indicates untranslated/unlocalized metadata.
- **Track keywords in each locale's store** — keyword popularity varies dramatically across territories. A keyword with 70 popularity in the US store may have 5 popularity in France. Query existing Astro tracking; report and skip untracked stores during an audit. Use `add_keywords` only when tracking setup is explicitly authorized.
- **Use competitor analysis per store** — top competitors differ by market. Run `extract_competitors_keywords` with locale-relevant competitor apps.

## Non-Latin Script Rules

- **Arabic ال-prefix:** Apple likely normalizes the definite article ال. Treat "القرآن" (with ال) and "قرآن" (without) as probable duplicates when checking subtitle/keyword overlap.
- **Arabic hamza variants:** Users commonly search without hamza diacritics. "قران" (no hamza) and "قرآن" (with hamza) target different search queries — both may be worth including.
- **Chinese tokenization:** Chinese text has no word-separating spaces. Subtitle tokens are separated by `、` (enumeration comma) or `，` (full comma). Do not use whitespace tokenization for Chinese metadata.
- **Cyrillic transliteration:** Including the Latin spelling of terms (e.g., "quran" in a Russian keyword field) targets users who search in Latin script on Cyrillic storefronts.

## Utilization Guidelines

- **Keyword field:** aim for 90%+ character usage (90+ of 100 chars).
- **Subtitle:** aim for 65%+ character usage (20+ of 30 chars). Short subtitles waste indexing opportunity.
- **Name:** balance branding with keyword inclusion.

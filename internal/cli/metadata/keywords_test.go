package metadata

import "testing"

func TestSplitMetadataKeywordTokensSupportsMixedSeparators(t *testing.T) {
	got := splitMetadataKeywordTokens(" habit tracker，mood journal、sleep log;\nenergy tracker； focus timer ")
	want := []string{
		"habit tracker",
		"mood journal",
		"sleep log",
		"energy tracker",
		"focus timer",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected token %d to be %q, got %q (%v)", i, want[i], got[i], got)
		}
	}
}

func TestNormalizeMetadataKeywordListDetailedDeduplicatesCaseAndWhitespace(t *testing.T) {
	got, duplicates, err := normalizeMetadataKeywordListDetailed([]string{
		"  habit   tracker ",
		"mood journal",
		"Habit Tracker",
		"mood   journal",
		"  ",
	})
	if err != nil {
		t.Fatalf("normalizeMetadataKeywordListDetailed() error: %v", err)
	}
	want := []string{"habit tracker", "mood journal"}
	if len(got) != len(want) {
		t.Fatalf("expected %d normalized keywords, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected keyword %d to be %q, got %q (%v)", i, want[i], got[i], got)
		}
	}
	if len(duplicates) != 2 || duplicates[0] != "Habit Tracker" || duplicates[1] != "mood journal" {
		t.Fatalf("expected duplicate tracking, got %v", duplicates)
	}
}

func TestDecodeMetadataKeywordValueArrayExpandsEmbeddedSeparators(t *testing.T) {
	got, err := decodeMetadataKeywordValue([]any{
		"habit tracker，mood journal",
		"sleep log",
		" Focus tracker ",
	})
	if err != nil {
		t.Fatalf("decodeMetadataKeywordValue() error: %v", err)
	}
	want := []string{"habit tracker", "mood journal", "sleep log", "Focus tracker"}
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected token %d to be %q, got %q (%v)", i, want[i], got[i], got)
		}
	}
}

func TestBuildMetadataKeywordFieldNormalizesMixedInput(t *testing.T) {
	field, count, err := buildMetadataKeywordField([]string{
		"habit tracker",
		" mood journal ",
		"Habit Tracker",
		"sleep log",
	})
	if err != nil {
		t.Fatalf("buildMetadataKeywordField() error: %v", err)
	}
	if field != "habit tracker,mood journal,sleep log" {
		t.Fatalf("expected canonical keyword field, got %q", field)
	}
	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}
}

func TestParseMetadataKeywordJSONAcceptsLocaleMapObjectsWithSideData(t *testing.T) {
	got, err := parseMetadataKeywordJSON([]byte(`{
		"en-US": {
			"keywords": ["habit tracker", "mood journal"],
			"popularity": 42,
			"difficulty": 30,
			"notes": "high intent",
			"tags": ["opportunity"]
		},
		"fr-FR": {
			"keyword": "journal humeur",
			"rank": 5
		}
	}`), "")
	if err != nil {
		t.Fatalf("parseMetadataKeywordJSON() error: %v", err)
	}

	if len(got.locales) != 2 {
		t.Fatalf("expected 2 locales, got %d: %v", len(got.locales), got.locales)
	}
	if len(got.locales["en-US"]) != 2 || got.locales["en-US"][0] != "habit tracker" || got.locales["en-US"][1] != "mood journal" {
		t.Fatalf("unexpected en-US keywords: %v", got.locales["en-US"])
	}
	if len(got.locales["fr-FR"]) != 1 || got.locales["fr-FR"][0] != "journal humeur" {
		t.Fatalf("unexpected fr-FR keywords: %v", got.locales["fr-FR"])
	}
	if len(got.sideData) != 2 {
		t.Fatalf("expected side data records, got %d: %+v", len(got.sideData), got.sideData)
	}
}

func TestResolveMetadataKeywordImportFormatAcceptsPreset(t *testing.T) {
	got, err := resolveMetadataKeywordImportFormat("astro-export.csv", "astro-csv")
	if err != nil {
		t.Fatalf("resolveMetadataKeywordImportFormat() error: %v", err)
	}
	if got != keywordImportFormatAstroCSV {
		t.Fatalf("expected astro-csv, got %q", got)
	}
}

func TestParseMetadataKeywordAstroCSVUsesKeywordColumn(t *testing.T) {
	data := []byte("Keyword,Notes,Popularity,Difficulty,Position,Apps in Ranking\nhabit tracker,high intent,42,31,7,App A\nmood journal,secondary,35,28,9,App B\n")
	got, err := parseMetadataKeywordAstroCSV(data, "en-US")
	if err != nil {
		t.Fatalf("parseMetadataKeywordAstroCSV() error: %v", err)
	}
	if len(got.locales) != 1 {
		t.Fatalf("expected 1 locale, got %d: %v", len(got.locales), got.locales)
	}
	if len(got.locales["en-US"]) != 2 || got.locales["en-US"][0] != "habit tracker" || got.locales["en-US"][1] != "mood journal" {
		t.Fatalf("unexpected astro keywords: %v", got.locales["en-US"])
	}
	if len(got.sideData) != 2 {
		t.Fatalf("expected side data rows from astro csv, got %d: %+v", len(got.sideData), got.sideData)
	}
}

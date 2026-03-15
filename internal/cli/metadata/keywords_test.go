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

func TestNormalizeMetadataKeywordListDeduplicatesCaseAndWhitespace(t *testing.T) {
	got, err := normalizeMetadataKeywordList([]string{
		"  habit   tracker ",
		"mood journal",
		"Habit Tracker",
		"mood   journal",
		"  ",
	})
	if err != nil {
		t.Fatalf("normalizeMetadataKeywordList() error: %v", err)
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

	if len(got) != 2 {
		t.Fatalf("expected 2 locales, got %d: %v", len(got), got)
	}
	if len(got["en-US"]) != 2 || got["en-US"][0] != "habit tracker" || got["en-US"][1] != "mood journal" {
		t.Fatalf("unexpected en-US keywords: %v", got["en-US"])
	}
	if len(got["fr-FR"]) != 1 || got["fr-FR"][0] != "journal humeur" {
		t.Fatalf("unexpected fr-FR keywords: %v", got["fr-FR"])
	}
}

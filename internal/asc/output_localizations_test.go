package asc

import (
	"strings"
	"testing"
)

func TestPrintTableAndMarkdown_AppStoreVersionLocalizationsIncludeID(t *testing.T) {
	resp := &AppStoreVersionLocalizationsResponse{
		Data: []Resource[AppStoreVersionLocalizationAttributes]{
			{
				ID: "loc-en-1",
				Attributes: AppStoreVersionLocalizationAttributes{
					Locale:   "en-US",
					WhatsNew: "Bug fixes",
					Keywords: "photo,editor",
				},
			},
		},
	}

	table := captureStdout(t, func() error { return PrintTable(resp) })
	for _, want := range []string{"ID", "Locale", "loc-en-1", "en-US", "Bug fixes"} {
		if !strings.Contains(table, want) {
			t.Fatalf("expected table to contain %q, got:\n%s", want, table)
		}
	}

	markdown := captureStdout(t, func() error { return PrintMarkdown(resp) })
	wantMarkdown := "| ID       | Locale | Whats New | Keywords     |\n" +
		"|:---------|:-------|:----------|:-------------|\n" +
		"| loc-en-1 | en-US  | Bug fixes | photo,editor |\n"
	if !strings.Contains(markdown, wantMarkdown) {
		t.Fatalf("expected markdown with ID column:\n%s\ngot:\n%s", wantMarkdown, markdown)
	}
}

func TestPrintTableAndMarkdown_AppInfoLocalizationsIncludeID(t *testing.T) {
	resp := &AppInfoLocalizationsResponse{
		Data: []Resource[AppInfoLocalizationAttributes]{
			{
				ID: "info-loc-1",
				Attributes: AppInfoLocalizationAttributes{
					Locale:   "en-US",
					Name:     "Example App",
					Subtitle: "Edit faster",
				},
			},
		},
	}

	table := captureStdout(t, func() error { return PrintTable(resp) })
	for _, want := range []string{"ID", "Locale", "info-loc-1", "Example App"} {
		if !strings.Contains(table, want) {
			t.Fatalf("expected table to contain %q, got:\n%s", want, table)
		}
	}

	markdown := captureStdout(t, func() error { return PrintMarkdown(resp) })
	wantMarkdown := "| ID         | Locale | Name        | Subtitle    | Privacy Policy URL |\n" +
		"|:-----------|:-------|:------------|:------------|:-------------------|\n" +
		"| info-loc-1 | en-US  | Example App | Edit faster |                    |\n"
	if !strings.Contains(markdown, wantMarkdown) {
		t.Fatalf("expected markdown with ID column:\n%s\ngot:\n%s", wantMarkdown, markdown)
	}
}

package asc

import (
	"net/url"
	"testing"
)

func TestBuildAppStoreVersionLocalizationsQueryEmitsInclude(t *testing.T) {
	query := &appStoreVersionLocalizationsQuery{}
	opts := []AppStoreVersionLocalizationsOption{
		Limit[AppStoreVersionLocalizationsOption](25),
		WithAppStoreVersionLocalizationLocales([]string{"en-US", "ja"}),
		WithAppStoreVersionLocalizationsInclude([]string{"appScreenshotSets", "appPreviewSets"}),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildAppStoreVersionLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if got := values.Get("include"); got != "appScreenshotSets,appPreviewSets" {
		t.Fatalf("expected include=appScreenshotSets,appPreviewSets, got %q", got)
	}
	if got := values.Get("filter[locale]"); got != "en-US,ja" {
		t.Fatalf("expected filter[locale]=en-US,ja, got %q", got)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("expected limit=25, got %q", got)
	}
}

func TestBuildAppStoreVersionLocalizationsQueryOmitsEmptyInclude(t *testing.T) {
	query := &appStoreVersionLocalizationsQuery{}
	WithAppStoreVersionLocalizationsInclude(nil)(query)

	values, err := url.ParseQuery(buildAppStoreVersionLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if _, ok := values["include"]; ok {
		t.Fatalf("expected no include parameter, got %q", values.Get("include"))
	}
}

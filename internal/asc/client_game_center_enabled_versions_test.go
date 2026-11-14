package asc

import (
	"net/url"
	"testing"
)

func TestGCEnabledVersionsOptions(t *testing.T) {
	query := &gcEnabledVersionsQuery{}
	Limit[GCEnabledVersionsOption](9)(query)
	NextURL[GCEnabledVersionsOption]("next")(query)

	values, err := url.ParseQuery(buildGCEnabledVersionsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "9" {
		t.Fatalf("expected limit=9, got %q", values.Get("limit"))
	}
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
}

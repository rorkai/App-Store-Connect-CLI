package asc

import (
	"net/url"
	"testing"
)

func TestBuildDevicesQuery(t *testing.T) {
	query := &devicesQuery{}
	opts := []DevicesOption{
		WithDevicesNames([]string{"My iPhone", " My iPad "}),
		WithDevicesPlatform("ios"),
		WithDevicesStatus("enabled"),
		WithDevicesUDIDs([]string{"UDID-1", " UDID-2 "}),
		WithDevicesIDs([]string{"device-1"}),
		WithDevicesSort("-name"),
		WithDevicesFields([]string{"name", "udid"}),
		Limit[DevicesOption](10),
	}
	for _, opt := range opts {
		opt(query)
	}

	values, err := url.ParseQuery(buildDevicesQuery(query))
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	if got := values.Get("filter[name]"); got != "My iPhone,My iPad" {
		t.Fatalf("expected filter[name] CSV, got %q", got)
	}
	if got := values.Get("filter[platform]"); got != "IOS" {
		t.Fatalf("expected filter[platform]=IOS, got %q", got)
	}
	if got := values.Get("filter[status]"); got != "ENABLED" {
		t.Fatalf("expected filter[status]=ENABLED, got %q", got)
	}
	if got := values.Get("filter[udid]"); got != "UDID-1,UDID-2" {
		t.Fatalf("expected filter[udid] CSV, got %q", got)
	}
	if got := values.Get("filter[id]"); got != "device-1" {
		t.Fatalf("expected filter[id]=device-1, got %q", got)
	}
	if got := values.Get("sort"); got != "-name" {
		t.Fatalf("expected sort=-name, got %q", got)
	}
	if got := values.Get("fields[devices]"); got != "name,udid" {
		t.Fatalf("expected fields[devices]=name,udid, got %q", got)
	}
	if got := values.Get("limit"); got != "10" {
		t.Fatalf("expected limit=10, got %q", got)
	}
}

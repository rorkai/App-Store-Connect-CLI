package asc

import (
	"reflect"
	"testing"
)

func TestAppPriceScheduleNotConfiguredRows(t *testing.T) {
	headers, rows := appPriceScheduleNotConfiguredRows(&AppPriceScheduleNotConfiguredResult{AppID: "app-1"})
	if want := []string{"App ID", "Configured"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("headers = %v, want %v", headers, want)
	}
	if want := [][]string{{"app-1", "false"}}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

func TestAppPriceScheduleNotConfiguredRowsWithReason(t *testing.T) {
	headers, rows := appPriceScheduleNotConfiguredRows(&AppPriceScheduleNotConfiguredResult{
		AppID:         "app-1",
		BaseTerritory: "USA",
		Reason:        "no_current_base_price",
	})
	if want := []string{"App ID", "Configured", "Base Territory", "Reason", "Next Start Date"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("headers = %v, want %v", headers, want)
	}
	if want := [][]string{{"app-1", "false", "USA", "no_current_base_price", ""}}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

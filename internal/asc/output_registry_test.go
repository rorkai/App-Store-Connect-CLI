package asc

import (
	"reflect"
	"testing"
)

func TestOutputRegistryNotEmpty(t *testing.T) {
	if len(outputRegistry) == 0 {
		t.Fatal("output registry is empty; init() may not have run")
	}
}

func TestOutputRegistryAllHandlersNonNil(t *testing.T) {
	for typ, fn := range outputRegistry {
		if fn == nil {
			t.Errorf("nil handler registered for type %s", typ)
		}
	}
}

func TestOutputRegistryExpectedTypeCount(t *testing.T) {
	// Total registered types across both registries should be ~471.
	total := len(outputRegistry) + len(directRenderRegistry)
	const minExpected = 460
	if total < minExpected {
		t.Errorf("expected at least %d registered types, got %d (rows: %d, direct: %d)",
			minExpected, total, len(outputRegistry), len(directRenderRegistry))
	}
}

func TestDirectRenderRegistryAllHandlersNonNil(t *testing.T) {
	for typ, fn := range directRenderRegistry {
		if fn == nil {
			t.Errorf("nil handler registered for type %s", typ)
		}
	}
}

func TestRenderByRegistryFallbackToJSON(t *testing.T) {
	// Unregistered type should fall back to JSON without error.
	type unregistered struct {
		Value string `json:"value"`
	}
	output := captureStdout(t, func() error {
		return renderByRegistry(&unregistered{Value: "test"}, RenderTable)
	})
	if output == "" {
		t.Fatal("expected JSON fallback output")
	}
	if !contains(output, "test") {
		t.Fatalf("expected JSON output to contain 'test', got: %s", output)
	}
}

func TestOutputRegistrySingleLinkageHelperRegistration(t *testing.T) {
	handler, ok := outputRegistry[reflect.TypeOf(&AppStoreVersionSubmissionLinkageResponse{})]
	if !ok || handler == nil {
		t.Fatal("expected AppStoreVersionSubmissionLinkageResponse handler")
	}

	headers, rows, err := handler(&AppStoreVersionSubmissionLinkageResponse{
		Data: ResourceData{
			Type: ResourceType("appStoreVersionSubmissions"),
			ID:   "submission-123",
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(headers) == 0 || len(rows) == 0 {
		t.Fatalf("expected non-empty headers/rows, got headers=%v rows=%v", headers, rows)
	}
	if len(rows[0]) < 2 {
		t.Fatalf("expected at least 2 columns in row, got row=%v", rows[0])
	}
	joined := rows[0][0] + " " + rows[0][1]
	if !contains(joined, "submission-123") {
		t.Fatalf("expected linkage row to contain ID, got row=%v", rows[0])
	}
}

func TestOutputRegistryIDStateHelperRegistration(t *testing.T) {
	handler, ok := outputRegistry[reflect.TypeOf(&BackgroundAssetVersionAppStoreReleaseResponse{})]
	if !ok || handler == nil {
		t.Fatal("expected BackgroundAssetVersionAppStoreReleaseResponse handler")
	}

	headers, rows, err := handler(&BackgroundAssetVersionAppStoreReleaseResponse{
		Data: Resource[BackgroundAssetVersionAppStoreReleaseAttributes]{
			ID:         "release-1",
			Attributes: BackgroundAssetVersionAppStoreReleaseAttributes{State: "READY_FOR_SALE"},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(headers) == 0 || len(rows) == 0 {
		t.Fatalf("expected non-empty headers/rows, got headers=%v rows=%v", headers, rows)
	}
	if len(rows[0]) < 2 {
		t.Fatalf("expected at least 2 columns in row, got row=%v", rows[0])
	}
	joined := rows[0][0] + " " + rows[0][1]
	if !contains(joined, "release-1") || !contains(joined, "READY_FOR_SALE") {
		t.Fatalf("expected row to contain ID/state, got row=%v", rows[0])
	}
}

func TestOutputRegistryIDBoolHelperRegistration(t *testing.T) {
	handler, ok := outputRegistry[reflect.TypeOf(&AlternativeDistributionDomainDeleteResult{})]
	if !ok || handler == nil {
		t.Fatal("expected AlternativeDistributionDomainDeleteResult handler")
	}

	headers, rows, err := handler(&AlternativeDistributionDomainDeleteResult{
		ID:      "domain-1",
		Deleted: true,
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(headers) == 0 || len(rows) == 0 {
		t.Fatalf("expected non-empty headers/rows, got headers=%v rows=%v", headers, rows)
	}
	if len(rows[0]) < 2 {
		t.Fatalf("expected at least 2 columns in row, got row=%v", rows[0])
	}
	joined := rows[0][0] + " " + rows[0][1]
	if !contains(joined, "domain-1") || !contains(joined, "true") {
		t.Fatalf("expected row to contain ID/deleted, got row=%v", rows[0])
	}
}

func TestOutputRegistryResponseDataHelperRegistration(t *testing.T) {
	handler, ok := outputRegistry[reflect.TypeOf(&Response[BetaGroupMetricAttributes]{})]
	if !ok || handler == nil {
		t.Fatal("expected Response[BetaGroupMetricAttributes] handler")
	}

	headers, rows, err := handler(&Response[BetaGroupMetricAttributes]{
		Data: []Resource[BetaGroupMetricAttributes]{
			{
				ID:         "metric-1",
				Attributes: BetaGroupMetricAttributes{"installs": 12},
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(headers) == 0 || len(rows) == 0 || len(rows[0]) < 2 {
		t.Fatalf("expected headers/rows with 2 columns, got headers=%v rows=%v", headers, rows)
	}
	joined := rows[0][0] + " " + rows[0][1]
	if !contains(joined, "metric-1") || !contains(joined, "installs=12") {
		t.Fatalf("expected row to contain metric data, got row=%v", rows[0])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

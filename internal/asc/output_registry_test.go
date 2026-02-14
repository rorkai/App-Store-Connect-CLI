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

func TestOutputRegistrySingleResourceHelperRegistration(t *testing.T) {
	type helperAttrs struct {
		Name string `json:"name"`
	}

	registerSingleResourceRowsAdapter(func(v *Response[helperAttrs]) ([]string, [][]string) {
		if len(v.Data) == 0 {
			return []string{"ID", "Name"}, nil
		}
		return []string{"ID", "Name"}, [][]string{{v.Data[0].ID, v.Data[0].Attributes.Name}}
	})

	key := reflect.TypeOf(&SingleResponse[helperAttrs]{})
	t.Cleanup(func() {
		delete(outputRegistry, key)
	})

	handler, ok := outputRegistry[key]
	if !ok || handler == nil {
		t.Fatal("expected SingleResponse helper handler")
	}

	headers, rows, err := handler(&SingleResponse[helperAttrs]{
		Data: Resource[helperAttrs]{
			ID:         "helper-id",
			Attributes: helperAttrs{Name: "helper-name"},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(headers) != 2 || headers[0] != "ID" || headers[1] != "Name" {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("unexpected rows shape: %v", rows)
	}
	if rows[0][0] != "helper-id" || rows[0][1] != "helper-name" {
		t.Fatalf("unexpected row: %v", rows[0])
	}
}

func TestOutputRegistryRowsWithSingleResourceHelperRegistration(t *testing.T) {
	type attrs struct {
		Name string `json:"name"`
	}

	registerRowsWithSingleResourceAdapter(func(v *Response[attrs]) ([]string, [][]string) {
		if len(v.Data) == 0 {
			return []string{"ID", "Name"}, nil
		}
		return []string{"ID", "Name"}, [][]string{{v.Data[0].ID, v.Data[0].Attributes.Name}}
	})

	listKey := reflect.TypeOf(&Response[attrs]{})
	singleKey := reflect.TypeOf(&SingleResponse[attrs]{})
	t.Cleanup(func() {
		delete(outputRegistry, listKey)
		delete(outputRegistry, singleKey)
	})

	listHandler, ok := outputRegistry[listKey]
	if !ok || listHandler == nil {
		t.Fatal("expected list handler from rows+single-resource helper")
	}
	singleHandler, ok := outputRegistry[singleKey]
	if !ok || singleHandler == nil {
		t.Fatal("expected single handler from rows+single-resource helper")
	}

	_, listRows, err := listHandler(&Response[attrs]{
		Data: []Resource[attrs]{{ID: "list-id", Attributes: attrs{Name: "list-name"}}},
	})
	if err != nil {
		t.Fatalf("list handler returned error: %v", err)
	}
	if len(listRows) != 1 || len(listRows[0]) != 2 || listRows[0][0] != "list-id" || listRows[0][1] != "list-name" {
		t.Fatalf("unexpected list rows: %v", listRows)
	}

	_, singleRows, err := singleHandler(&SingleResponse[attrs]{
		Data: Resource[attrs]{ID: "single-id", Attributes: attrs{Name: "single-name"}},
	})
	if err != nil {
		t.Fatalf("single handler returned error: %v", err)
	}
	if len(singleRows) != 1 || len(singleRows[0]) != 2 || singleRows[0][0] != "single-id" || singleRows[0][1] != "single-name" {
		t.Fatalf("unexpected single rows: %v", singleRows)
	}
}

func TestOutputRegistrySingleToListHelperRegistration(t *testing.T) {
	type single struct {
		Data string
	}
	type list struct {
		Data []string
	}

	registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
		if len(v.Data) == 0 {
			return []string{"value"}, nil
		}
		return []string{"value"}, [][]string{{v.Data[0]}}
	})

	key := reflect.TypeOf(&single{})
	t.Cleanup(func() {
		delete(outputRegistry, key)
	})

	handler, ok := outputRegistry[key]
	if !ok || handler == nil {
		t.Fatal("expected single-to-list helper handler")
	}

	headers, rows, err := handler(&single{Data: "converted"})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(headers) != 1 || headers[0] != "value" {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if len(rows) != 1 || len(rows[0]) != 1 || rows[0][0] != "converted" {
		t.Fatalf("unexpected rows: %v", rows)
	}
}

func TestOutputRegistryRowsWithSingleToListHelperRegistration(t *testing.T) {
	type single struct {
		Data string
	}
	type list struct {
		Data []string
	}

	registerRowsWithSingleToListAdapter[single, list](func(v *list) ([]string, [][]string) {
		if len(v.Data) == 0 {
			return []string{"value"}, nil
		}
		return []string{"value"}, [][]string{{v.Data[0]}}
	})

	singleKey := reflect.TypeOf(&single{})
	listKey := reflect.TypeOf(&list{})
	t.Cleanup(func() {
		delete(outputRegistry, singleKey)
		delete(outputRegistry, listKey)
	})

	singleHandler, ok := outputRegistry[singleKey]
	if !ok || singleHandler == nil {
		t.Fatal("expected single handler from rows+single-to-list helper")
	}
	listHandler, ok := outputRegistry[listKey]
	if !ok || listHandler == nil {
		t.Fatal("expected list handler from rows+single-to-list helper")
	}

	_, singleRows, err := singleHandler(&single{Data: "single-value"})
	if err != nil {
		t.Fatalf("single handler returned error: %v", err)
	}
	if len(singleRows) != 1 || len(singleRows[0]) != 1 || singleRows[0][0] != "single-value" {
		t.Fatalf("unexpected single rows: %v", singleRows)
	}

	_, listRows, err := listHandler(&list{Data: []string{"list-value"}})
	if err != nil {
		t.Fatalf("list handler returned error: %v", err)
	}
	if len(listRows) != 1 || len(listRows[0]) != 1 || listRows[0][0] != "list-value" {
		t.Fatalf("unexpected list rows: %v", listRows)
	}
}

func TestOutputRegistrySingleToListHelperCopiesLinks(t *testing.T) {
	type single struct {
		Data  ResourceData
		Links Links
	}
	type list struct {
		Data  []ResourceData
		Links Links
	}

	registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
		if len(v.Data) == 0 {
			return []string{"id", "self"}, nil
		}
		return []string{"id", "self"}, [][]string{{v.Data[0].ID, v.Links.Self}}
	})

	key := reflect.TypeOf(&single{})
	t.Cleanup(func() {
		delete(outputRegistry, key)
	})

	handler, ok := outputRegistry[key]
	if !ok || handler == nil {
		t.Fatal("expected single-to-list links helper handler")
	}

	headers, rows, err := handler(&single{
		Data: ResourceData{ID: "item-1", Type: ResourceType("items")},
		Links: Links{
			Self: "https://example.test/items/1",
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(headers) != 2 || headers[0] != "id" || headers[1] != "self" {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("unexpected rows shape: %v", rows)
	}
	if rows[0][0] != "item-1" || rows[0][1] != "https://example.test/items/1" {
		t.Fatalf("unexpected row: %v", rows[0])
	}
}

func TestOutputRegistrySingleToListHelperPanicsWithoutDataField(t *testing.T) {
	type single struct {
		Value string
	}
	type list struct {
		Data []string
	}

	registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
		return []string{"value"}, [][]string{{v.Data[0]}}
	})

	key := reflect.TypeOf(&single{})
	t.Cleanup(func() {
		delete(outputRegistry, key)
	})

	handler, ok := outputRegistry[key]
	if !ok || handler == nil {
		t.Fatal("expected helper handler")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when source Data field is missing")
		}
	}()

	_, _, _ = handler(&single{Value: "missing-data"})
}

func TestOutputRegistrySingleToListHelperPanicsWhenTargetDataIsNotSlice(t *testing.T) {
	type single struct {
		Data string
	}
	type list struct {
		Data string
	}

	registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
		return []string{"value"}, [][]string{{v.Data}}
	})

	key := reflect.TypeOf(&single{})
	t.Cleanup(func() {
		delete(outputRegistry, key)
	})

	handler, ok := outputRegistry[key]
	if !ok || handler == nil {
		t.Fatal("expected helper handler")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when target Data field is not slice")
		}
	}()

	_, _, _ = handler(&single{Data: "not-slice"})
}

func TestOutputRegistrySingleToListHelperPanicsOnDataTypeMismatch(t *testing.T) {
	type single struct {
		Data int
	}
	type list struct {
		Data []string
	}

	registerSingleToListRowsAdapter[single, list](func(v *list) ([]string, [][]string) {
		return []string{"value"}, nil
	})

	key := reflect.TypeOf(&single{})
	t.Cleanup(func() {
		delete(outputRegistry, key)
	})

	handler, ok := outputRegistry[key]
	if !ok || handler == nil {
		t.Fatal("expected helper handler")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when Data element types mismatch")
		}
	}()

	_, _, _ = handler(&single{Data: 42})
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

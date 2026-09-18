package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func decodeLocalizationAttributes(t *testing.T, req *http.Request) (string, string, map[string]json.RawMessage) {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var envelope struct {
		Data struct {
			Type       string                     `json:"type"`
			ID         string                     `json:"id"`
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode body %s: %v", body, err)
	}
	return envelope.Data.Type, envelope.Data.ID, envelope.Data.Attributes
}

func assertLocalizationAttributes(t *testing.T, attributes map[string]json.RawMessage, want map[string]string) {
	t.Helper()

	if len(attributes) != len(want) {
		t.Fatalf("attributes = %v, want exactly %v", attributes, want)
	}
	for field, wantValue := range want {
		raw, ok := attributes[field]
		if !ok {
			t.Fatalf("attributes = %v, want field %q", attributes, field)
		}
		if string(raw) != wantValue {
			t.Fatalf("attribute %q = %s, want %s", field, raw, wantValue)
		}
	}
}

func TestUpdateAppInfoLocalizationNullableFields_SendsNullForClearedFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"name":"New Name","subtitle":null}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appInfoLocalizations/loc-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		resourceType, id, attributes := decodeLocalizationAttributes(t, req)
		if resourceType != string(ResourceTypeAppInfoLocalizations) || id != "loc-1" {
			t.Fatalf("unexpected resource identity: %s %s", resourceType, id)
		}
		assertLocalizationAttributes(t, attributes, map[string]string{
			"name":     `"New Name"`,
			"subtitle": "null",
		})
	}, response)

	name := "New Name"
	if _, err := client.UpdateAppInfoLocalizationNullableFields(context.Background(), "loc-1", map[string]NullableString{
		"name":     {Value: &name},
		"subtitle": {},
	}); err != nil {
		t.Fatalf("UpdateAppInfoLocalizationNullableFields() error: %v", err)
	}
}

func TestUpdateAppStoreVersionLocalizationNullableFields_SendsNullForClearedFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"promotionalText":null}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		resourceType, id, attributes := decodeLocalizationAttributes(t, req)
		if resourceType != string(ResourceTypeAppStoreVersionLocalizations) || id != "loc-1" {
			t.Fatalf("unexpected resource identity: %s %s", resourceType, id)
		}
		assertLocalizationAttributes(t, attributes, map[string]string{"promotionalText": "null"})
	}, response)

	if _, err := client.UpdateAppStoreVersionLocalizationNullableFields(context.Background(), "loc-1", map[string]NullableString{
		"promotionalText": {},
	}); err != nil {
		t.Fatalf("UpdateAppStoreVersionLocalizationNullableFields() error: %v", err)
	}
}

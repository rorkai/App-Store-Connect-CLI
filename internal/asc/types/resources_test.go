package types

import (
	"encoding/json"
	"testing"
)

func TestResponseAccessors(t *testing.T) {
	r := &Response[struct{ Name string }]{
		Data: []Resource[struct{ Name string }]{
			{
				Type: ResourceTypeApps,
				ID:   "app-1",
				Attributes: struct{ Name string }{
					Name: "Example",
				},
			},
		},
		Links: Links{
			Self: "/v1/apps",
			Next: "/v1/apps?page=2",
		},
	}

	links := r.GetLinks()
	if links == nil || links.Next != "/v1/apps?page=2" {
		t.Fatalf("unexpected links: %+v", links)
	}

	data, ok := r.GetData().([]Resource[struct{ Name string }])
	if !ok {
		t.Fatalf("expected []Resource data type, got %T", r.GetData())
	}
	if len(data) != 1 || data[0].ID != "app-1" {
		t.Fatalf("unexpected data payload: %+v", data)
	}
}

func TestLinkagesResponseAccessors(t *testing.T) {
	r := &LinkagesResponse{
		Data: []ResourceData{
			{Type: ResourceTypeBuilds, ID: "build-1"},
		},
		Links: Links{
			Self: "/v1/builds",
		},
	}

	links := r.GetLinks()
	if links == nil || links.Self != "/v1/builds" {
		t.Fatalf("unexpected links: %+v", links)
	}

	data, ok := r.GetData().([]ResourceData)
	if !ok {
		t.Fatalf("expected []ResourceData type, got %T", r.GetData())
	}
	if len(data) != 1 || data[0].ID != "build-1" {
		t.Fatalf("unexpected linkage payload: %+v", data)
	}
}

func TestTypeConstants(t *testing.T) {
	if PlatformIOS != "IOS" || PlatformMacOS != "MAC_OS" {
		t.Fatalf("unexpected platform constants: %q %q", PlatformIOS, PlatformMacOS)
	}
	if ChecksumAlgorithmSHA256 != "SHA_256" {
		t.Fatalf("unexpected checksum algorithm constant: %q", ChecksumAlgorithmSHA256)
	}
	if UTIIPA != "com.apple.ipa" || UTIPKG != "com.apple.pkg" {
		t.Fatalf("unexpected UTI constants: %q %q", UTIIPA, UTIPKG)
	}
}

func TestParsePagingTotalOK(t *testing.T) {
	tests := []struct {
		name      string
		meta      string
		wantTotal int
		wantOK    bool
	}{
		{
			name:      "nil meta",
			meta:      "",
			wantTotal: 0,
			wantOK:    false,
		},
		{
			name:      "meta missing total field",
			meta:      `{"paging":{"limit":1}}`,
			wantTotal: 0,
			wantOK:    false,
		},
		{
			name:      "meta with total zero",
			meta:      `{"paging":{"total":0,"limit":1}}`,
			wantTotal: 0,
			wantOK:    true,
		},
		{
			name:      "meta with positive total",
			meta:      `{"paging":{"total":42,"limit":1}}`,
			wantTotal: 42,
			wantOK:    true,
		},
		{
			name:      "invalid json",
			meta:      `not-json`,
			wantTotal: 0,
			wantOK:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var raw json.RawMessage
			if tc.meta != "" {
				raw = json.RawMessage(tc.meta)
			}
			got, ok := ParsePagingTotalOK(raw)
			if ok != tc.wantOK {
				t.Errorf("ParsePagingTotalOK() ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.wantTotal {
				t.Errorf("ParsePagingTotalOK() total = %d, want %d", got, tc.wantTotal)
			}
		})
	}
}

func TestParsePagingTotal_BackwardCompatibility(t *testing.T) {
	// Verify ParsePagingTotal still returns 0 in all absent-total cases.
	if got := ParsePagingTotal(nil); got != 0 {
		t.Errorf("ParsePagingTotal(nil) = %d, want 0", got)
	}
	if got := ParsePagingTotal(json.RawMessage(`{"paging":{"limit":1}}`)); got != 0 {
		t.Errorf("ParsePagingTotal(no total) = %d, want 0", got)
	}
	if got := ParsePagingTotal(json.RawMessage(`{"paging":{"total":7,"limit":1}}`)); got != 7 {
		t.Errorf("ParsePagingTotal(total=7) = %d, want 7", got)
	}
}

func TestRelationshipRequest_MarshalJSON_EncodesEmptyArray(t *testing.T) {
	// RelationshipRequest represents a to-many relationship payload. In JSON:API, an empty
	// relationship list is encoded as {"data":[]} (not {"data":null}).
	body, err := json.Marshal(RelationshipRequest{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RelationshipRequest
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Data == nil {
		t.Fatalf("expected data to decode as an empty array, got nil (body=%q)", string(body))
	}
	if len(got.Data) != 0 {
		t.Fatalf("expected empty data array, got %d items", len(got.Data))
	}
}

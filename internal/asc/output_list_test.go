package asc

import (
	"encoding/json"
	"reflect"
	"testing"
)

type listEnvelopeTestAttrs struct {
	Name string `json:"name"`
}

func listEnvelopeTestResponse() *Response[listEnvelopeTestAttrs] {
	return &Response[listEnvelopeTestAttrs]{
		Data: []Resource[listEnvelopeTestAttrs]{
			{Type: "apps", ID: "app-1", Attributes: listEnvelopeTestAttrs{Name: "First"}},
			{Type: "apps", ID: "app-2", Attributes: listEnvelopeTestAttrs{Name: "Second"}},
		},
		Links: Links{
			Self: "https://api.example.test/v1/apps",
			Next: "https://api.example.test/v1/apps?cursor=abc",
		},
		Included: json.RawMessage(`[{"type":"betaGroups","id":"group-1"}]`),
		Meta:     json.RawMessage(`{"paging":{"total":7,"limit":2}}`),
	}
}

func TestNewListEnvelopeMarshalShape(t *testing.T) {
	resp := listEnvelopeTestResponse()
	envelope := NewListEnvelope(resp)

	if envelope.TotalCount != 7 {
		t.Fatalf("TotalCount = %d, want 7 (meta.paging.total)", envelope.TotalCount)
	}
	if !envelope.HasMore {
		t.Fatal("HasMore = false, want true when links.next is set")
	}

	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	responseJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var envelopeMap map[string]json.RawMessage
	if err := json.Unmarshal(envelopeJSON, &envelopeMap); err != nil {
		t.Fatalf("unmarshal envelope JSON: %v", err)
	}
	var responseMap map[string]json.RawMessage
	if err := json.Unmarshal(responseJSON, &responseMap); err != nil {
		t.Fatalf("unmarshal response JSON: %v", err)
	}

	// Additive superset: every raw envelope key marshals byte-identically.
	for key, want := range responseMap {
		got, ok := envelopeMap[key]
		if !ok {
			t.Fatalf("envelope JSON missing raw envelope key %q", key)
		}
		if string(got) != string(want) {
			t.Fatalf("envelope key %q = %s, want %s", key, got, want)
		}
	}

	// Exactly two additive fields beyond the raw envelope.
	if len(envelopeMap) != len(responseMap)+2 {
		t.Fatalf("envelope has %d keys, want %d (raw envelope keys + totalCount + hasMore)",
			len(envelopeMap), len(responseMap)+2)
	}
	if string(envelopeMap["totalCount"]) != "7" {
		t.Fatalf("totalCount JSON = %s, want 7", envelopeMap["totalCount"])
	}
	if string(envelopeMap["hasMore"]) != "true" {
		t.Fatalf("hasMore JSON = %s, want true", envelopeMap["hasMore"])
	}
}

func TestNewListEnvelopeTotalCountAndHasMore(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(*Response[listEnvelopeTestAttrs])
		wantTotal     int
		wantHasMore   bool
		wantDataCount int
	}{
		{
			name:          "meta paging total wins over data length",
			mutate:        func(*Response[listEnvelopeTestAttrs]) {},
			wantTotal:     7,
			wantHasMore:   true,
			wantDataCount: 2,
		},
		{
			name: "missing meta falls back to len(data)",
			mutate: func(r *Response[listEnvelopeTestAttrs]) {
				r.Meta = nil
			},
			wantTotal:     2,
			wantHasMore:   true,
			wantDataCount: 2,
		},
		{
			name: "unparseable meta falls back to len(data)",
			mutate: func(r *Response[listEnvelopeTestAttrs]) {
				r.Meta = json.RawMessage(`{"paging":`)
			},
			wantTotal:     2,
			wantHasMore:   true,
			wantDataCount: 2,
		},
		{
			name: "meta without paging total falls back to len(data)",
			mutate: func(r *Response[listEnvelopeTestAttrs]) {
				r.Meta = json.RawMessage(`{"paging":{"limit":2}}`)
			},
			wantTotal:     2,
			wantHasMore:   true,
			wantDataCount: 2,
		},
		{
			name: "no next link means no more pages",
			mutate: func(r *Response[listEnvelopeTestAttrs]) {
				r.Links.Next = ""
			},
			wantTotal:     7,
			wantHasMore:   false,
			wantDataCount: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := listEnvelopeTestResponse()
			tc.mutate(resp)

			envelope := NewListEnvelope(resp)
			if envelope.TotalCount != tc.wantTotal {
				t.Fatalf("TotalCount = %d, want %d", envelope.TotalCount, tc.wantTotal)
			}
			if envelope.HasMore != tc.wantHasMore {
				t.Fatalf("HasMore = %t, want %t", envelope.HasMore, tc.wantHasMore)
			}
			if len(envelope.Data) != tc.wantDataCount {
				t.Fatalf("len(Data) = %d, want %d", len(envelope.Data), tc.wantDataCount)
			}
			if !reflect.DeepEqual(envelope.Response, *resp) {
				t.Fatal("embedded response was not copied unmodified")
			}
		})
	}

	t.Run("nil response yields zero envelope", func(t *testing.T) {
		envelope := NewListEnvelope[listEnvelopeTestAttrs](nil)
		if envelope.TotalCount != 0 || envelope.HasMore || len(envelope.Data) != 0 {
			t.Fatalf("expected zero envelope for nil response, got %+v", envelope)
		}
	})
}

func TestItemListMarshalShape(t *testing.T) {
	list := ItemList[listEnvelopeTestAttrs]{
		Items:      []listEnvelopeTestAttrs{{Name: "Only"}},
		TotalCount: 4,
		HasMore:    true,
	}
	got, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshal item list: %v", err)
	}
	want := `{"items":[{"name":"Only"}],"totalCount":4,"hasMore":true}`
	if string(got) != want {
		t.Fatalf("item list JSON = %s, want %s", got, want)
	}
}

// TestListEnvelopeRegistryFallback documents the soft table-output contract for
// ListEnvelope: instantiations are not pre-registered, and renderByRegistry
// intentionally falls back to JSON output for unregistered types. Commands that
// want a table for a ListEnvelope instantiation must register a renderer for
// that concrete instantiation; until they do, --output table degrades to JSON
// rather than failing.
func TestListEnvelopeRegistryFallback(t *testing.T) {
	envelope := NewListEnvelope(listEnvelopeTestResponse())

	output := captureStdout(t, func() error {
		return renderByRegistry(&envelope, RenderTable)
	})
	if output == "" {
		t.Fatal("expected JSON fallback output")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("expected JSON fallback for unregistered ListEnvelope, got: %s", output)
	}
	if _, ok := parsed["data"]; !ok {
		t.Fatalf("fallback JSON missing raw envelope data key: %s", output)
	}
	if got, ok := parsed["totalCount"].(float64); !ok || int(got) != 7 {
		t.Fatalf("fallback JSON totalCount = %v, want 7: %s", parsed["totalCount"], output)
	}
	if got, ok := parsed["hasMore"].(bool); !ok || !got {
		t.Fatalf("fallback JSON hasMore = %v, want true: %s", parsed["hasMore"], output)
	}
}

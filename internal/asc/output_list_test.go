package asc

import (
	"encoding/json"
	"testing"
)

type listEnvelopeTestAttrs struct {
	Name string `json:"name"`
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

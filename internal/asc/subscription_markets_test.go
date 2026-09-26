package asc

import (
	"encoding/json"
	"testing"
)

func TestSubscriptionResponsePreservesEmptyMarkets(t *testing.T) {
	var response SubscriptionResponse
	if err := json.Unmarshal([]byte(`{"data":{"type":"subscriptions","id":"sub-45","attributes":{"marketSettings":[],"multiSeatStatus":"DISABLED"}}}`), &response); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Data struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if string(result.Data.Attributes["marketSettings"]) != "[]" || string(result.Data.Attributes["multiSeatStatus"]) != `"DISABLED"` {
		t.Fatalf("settings changed: %s", data)
	}
}

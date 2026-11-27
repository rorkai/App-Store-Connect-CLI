package asc

import "encoding/json"

type CatalogItem struct {
	Kind     string         `json:"kind"`
	Key      string         `json:"key"`
	ID       string         `json:"id,omitempty"`
	Action   string         `json:"action"`
	Status   string         `json:"status"`
	Changes  map[string]any `json:"changes,omitempty"`
	Error    string         `json:"error,omitempty"`
	Deferred bool           `json:"deferred,omitempty"`
}

type CatalogResult struct {
	AppID      string        `json:"appId"`
	Status     string        `json:"status"`
	Items      []CatalogItem `json:"items"`
	PlanSHA256 string        `json:"planSha256,omitempty"`
	Recovery   string        `json:"recovery,omitempty"`
}

func catalogResultRows(result *CatalogResult) ([]string, [][]string) {
	rows := make([][]string, 0, len(result.Items))
	for _, item := range result.Items {
		changes, _ := json.Marshal(item.Changes)
		rows = append(rows, []string{item.Kind, item.Key, item.ID, item.Action, item.Status, string(changes), item.Error})
	}
	return []string{"Kind", "Key", "ID", "Action", "Status", "Changes", "Error"}, rows
}

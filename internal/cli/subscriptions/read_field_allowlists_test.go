package subscriptions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSubscriptionReadAllowlistsMatchOpenAPIEnums(t *testing.T) {
	groupLists := map[string][]string{
		"fields[subscriptionGroups]":             subscriptionGroupVersionGroupFields,
		"fields[subscriptionGroupVersions]":      subscriptionGroupVersionFields,
		"fields[subscriptions]":                  subscriptionFieldsList(),
		"fields[subscriptionGroupLocalizations]": subscriptionGroupLocalizationFieldsList(),
	}
	subscriptionLists := map[string][]string{
		"fields[subscriptions]":        subscriptionFieldsList(),
		"fields[subscriptionVersions]": subscriptionVersionFieldsList(),
		"include":                      subscriptionIncludeList(),
	}
	for _, field := range subscriptionRelatedFieldFlags {
		subscriptionLists["fields["+field.resource+"]"] = field.allowed
	}
	operations := map[string]map[string][]string{
		"/v1/apps/{id}/subscriptionGroups":          groupLists,
		"/v1/subscriptionGroups/{id}":               groupLists,
		"/v1/subscriptionGroups/{id}/subscriptions": subscriptionLists,
		"/v1/subscriptions/{id}":                    subscriptionLists,
	}

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "openapi", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Paths map[string]struct {
			Get struct {
				Parameters []struct {
					Name   string `json:"name"`
					Schema struct {
						Items struct {
							Enum []string `json:"enum"`
						} `json:"items"`
					} `json:"schema"`
				} `json:"parameters"`
			} `json:"get"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}

	for path, lists := range operations {
		enums := make(map[string][]string)
		for _, parameter := range schema.Paths[path].Get.Parameters {
			enums[parameter.Name] = parameter.Schema.Items.Enum
		}
		for name, list := range lists {
			got := slices.Sorted(slices.Values(list))
			want := slices.Sorted(slices.Values(enums[name]))
			if len(want) == 0 || !slices.Equal(got, want) {
				t.Errorf("%s %s allowlist = %v, want %v", path, name, got, want)
			}
		}
	}
}

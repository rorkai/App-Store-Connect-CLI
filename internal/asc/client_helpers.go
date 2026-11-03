package asc

import (
	"net/url"
	"strconv"
	"strings"
)

func buildRelationshipList(resourceType ResourceType, ids []string) *RelationshipList {
	normalized := normalizeList(ids)
	if len(normalized) == 0 {
		return nil
	}
	data := make([]ResourceData, 0, len(normalized))
	for _, id := range normalized {
		data = append(data, ResourceData{Type: resourceType, ID: id})
	}
	return &RelationshipList{Data: data}
}

func normalizeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	return normalized
}

// buildToManyRelationship returns nil for an empty list so an omitted optional
// relationship stays out of the request body.
func buildToManyRelationship(resourceType ResourceType, ids []string) *RelationshipList {
	ids = normalizeList(ids)
	if len(ids) == 0 {
		return nil
	}
	data := make([]ResourceData, 0, len(ids))
	for _, id := range ids {
		data = append(data, ResourceData{Type: resourceType, ID: id})
	}
	return &RelationshipList{Data: data}
}

func normalizeUniqueList(values []string) []string {
	values = normalizeList(values)
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func sameOrderedList(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func normalizeUpperList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized = append(normalized, strings.ToUpper(value))
	}
	return normalized
}

func normalizeCSVString(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	normalized := normalizeList(strings.Split(value, ","))
	if len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, ",")
}

func normalizeUpperCSVString(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	normalized := normalizeUpperList(strings.Split(value, ","))
	if len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, ",")
}

func addCSV(values url.Values, key string, items []string) {
	items = normalizeList(items)
	if len(items) == 0 {
		return
	}
	values.Set(key, strings.Join(items, ","))
}

func addLimit(values url.Values, limit int) {
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
}

func addValue(values url.Values, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	values.Set(key, value)
}

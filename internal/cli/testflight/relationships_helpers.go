package testflight

import "sort"

type relationshipKind int

const (
	relationshipSingle relationshipKind = iota
	relationshipList
)

func relationshipTypeList(kinds map[string]relationshipKind) []string {
	relationships := make([]string, 0, len(kinds))
	for key := range kinds {
		relationships = append(relationships, key)
	}
	sort.Strings(relationships)
	return relationships
}

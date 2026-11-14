package asc

// ItemList is the envelope for computed collection results that do not come
// from a single ASC response (merged, filtered, or lookup-derived items).
// Adding fields here must remain additive and removals follow the stability
// ladder.
type ItemList[T any] struct {
	Items      []T  `json:"items"`
	TotalCount int  `json:"totalCount"`
	HasMore    bool `json:"hasMore"`
}

package asc

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc/types"

// ListEnvelope wraps Apple's collection envelope with computed pagination
// fields. The JSON shape is an additive superset of the raw envelope: the
// embedded Response's data/links/included/meta keys marshal unmodified at the
// top level, and totalCount/hasMore are added alongside them. Existing
// consumers of the raw envelope keep working; new consumers can rely on the
// computed fields without parsing meta.paging themselves.
type ListEnvelope[T any] struct {
	types.Response[T]
	TotalCount int  `json:"totalCount"`
	HasMore    bool `json:"hasMore"`
}

// ItemList is the envelope for computed collection results that do not come
// from a single ASC response (merged, filtered, or lookup-derived items).
// It intentionally mirrors ListEnvelope's computed fields so consumers read
// totalCount/hasMore identically for both shapes; adding fields here must
// remain additive and removals follow the stability ladder.
type ItemList[T any] struct {
	Items      []T  `json:"items"`
	TotalCount int  `json:"totalCount"`
	HasMore    bool `json:"hasMore"`
}

// NewListEnvelope builds an additive superset of the given collection
// response. TotalCount comes from meta.paging.total when present and falls
// back to len(r.Data) when the API omits it; HasMore reflects whether a next
// page link exists. The embedded response is copied unmodified so the raw
// envelope keys marshal byte-identically to the original.
func NewListEnvelope[T any](r *types.Response[T]) ListEnvelope[T] {
	if r == nil {
		return ListEnvelope[T]{}
	}
	total, ok := types.ParsePagingTotalOK(r.Meta)
	if !ok {
		total = len(r.Data)
	}
	return ListEnvelope[T]{
		Response:   *r,
		TotalCount: total,
		HasMore:    r.Links.Next != "",
	}
}

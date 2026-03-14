package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// GetLinks returns the links field for pagination.
func (r *PreReleaseVersionsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *PreReleaseVersionsResponse) GetData() any {
	return r.Data
}

// PaginateFunc is a function that fetches a page of results
type PaginateFunc func(ctx context.Context, nextURL string) (PaginatedResponse, error)

// PageConsumer handles one pagination page.
type PageConsumer func(page PaginatedResponse) error

// PaginateAll fetches all pages and aggregates results.
// It uses reflection to create an empty result container of the same type as
// firstPage, eliminating the need for a type switch per response type.
func PaginateAll(ctx context.Context, firstPage PaginatedResponse, fetchNext PaginateFunc) (PaginatedResponse, error) {
	if firstPage == nil {
		return nil, nil
	}

	// Check for typed nil (non-nil interface containing nil pointer).
	// Return an empty result of the same type rather than panicking.
	if reflect.ValueOf(firstPage).IsNil() {
		return newEmptyPaginatedResponse(firstPage)
	}

	// Create an empty result of the same concrete type using reflection.
	result, err := newEmptyPaginatedResponse(firstPage)
	if err != nil {
		return nil, err
	}

	page := 1
	seenNext := make(map[string]struct{})
	for {
		// Aggregate data from current page using reflection over the Data field.
		if err := aggregatePageData(result, firstPage); err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}

		// Check for next page
		links := firstPage.GetLinks()
		if links == nil || links.Next == "" {
			break
		}

		if _, ok := seenNext[links.Next]; ok {
			return result, fmt.Errorf("page %d: %w", page+1, ErrRepeatedPaginationURL)
		}
		seenNext[links.Next] = struct{}{}
		page++

		// Fetch next page
		nextPage, err := fetchNext(ctx, links.Next)
		if err != nil {
			return result, fmt.Errorf("page %d: %w", page, err)
		}

		// Validate that the response type matches
		if reflect.TypeOf(nextPage) != reflect.TypeOf(firstPage) {
			return result, fmt.Errorf("page %d: unexpected response type (expected %T, got %T)", page, firstPage, nextPage)
		}

		firstPage = nextPage
	}

	return result, nil
}

// PaginateEach iterates pages and invokes consume for each page without
// aggregating all page data in memory.
func PaginateEach(ctx context.Context, firstPage PaginatedResponse, fetchNext PaginateFunc, consume PageConsumer) error {
	if firstPage == nil {
		return nil
	}
	if consume == nil {
		return fmt.Errorf("page consumer is required")
	}

	// Handle typed nil (non-nil interface containing nil pointer).
	if reflect.ValueOf(firstPage).IsNil() {
		return nil
	}

	page := 1
	current := firstPage
	seenNext := make(map[string]struct{})

	for {
		if err := consume(current); err != nil {
			return fmt.Errorf("page %d: %w", page, err)
		}

		links := current.GetLinks()
		if links == nil || links.Next == "" {
			return nil
		}
		if _, ok := seenNext[links.Next]; ok {
			return fmt.Errorf("page %d: %w", page+1, ErrRepeatedPaginationURL)
		}
		seenNext[links.Next] = struct{}{}

		nextPage, err := fetchNext(ctx, links.Next)
		if err != nil {
			return fmt.Errorf("page %d: %w", page+1, err)
		}
		if reflect.TypeOf(nextPage) != reflect.TypeOf(current) {
			return fmt.Errorf("page %d: unexpected response type (expected %T, got %T)", page+1, current, nextPage)
		}

		current = nextPage
		page++
	}
}

// newEmptyPaginatedResponse creates a new zero-valued instance of the same
// concrete type as src. The returned value is a pointer to a new struct that
// satisfies PaginatedResponse.
func newEmptyPaginatedResponse(src PaginatedResponse) (PaginatedResponse, error) {
	srcValue := reflect.ValueOf(src)
	if srcValue.Kind() != reflect.Pointer {
		return nil, fmt.Errorf("unsupported response type for pagination: %T (expected pointer)", src)
	}

	// Create a new zero-valued struct of the same type.
	// Use srcValue.Type().Elem() instead of srcValue.Elem().Type() to handle
	// typed nil pointers (e.g., var resp *Type = nil passed as interface).
	newPtr := reflect.New(srcValue.Type().Elem())
	result, ok := newPtr.Interface().(PaginatedResponse)
	if !ok {
		return nil, fmt.Errorf("unsupported response type for pagination: %T does not implement PaginatedResponse", src)
	}
	return result, nil
}

// aggregatePageData appends page data to result by reflecting on the shared Data field.
// This keeps pagination aggregation generic while still validating type compatibility.
func aggregatePageData(result, page PaginatedResponse) error {
	if result == nil || page == nil {
		return fmt.Errorf("page aggregation received nil result or page")
	}

	resultValue := reflect.ValueOf(result)
	pageValue := reflect.ValueOf(page)
	if resultValue.Kind() != reflect.Pointer || pageValue.Kind() != reflect.Pointer {
		return fmt.Errorf("page aggregation expects pointer types (got %T and %T)", result, page)
	}

	if resultValue.Type() != pageValue.Type() {
		return fmt.Errorf("type mismatch: page is %T but result is %T", page, result)
	}

	// Handle typed nil pointers (non-nil interface containing nil pointer).
	// A typed nil page has no data to aggregate, so skip it.
	if pageValue.IsNil() {
		return nil
	}
	if resultValue.IsNil() {
		return fmt.Errorf("page aggregation received nil result pointer")
	}

	resultElem := resultValue.Elem()
	pageElem := pageValue.Elem()
	resultData := resultElem.FieldByName("Data")
	pageData := pageElem.FieldByName("Data")
	if !resultData.IsValid() || !pageData.IsValid() {
		return fmt.Errorf("missing Data field for %T", page)
	}
	if resultData.Kind() != reflect.Slice || pageData.Kind() != reflect.Slice {
		return fmt.Errorf("data field is not a slice for %T", page)
	}
	if resultData.Type() != pageData.Type() {
		return fmt.Errorf("data field type mismatch: %s vs %s", resultData.Type(), pageData.Type())
	}

	resultData.Set(reflect.AppendSlice(resultData, pageData))
	if err := aggregateJSONRawArrayField(resultElem, pageElem, "Included"); err != nil {
		return err
	}
	return nil
}

func aggregateJSONRawArrayField(resultElem, pageElem reflect.Value, fieldName string) error {
	resultField := resultElem.FieldByName(fieldName)
	pageField := pageElem.FieldByName(fieldName)
	if !resultField.IsValid() || !pageField.IsValid() {
		return nil
	}

	rawMessageType := reflect.TypeOf(json.RawMessage{})
	if resultField.Type() != rawMessageType || pageField.Type() != rawMessageType {
		return nil
	}

	merged, err := mergeRawJSONArray(resultField.Interface().(json.RawMessage), pageField.Interface().(json.RawMessage))
	if err != nil {
		return fmt.Errorf("merge %s: %w", fieldName, err)
	}
	resultField.Set(reflect.ValueOf(merged))
	return nil
}

func mergeRawJSONArray(dst, src json.RawMessage) (json.RawMessage, error) {
	switch {
	case len(src) == 0:
		return dst, nil
	case len(dst) == 0:
		return append(json.RawMessage(nil), src...), nil
	}

	var dstItems []json.RawMessage
	if err := json.Unmarshal(dst, &dstItems); err != nil {
		return nil, fmt.Errorf("parse existing array: %w", err)
	}
	var srcItems []json.RawMessage
	if err := json.Unmarshal(src, &srcItems); err != nil {
		return nil, fmt.Errorf("parse incoming array: %w", err)
	}

	merged := make([]json.RawMessage, 0, len(dstItems)+len(srcItems))
	seen := make(map[string]struct{}, len(dstItems)+len(srcItems))
	appendUnique := func(items []json.RawMessage) {
		for _, item := range items {
			key := string(item)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, item)
		}
	}
	appendUnique(dstItems)
	appendUnique(srcItems)

	result, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshal merged array: %w", err)
	}
	return result, nil
}
